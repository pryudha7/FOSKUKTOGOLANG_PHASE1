package cases

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Manual SLIK Entry (FR-PSAPI-011, FR-PSAPI-019): when the SLIK results do
// not all arrive within the SLA, or a failed SLIK request is bypassed, the
// System records an Input Manual Recommendation and the case returns to its
// Branch Officer, who uploads the SLIK document by hand (ERR-036). After the
// Team Lead's Sesuai the run goes on at Pefindo; SLIK is not repeated.

const (
	// runStoppedSLIKBypass is a run stopped by the bypass (FR-PSAPI-019,
	// PROPOSED, agreed for ticket 12).
	runStoppedSLIKBypass = "STOPPED_SLIK_BYPASS"

	// apiBypass is the Connect API Log status of a bypassed retry.
	apiBypass = "Bypass"

	operatorSystem      = "System"
	decisionInputManual = "INPUTMANUAL" // DB-RULE-065
	manualSLIKNotes     = "Hasil SLIK belum diterima sistem lebih dari waktu tunggu. Harap lanjutkan untuk unggah hasil SLIK secara manual."

	// docSLIK is the SLIK document of Dokumen Pre-Screening (FR-FILE-004 rule 7).
	docSLIK         = "SLIK"
	msgSLIKDocument = "Upload Dokumen SLIK" // ERR-036
)

// manualSLIKStates are the run states that hand the case back for Manual
// SLIK Entry: the SLA timeout and the bypass.
var manualSLIKStates = []string{runStoppedSLIKTimeout, runStoppedSLIKBypass}

// manualSLIK hands the case back for Manual SLIK Entry in tx: the System's
// Input Manual Recommendation, then a new Pre-Screening Submission for the
// case's Branch Officer, as Tidak Sesuai does (FR-PSAPI-011 Process 3c, 3d;
// agreed for ticket 12). case_status stays Pending-Prescreening.
func (e *Executor) manualSLIK(ctx context.Context, tx pgx.Tx, caseID string) error {
	now := e.s.now()
	tag, err := tx.Exec(ctx, `
		INSERT INTO fos.case_pre_screening_decision (case_id, recommendation_type, decision_code, notes, decided_by,
			decided_by_role_code, decided_at)
		SELECT $1, $2, $3, $4, user_id, role_code, $5 FROM fos.operator WHERE user_id = $6`,
		caseID, recommendationPreScreening, decisionInputManual, manualSLIKNotes, now, operatorSystem)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("Input Manual not recorded: the %s operator is missing", operatorSystem)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
		SELECT case_id, $2, $3, branch_officer, $4, $5 FROM fos.work WHERE case_id = $1`,
		caseID, AssignmentPreScreeningSubmission, string(Worklist), string(AssignmentNew), now)
	return err
}

// expireSLIKWaits times out the runs whose SLIK wait passed its deadline
// (FR-PSAPI-011 Process 3); the recovery sweep calls it (FR-PSAPI-002 rule 6).
func (e *Executor) expireSLIKWaits(ctx context.Context) {
	ids, err := collect(ctx, e.s, func(r pgx.Rows) (int64, error) {
		var id int64
		return id, r.Scan(&id)
	}, `SELECT run_id FROM fos.prescreening_runs WHERE state = $1 AND wait_deadline_at < $2`, runWaitingSLIK, e.s.now())
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("SLIK wait check failed", "error", err)
		}
		return
	}
	for _, id := range ids {
		if err := e.timeOutSLIK(ctx, id); err != nil && ctx.Err() == nil {
			slog.Error("SLIK timeout not recorded", "run_id", id, "error", err)
		}
	}
}

// timeOutSLIK records one run's SLIK timeout under the run's lock, which a
// PBSResponse call for the case takes too, so a result and the timeout never
// both act: one timeout log row (no reference code), STOPPED_SLIK_TIMEOUT
// and the hand-back, in one transaction.
func (e *Executor) timeOutSLIK(ctx context.Context, runID int64) error {
	tx, err := e.s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	now := e.s.now()
	var caseID string
	err = tx.QueryRow(ctx, `SELECT case_id FROM fos.prescreening_runs WHERE run_id = $1 AND state = $2 AND wait_deadline_at < $3 FOR UPDATE`,
		runID, runWaitingSLIK, now).Scan(&caseID)
	if isNotFound(err) {
		return nil // the results came in, or another executor timed it out
	}
	if err != nil {
		return err
	}
	if pending, err := pendingSLIK(ctx, tx, caseID); err != nil || !pending {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fos.connect_api_log (log_id, case_id, api_type, api_name, submitted_at, api_status, response)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, uuid.New(), caseID, apiTypeServiceRest, apiPBSResponse, now, apiFail, timeoutResponse); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE fos.prescreening_runs SET state = $2, updated_at = $3 WHERE run_id = $1`, runID, runStoppedSLIKTimeout, now); err != nil {
		return err
	}
	if err := completeWaitingSLIK(ctx, tx, caseID, now); err != nil {
		return err
	}
	if err := e.manualSLIK(ctx, tx, caseID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	slog.Info("SLIK wait timed out; the case returns for Manual SLIK Entry", "run_id", runID, "case_id", caseID)
	return nil
}

// slikBypass reads process/slik/bypass_enabled (FR-PSAPI-019 Configuration).
func (e *Executor) slikBypass(ctx context.Context) (bool, error) {
	return e.s.settings.Bool(ctx, "process/slik/bypass_enabled")
}

// bypass stops the run instead of the retry of a failed SLIK request: the
// failed attempt's row, then a Bypass row without a call (empty Request and
// Response), STOPPED_SLIK_BYPASS and the hand-back (FR-PSAPI-019 Process 2-4).
func (e *Executor) bypass(ctx context.Context, r runRef, failed *apiLog) {
	e.finish(ctx, r, failed, func(ctx context.Context, tx pgx.Tx, r runRef) (string, error) {
		row := &apiLog{name: failed.name, submittedAt: e.s.now(), status: apiBypass, attempt: r.attempt + 1}
		if err := row.insert(ctx, tx, r.caseID); err != nil {
			return "", err
		}
		return runStoppedSLIKBypass, e.manualSLIK(ctx, tx, r.caseID)
	})
}

// inManualSLIKEntry reports whether the case's latest run stopped for
// Manual SLIK Entry (timeout or bypass).
func inManualSLIKEntry(ctx context.Context, q rowQuerier, caseID string) (bool, time.Time, error) {
	var state string
	var started time.Time
	err := q.QueryRow(ctx, `SELECT state, started_at FROM fos.prescreening_runs WHERE run_id = `+latestRun, caseID).Scan(&state, &started)
	if isNotFound(err) {
		return false, started, nil
	}
	return slices.Contains(manualSLIKStates, state), started, err
}

// resumeAfterManualSLIK puts the case's run stopped for Manual SLIK Entry
// back to RUNNING at the step after SLIK (Pefindo), attempt 1, in the
// caller's transaction (FR-PSAPI-011 Process 6, agreed for ticket 12).
func (e *Executor) resumeAfterManualSLIK(ctx context.Context, tx pgx.Tx, caseID string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		UPDATE fos.prescreening_runs SET state = $2, current_step = $3, current_api = NULL, attempt = 1, owner_id = $4,
			heartbeat_at = $5, wait_deadline_at = NULL, updated_at = $5
		WHERE run_id = `+latestRun+` AND state = ANY($6)
		RETURNING run_id`, caseID, runRunning, nextStep(stepSLIKSpouse), e.owner, e.s.now(), manualSLIKStates).Scan(&id)
	return id, err
}
