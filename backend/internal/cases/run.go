package cases

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// The Pre-Screening Check (FR-PSAPI-001, FR-PSAPI-002) runs as a row of
// prescreening_runs (§9.15) driven by an in-process Executor.

// Run states (§9.15).
const (
	runRunning         = "RUNNING"
	runWaitingSLIK     = "WAITING_SLIK"
	runCompleted       = "COMPLETED"
	runStoppedRejected = "STOPPED_REJECTED"
	runStoppedFailed   = "STOPPED_FAILED"
	runStoppedBroken   = "STOPPED_BROKEN"
)

// Case statuses of the Pre-Screening Check (FR-PSAPI-001).
const (
	CaseStatusRejectedPreScreening = "ResolvedRejected-Prescreening"
	CaseStatusFailedPreScreening   = "Failed-PrescreeningConnection"

	StageRejected = "Rejected"
)

// checkOrder is FR-PSAPI-001's order of steps, by their SoT names. A step
// without an implementation pauses the run there until a later ticket adds
// it (agreed for ticket 09).
var checkOrder = []string{"Dukcapil", "DHN", "Rejected List", "SLIK", "SLIK Pasangan", "Pefindo", "Pefindo Pasangan"}

// Executor timings (FR-PSAPI-002 Process 4 and 5).
const (
	heartbeatEvery = 15 * time.Second
	sweepEvery     = 60 * time.Second
	staleAfter     = 60 * time.Second
)

// outcome is a check attempt's result (FR-PSAPI-004 rule 4).
type outcome int

const (
	outcomeSuccess outcome = iota
	outcomeRejected
	outcomeFail
	outcomeSkip // the check does not apply to the case
)

// logStatus is the outcome's Connect API Log status.
func (o outcome) logStatus() string {
	switch o {
	case outcomeRejected:
		return apiRejected
	case outcomeFail:
		return apiFail
	}
	return apiSuccess
}

// attempt is one try of a check: its outcome, the log row it made, and
// what a passed or rejected attempt changes besides the run (e.g.
// slik_report REQUESTED), applied in the transaction that records it.
type attempt struct {
	outcome outcome
	log     *apiLog
	apply   func(context.Context, pgx.Tx) error
	// A step of several APIs (Pefindo) goes on at nextAPI after a pass; a
	// pass with no nextAPI ends the step.
	nextAPI string
	// immediate repeats a failed attempt without the retry gap (FR-PSAPI-015).
	immediate bool
	// failIfNotApplied turns a pass whose apply cannot be saved into a Fail
	// that is retried (DB-RULE-075), instead of a system failure.
	failIfNotApplied bool
}

// check is one step of the run. apiName is its Connect API Log api_name and
// criterion its rejection criterion on the Rejected List; try returns an
// error only for a system failure (a missing setting), which stops the run
// broken.
type check struct {
	apiName, criterion string
	try                func(ctx context.Context, r runRef) (attempt, error)
	// retry gives the maximum retries and gap of the step's API
	// (FR-PSAPI-002); api is "" on a step of one API.
	retry func(ctx context.Context, api string) (int, time.Duration, error)
}

// runRef is a run loaded for execution: its step, the API inside the step
// ("" on a step of one API, agreed for ticket 13) and the attempt there.
type runRef struct {
	id      int64
	caseID  string
	step    string
	api     string
	attempt int
}

// Executor executes Pre-Screening Check runs in the background, one
// goroutine per run, and resumes runs whose executor died (DEC-058).
type Executor struct {
	s      *Service
	owner  string
	http   *http.Client
	getenv func(string) string
	checks map[string]check

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
	active map[int64]bool
}

func newExecutor(s *Service) *Executor {
	host, _ := os.Hostname()
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	ctx, cancel := context.WithCancel(context.Background())
	e := &Executor{
		s: s, owner: fmt.Sprintf("%s/%d/%s", host, os.Getpid(), hex.EncodeToString(b)),
		http: &http.Client{}, getenv: os.Getenv, ctx: ctx, cancel: cancel, active: map[int64]bool{},
	}
	e.checks = map[string]check{
		dukcapilAPI.logName: e.dukcapilCheck(), dhnAPI.logName: e.dhnCheck(), stepRejectedList: e.rejectedListCheck(),
		stepSLIK: e.slikCheck(slikSubjects[subjectCustomer]), stepSLIKSpouse: e.slikCheck(slikSubjects[subjectSpouse]),
		stepPefindo: e.pefindoCheck(pefindoSubjects[subjectCustomer]), stepPefindoSpouse: e.pefindoCheck(pefindoSubjects[subjectSpouse]),
	}
	return e
}

