package cases

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"fos/internal/auth"
)

// Pre-Screening Review vocabulary (FR-APP-010, §9.12).
const (
	recommendationPreScreening = "Pre Screening"
	bmvDecision                = "FAC_DECISION"
	decisionMatch              = "SESUAI"
	decisionMismatch           = "TIDAKSESUAI"
	// CaseStatusPendingPreScreening follows a Sesuai decision (FR-APP-010 step 5).
	CaseStatusPendingPreScreening = "Pending-Prescreening"

	keyDecision = "decision"
	keyNotes    = "notes"

	msgNotesLength = "Notes maksimal 1000 karakter" // ERR-030, SoT PROPOSED text
	// maxNotesRunes bounds Notes, counted after CRLF becomes LF (DB-RULE-037).
	maxNotesRunes = 1000
)

// Recommendation is one entry of History Pre-screening Notes (UI-017).
type Recommendation struct {
	// DecidedBy is the full name of decided_by from the operator data.
	DecidedBy string
	// SubmittedAt is in the viewing operator's timezone, dd/MM/yy H:mm.
	SubmittedAt string
	Decision    string // FAC_DECISION description
	Notes       string
}

// ReviewPage is UI-017 for the Team Lead.
type ReviewPage struct {
	DecisionOptions []Option
	History         []Recommendation
	Editable        bool
	// Retry is set when the case returned after a Check Failure (UI-017
	// Varian Retry); the page then has no decision fields.
	Retry *APILogView
	// SLIKTimeout is set after Manual SLIK Entry (UI-017 Varian Timeout
	// SLIK): the SLIK rows of the run, above the history and the decision.
	SLIKTimeout *APILogView
}

// APILogView is an API Log History panel: UI-020 rows (Gambar 35). On the
// Retry view, the failed API's attempts of its last cycle (agreed for
// TBD-042); on the Timeout SLIK view, the run's SLIK rows (agreed for
// TBD-048g).
type APILogView struct {
	Logs []APILogEntry
}

// APILogEntry is one Connect API Log row as UI-020 shows it.
type APILogEntry struct {
	Name, SubmittedAt, Status, Request, Response string
}

// recommendations lists the case's Pre-Screening Review Recommendations,
// oldest first (DEC-051, DB-RULE-123). The description is read by code
// without a status filter (FR-APP-010 rule 5).
func (s *Service) recommendations(ctx context.Context, caseID string, viewer *time.Location) ([]Recommendation, error) {
	out, err := collect(ctx, s, func(r pgx.Rows) (Recommendation, error) {
		var (
			rec Recommendation
			at  time.Time
		)
		err := r.Scan(&rec.DecidedBy, &at, &rec.Decision, &rec.Notes)
		rec.SubmittedAt = submittedAt(at.In(viewer))
		return rec, err
	}, `
		SELECT COALESCE(o.full_name, d.decided_by), d.decided_at, COALESCE(bmv.description, d.decision_code, ''), d.notes
		FROM fos.case_pre_screening_decision d
		LEFT JOIN fos.operator o ON o.user_id = d.decided_by
		LEFT JOIN fos.business_mapping_value bmv ON bmv.type = $2 AND bmv.sub_type = '' AND bmv.code = d.decision_code
		WHERE d.case_id = $1 AND d.recommendation_type = $3
		ORDER BY d.decided_at, d.decision_id`, caseID, bmvDecision, recommendationPreScreening)
	if out == nil {
		out = []Recommendation{}
	}
	return out, err
}

// submittedAt is Tanggal & Waktu Submit, dd/MM/yy H:mm: the hour without a
// leading zero, which Go layouts cannot express.
func submittedAt(t time.Time) string {
	return fmt.Sprintf("%s %d:%02d", t.Format("02/01/06"), t.Hour(), t.Minute())
}

// latestRecommendation is the latest Pre-Screening Review decision, shown
// as UI-010's Notes banner, or nil before the first (FR-APP-010 rule 2,
// DB-RULE-036: the last of the oldest-first order).
func (s *Service) latestRecommendation(ctx context.Context, caseID string) (*Recommendation, error) {
	all, err := s.recommendations(ctx, caseID, time.UTC)
	if err != nil || len(all) == 0 {
		return nil, err
	}
	return &all[len(all)-1], nil
}