// Start runs the recovery sweep now and every sweepEvery.
func (e *Executor) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		t := time.NewTicker(sweepEvery)
		defer t.Stop()
		for {
			e.sweep()
			select {
			case <-e.ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

// Close stops every run goroutine and waits for them; their runs resume
// through another executor's sweep. No run starts after Close.
func (e *Executor) Close() {
	e.mu.Lock()
	e.closed = true
	e.cancel()
	e.mu.Unlock()
	e.wg.Wait()
}

// launch executes a run this executor owns, unless it already does.
func (e *Executor) launch(runID int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed || e.active[runID] {
		return
	}
	e.active[runID] = true
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() {
			e.mu.Lock()
			delete(e.active, runID)
			e.mu.Unlock()
		}()
		e.execute(runID)
	}()
}

// sweep takes over RUNNING runs whose heartbeat is older than staleAfter, on
// a step that exists, atomically (DB-RULE-047), and executes them.
func (e *Executor) sweep() {
	ctx := e.ctx
	implemented := make([]string, 0, len(e.checks))
	for name := range e.checks {
		implemented = append(implemented, name)
	}
	stale := e.s.now().Add(-staleAfter)
	ids, err := collect(ctx, e.s, func(r pgx.Rows) (int64, error) {
		var id int64
		return id, r.Scan(&id)
	}, `SELECT run_id FROM fos.prescreening_runs WHERE state = $1 AND heartbeat_at < $2 AND current_step = ANY($3)`,
		runRunning, stale, implemented)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("Pre-Screening Check recovery sweep failed", "error", err)
		}
		return
	}
	for _, id := range ids {
		e.mu.Lock()
		busy := e.active[id]
		e.mu.Unlock()
		if busy {
			continue
		}
		tag, err := e.s.db.Exec(ctx, `
			UPDATE fos.prescreening_runs SET owner_id = $2, heartbeat_at = $3, updated_at = $3
			WHERE run_id = $1 AND state = $4 AND heartbeat_at < $5`, id, e.owner, e.s.now(), runRunning, stale)
		if err != nil || tag.RowsAffected() != 1 {
			continue // another executor took it
		}
		slog.Info("Pre-Screening Check run taken over", "run_id", id, "owner", e.owner)
		e.launch(id)
	}
}

// heartbeat keeps the run's heartbeat_at fresh while it executes. When the
// run is no longer this executor's (taken over), it stops the execution at
// once, including a call in flight (FR-PSAPI-002 rule 3).
func (e *Executor) heartbeat(ctx context.Context, stop context.CancelFunc, runID int64) {
	t := time.NewTicker(heartbeatEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			tag, err := e.s.db.Exec(ctx, `UPDATE fos.prescreening_runs SET heartbeat_at = $3 WHERE run_id = $1 AND owner_id = $2 AND state = $4`,
				runID, e.owner, e.s.now(), runRunning)
			if err != nil {
				if ctx.Err() == nil {
					slog.Error("Pre-Screening Check heartbeat failed", "run_id", runID, "error", err)
				}
				continue
			}
			if tag.RowsAffected() == 0 {
				slog.Warn("Pre-Screening Check run lost to another executor; stopping", "run_id", runID)
				stop()
				return
			}
		}
	}
}

// load reads a run this executor owns and that is still RUNNING.
func (e *Executor) load(ctx context.Context, runID int64) (runRef, bool, error) {
	var r runRef
	err := e.s.db.QueryRow(ctx, `
		SELECT run_id, case_id, current_step, COALESCE(current_api, ''), attempt FROM fos.prescreening_runs
		WHERE run_id = $1 AND state = $2 AND owner_id = $3`, runID, runRunning, e.owner).Scan(&r.id, &r.caseID, &r.step, &r.api, &r.attempt)
	if isNotFound(err) {
		return r, false, nil
	}
	return r, err == nil, err
}

// execute runs the checks from current_step until the run stops, waits on
// a step not implemented yet, loses its ownership, or the executor closes
// (FR-PSAPI-002).
func (e *Executor) execute(runID int64) {
	ctx, stop := context.WithCancel(e.ctx)
	var beat sync.WaitGroup
	beat.Add(1)
	go func() {
		defer beat.Done()
		e.heartbeat(ctx, stop, runID)
	}()
	defer beat.Wait()
	defer stop()
	for {
		r, ok, err := e.load(ctx, runID)
		if err != nil || !ok {
			if err != nil && ctx.Err() == nil {
				slog.Error("Pre-Screening Check run load failed", "run_id", runID, "error", err)
			}
			return
		}
		c, ok := e.checks[r.step]
		if !ok {
			slog.Info("Pre-Screening Check run waits: step not implemented yet", "run_id", r.id, "case_id", r.caseID, "step", r.step)
			return
		}
		a, err := c.try(ctx, r)
		if ctx.Err() != nil {
			return // closing or lost: the attempt is not recorded and is repeated on recovery
		}
		if err != nil {
			e.stopBroken(ctx, r, err)
			return
		}
		notSaved := false
		if a.failIfNotApplied {
			a.apply = savedOr(&notSaved, applyOf(a))
		}
		goOn := e.settle(ctx, r, c, a)
		if notSaved && ctx.Err() == nil {
			// DB-RULE-075: a result that cannot be saved fails the attempt.
			slog.Error("Pre-Screening Check result not saved; the attempt fails", "run_id", r.id, "case_id", r.caseID, "api", a.log.name)
			a.log.status = apiFail
			goOn = e.settle(ctx, r, c, attempt{outcome: outcomeFail, log: a.log})
		}
		if !goOn {
			return
		}
	}
}

// settle records an attempt's outcome and moves the run on; it reports
// whether the execution goes on with the run's next attempt.
func (e *Executor) settle(ctx context.Context, r runRef, c check, a attempt) bool {
	switch a.outcome {
	case outcomeSuccess, outcomeSkip:
		if a.nextAPI != "" {
			return e.recordAttempt(ctx, r, a, r.step, a.nextAPI, 1)
		}
		return e.advance(ctx, r, a)
	case outcomeRejected:
		e.finish(ctx, r, a.log, func(ctx context.Context, tx pgx.Tx, r runRef) (string, error) {
			if err := applyOf(a)(ctx, tx); err != nil {
				return "", err
			}
			return e.reject(ctx, tx, r)
		})
		return false
	}
	max, gap, err := c.retry(ctx, r.api)
	if err != nil {
		e.stopBroken(ctx, r, err)
		return false
	}
	if r.attempt >= 1+max {
		e.finish(ctx, r, a.log, e.fail)
		return false
	}
	if !e.recordAttempt(ctx, r, attempt{log: a.log}, r.step, r.api, r.attempt+1) {
		return false
	}
	return a.immediate || e.wait(ctx, gap)
}

var errNotSaved = errors.New("result not saved")

// savedOr wraps a pass's change so that its failure is reported as
// errNotSaved and flagged in *notSaved.
func savedOr(notSaved *bool, apply func(context.Context, pgx.Tx) error) func(context.Context, pgx.Tx) error {
	return func(ctx context.Context, tx pgx.Tx) error {
		if err := apply(ctx, tx); err != nil {
			*notSaved = true
			return fmt.Errorf("%w: %v", errNotSaved, err)
		}
		return nil
	}
}

// unrecorded reports whether a failure to record must stop the run broken:
// not when the executor stops, lost the run, or a result was not saved.
func unrecorded(ctx context.Context, err error) bool {
	return err != nil && ctx.Err() == nil && !errors.Is(err, errRunLost) && !errors.Is(err, errNotSaved)
}