func (s *Service) reviewPage(ctx context.Context, sess auth.Session, caseID string) (ReviewPage, error) {
	var (
		page ReviewPage
		err  error
	)
	if page.DecisionOptions, err = s.options(ctx, bmvDecision); err != nil {
		return ReviewPage{}, err
	}
	// A viewer whose timezone cannot be read still sees the page, in UTC.
	loc, err := operatorLocationOf(ctx, s.db, sess.UserID)
	if err != nil {
		slog.Error("operator timezone unreadable; showing times in UTC", "user_id", sess.UserID, "error", err)
		loc = time.UTC
	}
	if page.History, err = s.recommendations(ctx, caseID, loc); err != nil {
		return ReviewPage{}, err
	}
	retrying, err := s.retrying(ctx, s.db, caseID)
	if err != nil {
		return ReviewPage{}, err
	}
	logEntry := func(r pgx.Rows) (APILogEntry, error) {
		var (
			l  APILogEntry
			at time.Time
		)
		err := r.Scan(&l.Name, &at, &l.Status, &l.Request, &l.Response)
		l.SubmittedAt = submittedAt(at.In(loc))
		return l, err
	}
	if !retrying {
		manual, started, err := inManualSLIKEntry(ctx, s.db, caseID)
		if err != nil || !manual {
			return page, err
		}
		// The run's SLIK rows: requests, results and the timeout or Bypass row
		// (agreed for TBD-048g).
		logs, err := collect(ctx, s, logEntry, `
			SELECT api_name, submitted_at, api_status, COALESCE(request, ''), COALESCE(response, '')
			FROM fos.connect_api_log
			WHERE case_id = $1 AND api_name IN ($2, $3, $4) AND submitted_at >= $5
			ORDER BY submitted_at, attempt_number`, caseID, slikSubjects[subjectCustomer].api.logName, slikSubjects[subjectSpouse].api.logName, apiPBSResponse, started)
		if err != nil {
			return ReviewPage{}, err
		}
		page.SLIKTimeout = &APILogView{Logs: logs}
		return page, nil
	}
	// The failed API's attempts of the newest run, its last cycle only: the
	// API inside the step when it has several (Pefindo), else the step's.
	var step, api string
	var attempts int
	var started time.Time
	if err := s.db.QueryRow(ctx, `SELECT current_step, COALESCE(current_api, ''), attempt, started_at FROM fos.prescreening_runs WHERE run_id = `+latestRun+` AND state = $2`,
		caseID, runStoppedFailed).Scan(&step, &api, &attempts, &started); err != nil && !isNotFound(err) {
		return ReviewPage{}, err
	}
	if api == "" {
		api = s.runs.apiNameOf(step)
	}
	logs, err := collect(ctx, s, logEntry, `
		SELECT * FROM (
			SELECT api_name, submitted_at, api_status, COALESCE(request, ''), COALESCE(response, '')
			FROM fos.connect_api_log
			WHERE case_id = $1 AND api_name = $2 AND submitted_at >= $3
			ORDER BY submitted_at DESC, attempt_number DESC
			LIMIT $4
		) last ORDER BY submitted_at`, caseID, api, started, attempts)
	if err != nil {
		return ReviewPage{}, err
	}
	page.Retry = &APILogView{Logs: logs}
	return page, nil
}