// wait sleeps d unless the execution stops first; it reports whether to go on.
func (e *Executor) wait(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// nextStep is the step after step, or "" after the last.
func nextStep(step string) string {
	i := slices.Index(checkOrder, step)
	if i < 0 || i+1 >= len(checkOrder) {
		return ""
	}
	return checkOrder[i+1]
}

// advance records a passed attempt and moves to the next step. After the
// SLIK requests the run waits for their results (FR-PSAPI-011) unless they
// are all in already.
func (e *Executor) advance(ctx context.Context, r runRef, a attempt) bool {
	next := nextStep(r.step)
	if next == "" {
		// Every check passed: Pipeline Data Entry (FR-APP-004) is ticket 15.
		e.finish(ctx, r, a.log, func(ctx context.Context, tx pgx.Tx, _ runRef) (string, error) {
			return runCompleted, applyOf(a)(ctx, tx)
		})
		return false
	}
	if r.step != stepSLIKSpouse {
		return e.recordAttempt(ctx, r, a, next, "", 1)
	}
	waiting := false
	err := e.inOwnedRun(ctx, r, a.log, func(tx pgx.Tx) error {
		if err := applyOf(a)(ctx, tx); err != nil {
			return err
		}
		pending, err := pendingSLIK(ctx, tx, r.caseID)
		if err != nil {
			return err
		}
		state, deadline := runRunning, (*time.Time)(nil)
		if pending {
			sla, err := e.s.settings.Int(ctx, "process/slik/wait_sla_seconds")
			if err != nil {
				return err
			}
			d := e.s.now().Add(time.Duration(sla) * time.Second)
			state, deadline, waiting = runWaitingSLIK, &d, true
		}
		_, err = tx.Exec(ctx, `UPDATE fos.prescreening_runs SET state = $2, current_step = $3, current_api = NULL, attempt = 1,
			wait_deadline_at = $4, heartbeat_at = $5, updated_at = $5 WHERE run_id = $1`, r.id, state, next, deadline, e.s.now())
		return err
	})
	if err != nil {
		if unrecorded(ctx, err) {
			e.stopBroken(ctx, r, fmt.Errorf("attempt not recorded: %w", err))
		}
		return false
	}
	return !waiting
}

// applyOf is the attempt's extra change, or none.
func applyOf(a attempt) func(context.Context, pgx.Tx) error {
	if a.apply == nil {
		return func(context.Context, pgx.Tx) error { return nil }
	}
	return a.apply
}

// recordAttempt writes the attempt's log row, its extra change and the
// run's next position (step, API inside it, attempt) in one transaction,
// only while this executor still owns the run.
func (e *Executor) recordAttempt(ctx context.Context, r runRef, a attempt, step, api string, attemptNumber int) bool {
	err := e.inOwnedRun(ctx, r, a.log, func(tx pgx.Tx) error {
		if err := applyOf(a)(ctx, tx); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE fos.prescreening_runs SET current_step = $2, current_api = NULLIF($3, ''), attempt = $4,
			heartbeat_at = $5, updated_at = $5 WHERE run_id = $1`, r.id, step, api, attemptNumber, e.s.now())
		return err
	})
	if err != nil {
		if unrecorded(ctx, err) {
			e.stopBroken(ctx, r, fmt.Errorf("attempt not recorded: %w", err))
		}
		return false
	}
	return true
}

// finish writes the attempt's log row (if any) and stops the run with the
// state end returns, applying end's case changes in the same transaction.
func (e *Executor) finish(ctx context.Context, r runRef, log *apiLog, end func(context.Context, pgx.Tx, runRef) (string, error)) {
	err := e.inOwnedRun(ctx, r, log, func(tx pgx.Tx) error {
		state, err := end(ctx, tx, r)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE fos.prescreening_runs SET state = $2, updated_at = $3 WHERE run_id = $1`, r.id, state, e.s.now())
		return err
	})
	if unrecorded(ctx, err) {
		e.stopBroken(ctx, r, fmt.Errorf("run not finished: %w", err))
	}
}

// stopBroken stops the run on a system failure: a missing setting, or a
// result that could not be saved (e.g. the Rejected List entry). The run
// never repeats a step forever (FR-PSAPI-001 rule 9); case_status does not
// change (rule 10); the Problem Flow is ticket 14.
func (e *Executor) stopBroken(ctx context.Context, r runRef, cause error) {
	slog.Error("Pre-Screening Check run stopped by a system failure", "run_id", r.id, "case_id", r.caseID, "step", r.step, "error", cause)
	err := e.inOwnedRun(ctx, r, nil, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE fos.prescreening_runs SET state = $2, updated_at = $3 WHERE run_id = $1`, r.id, runStoppedBroken, e.s.now())
		return err
	})
	if err != nil && ctx.Err() == nil && !errors.Is(err, errRunLost) {
		slog.Error("Pre-Screening Check run could not be stopped", "run_id", r.id, "error", err)
	}
}

var errRunLost = errors.New("run no longer owned")

// inOwnedRun locks the run, checks this executor still owns it RUNNING,
// appends the log row (DB-RULE-041) and applies change, all in one
// transaction, so a step is never recorded twice.
func (e *Executor) inOwnedRun(ctx context.Context, r runRef, log *apiLog, change func(pgx.Tx) error) error {
	tx, err := e.s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var owner *string
	var state string
	if err := tx.QueryRow(ctx, `SELECT owner_id, state FROM fos.prescreening_runs WHERE run_id = $1 FOR UPDATE`, r.id).Scan(&owner, &state); err != nil {
		return err
	}
	if value(owner) != e.owner || state != runRunning {
		return errRunLost
	}
	if log != nil {
		if err := log.insert(ctx, tx, r.caseID); err != nil {
			return err
		}
	}
	if err := change(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// reject is a Check Rejection (FR-PSAPI-001 Process 3): the case goes to
// the alternate stage Rejected, and its Customer and Spouse to the Rejected
// List unless the Rejected List itself rejected it (FR-PSAPI-007).
func (e *Executor) reject(ctx context.Context, tx pgx.Tx, r runRef) (string, error) {
	reason := ""
	if r.step != stepRejectedList {
		reason = rejectionReason(e.apiNameOf(r.step), e.checks[r.step].criterion)
	}
	return runStoppedRejected, e.rejectCase(ctx, tx, r.caseID, reason)
}

// rejectCase moves the case to Rejected and, given a reason, records its
// Customer and Spouse on the Rejected List, in the caller's transaction.
func (e *Executor) rejectCase(ctx context.Context, tx pgx.Tx, caseID, reason string) error {
	if reason != "" {
		if err := e.recordRejection(ctx, tx, caseID, reason); err != nil {
			return err
		}
	}
	now := e.s.now()
	if _, err := tx.Exec(ctx, `
		INSERT INTO fos.case_stage_progress (case_id, stage_name, first_entered_at, entered_from_stage)
		SELECT case_id, $2, $3, current_stage FROM fos.work WHERE case_id = $1
		ON CONFLICT (case_id, stage_name) DO UPDATE SET entered_from_stage = EXCLUDED.entered_from_stage`,
		caseID, StageRejected, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE fos.work SET case_status = $2, current_stage = $3, updated_at = $4 WHERE case_id = $1`,
		caseID, CaseStatusRejectedPreScreening, StageRejected, now)
	return err
}

// fail is a Check Failure (FR-PSAPI-001 Process 4): the case returns to its
// Team Lead on a new Pre-Screening Review assignment, shown as the Retry view.
func (e *Executor) fail(ctx context.Context, tx pgx.Tx, r runRef) (string, error) {
	now := e.s.now()
	if _, err := tx.Exec(ctx, `UPDATE fos.work SET case_status = $2, updated_at = $3 WHERE case_id = $1`,
		r.caseID, CaseStatusFailedPreScreening, now); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
		SELECT case_id, $2, $3, branch_officer_team_lead, $4, $5 FROM fos.work WHERE case_id = $1`,
		r.caseID, AssignmentPreScreeningReview, string(Worklist), string(AssignmentNew), now); err != nil {
		return "", err
	}
	return runStoppedFailed, nil
}

// startRun creates the case's run at the first step, owned by this
// executor, inside the caller's transaction; launch it after the commit.
func (e *Executor) startRun(ctx context.Context, tx pgx.Tx, caseID string) (int64, error) {
	now := e.s.now()
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO fos.prescreening_runs (case_id, state, current_step, attempt, owner_id, heartbeat_at, started_at, updated_at)
		VALUES ($1, $2, $3, 1, $4, $5, $5, $5) RETURNING run_id`, caseID, runRunning, checkOrder[0], e.owner, now).Scan(&id)
	return id, err
}

// latestRun is the case's newest run (§9.15 keeps older ones as history).
const latestRun = `(SELECT max(run_id) FROM fos.prescreening_runs WHERE case_id = $1)`

// resumeRun restarts the case's failed run at its failed step with a fresh
// attempt cycle (FR-PSAPI-001 rule 5), inside the caller's transaction.
func (e *Executor) resumeRun(ctx context.Context, tx pgx.Tx, caseID string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		UPDATE fos.prescreening_runs SET state = $2, attempt = 1, owner_id = $3, heartbeat_at = $4, updated_at = $4
		WHERE run_id = `+latestRun+` AND state = $5
		RETURNING run_id`, caseID, runRunning, e.owner, e.s.now(), runStoppedFailed).Scan(&id)
	if isNotFound(err) {
		return 0, ErrUnsupported
	}
	return id, err
}

// apiNameOf is the Connect API Log api_name of a step, or the step itself.
func (e *Executor) apiNameOf(step string) string {
	if c, ok := e.checks[step]; ok {
		return c.apiName
	}
	return step
}