// SubmitReview is Submit on the Pre-Screening Review (FR-APP-010): it validates
// Keputusan and Notes, appends the Recommendation and completes the
// assignment. Tidak Sesuai gives the case's Branch Officer a new
// Pre-Screening Submission; Sesuai sets case_status Pending-Prescreening (the
// Pre-Screening Check run is ticket 09). Branching uses the code.
func (s *Service) SubmitReview(ctx context.Context, sess auth.Session, caseID string, assignmentID int64, decisionCode, notes string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	a, err := ownOpenAssignment(ctx, tx, sess, caseID, assignmentID)
	if err != nil {
		return err
	}
	if a.name != AssignmentPreScreeningReview {
		return ErrUnsupported
	}
	// The Retry view has no decision to submit.
	if retrying, err := s.retrying(ctx, tx, caseID); err != nil || retrying {
		if err != nil {
			return err
		}
		return ErrUnsupported
	}
	var runID int64

	decisionCode = strings.TrimSpace(decisionCode)
	notes = strings.TrimSpace(strings.ReplaceAll(notes, "\r\n", "\n"))
	var errs []FieldError
	if decisionCode == "" {
		errs = append(errs, required(SectionForm, keyDecision, "Keputusan"))
	} else {
		opts, err := optionsIn(ctx, tx, bmvDecision)
		if err != nil {
			return err
		}
		// Not an option the page offered, or an option this branching does
		// not know, which would leave the case with no next step.
		if !hasCode(opts, decisionCode) || (decisionCode != decisionMatch && decisionCode != decisionMismatch) {
			return ErrUnsupported
		}
	}
	switch {
	case notes == "":
		errs = append(errs, required(SectionForm, keyNotes, "Notes"))
	case utf8.RuneCountInString(notes) > maxNotesRunes:
		errs = append(errs, FieldError{SectionForm, keyNotes, "Notes", "ERR-030", msgNotesLength})
	}
	if len(errs) > 0 {
		return &ValidationError{errs}
	}

	var branchOfficer string
	if err := tx.QueryRow(ctx, `SELECT branch_officer FROM fos.work WHERE case_id = $1`, caseID).Scan(&branchOfficer); err != nil {
		return err
	}
	// DB-RULE-116: Sesuai records the case's Branch Officer, Tidak Sesuai the Team Lead.
	decidedBy := sess.UserID
	if decisionCode == decisionMatch {
		decidedBy = branchOfficer
	}
	now := s.now()
	// DB-RULE-117: the decider's role as it is now, kept as a snapshot.
	tag, err := tx.Exec(ctx, `
		INSERT INTO fos.case_pre_screening_decision (case_id, recommendation_type, decision_code, notes, decided_by,
			decided_by_role_code, decided_at)
		SELECT $1, $2, $3, $4, $5::varchar, role_code, $6 FROM fos.operator WHERE user_id = $5::varchar`,
		caseID, recommendationPreScreening, decisionCode, notes, decidedBy, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("recommendation not recorded: %s is not in the operator data", decidedBy)
	}
	if _, err := tx.Exec(ctx, `UPDATE fos.assignments SET assignment_status = $2, completed_at = $3 WHERE assignment_id = $1`,
		a.id, string(AssignmentCompleted), now); err != nil {
		return err
	}
	switch decisionCode {
	case decisionMismatch:
		// FR-APP-010 step 4: back to the case's Branch Officer; case_status unchanged (DEC-045).
		if _, err := tx.Exec(ctx, `
			INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			caseID, AssignmentPreScreeningSubmission, string(Worklist), branchOfficer, string(AssignmentNew), now); err != nil {
			return err
		}
	case decisionMatch:
		// FR-PSAPI-001 Process 1: the run starts; Submit returns at once (DEC-095).
		if _, err := tx.Exec(ctx, `UPDATE fos.work SET case_status = $2 WHERE case_id = $1`, caseID, CaseStatusPendingPreScreening); err != nil {
			return err
		}
		// After Manual SLIK Entry the run goes on at Pefindo (agreed for ticket 12).
		manual, _, err := inManualSLIKEntry(ctx, tx, caseID)
		if err != nil {
			return err
		}
		start := s.runs.startRun
		if manual {
			start = s.runs.resumeAfterManualSLIK
		}
		if runID, err = start(ctx, tx, caseID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE fos.work SET updated_at = $2, updated_by = $3 WHERE case_id = $1`, caseID, now, sess.UserID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if runID != 0 {
		s.runs.launch(runID)
	}
	return nil
}

// RetryReview is Submit on the Retry view (FR-PSAPI-001 rule 5): the case
// returns to Pending-Prescreening and its run resumes at the check that
// failed, with a fresh attempt cycle. No Recommendation is added.
func (s *Service) RetryReview(ctx context.Context, sess auth.Session, caseID string, assignmentID int64) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	a, err := ownOpenAssignment(ctx, tx, sess, caseID, assignmentID)
	if err != nil {
		return err
	}
	if a.name != AssignmentPreScreeningReview {
		return ErrUnsupported
	}
	retrying, err := s.retrying(ctx, tx, caseID)
	if err != nil {
		return err
	}
	if !retrying {
		return ErrUnsupported
	}
	now := s.now()
	if _, err := tx.Exec(ctx, `UPDATE fos.assignments SET assignment_status = $2, completed_at = $3 WHERE assignment_id = $1`,
		a.id, string(AssignmentCompleted), now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE fos.work SET case_status = $2, updated_at = $3, updated_by = $4 WHERE case_id = $1`,
		caseID, CaseStatusPendingPreScreening, now, sess.UserID); err != nil {
		return err
	}
	runID, err := s.runs.resumeRun(ctx, tx, caseID)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.runs.launch(runID)
	return nil
}

// retrying reports whether the case waits on its Team Lead after a Check
// Failure, which shows Pre Screening as the Retry view.
func (s *Service) retrying(ctx context.Context, q rowQuerier, caseID string) (bool, error) {
	var status *string
	if err := q.QueryRow(ctx, `SELECT case_status FROM fos.work WHERE case_id = $1`, caseID).Scan(&status); err != nil {
		return false, err
	}
	return value(status) == CaseStatusFailedPreScreening, nil
}
