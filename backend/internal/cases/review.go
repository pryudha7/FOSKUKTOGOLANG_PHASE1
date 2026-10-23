package cases

import (
	"context"
	"fmt"
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
	// CaseStatusPendingPrescreening follows a Sesuai decision (FR-APP-010 step 5).
	CaseStatusPendingPrescreening = "Pending-Prescreening"

	KeyDecision = "decision"
	KeyNotes    = "notes"

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
}

// NotesBanner is the latest Pre Screening decision shown on UI-010 once the
// case has one (FR-APP-010 rule 2).
type NotesBanner struct {
	Decision string
	Notes    string
}

// recommendations lists the case's Pre Screening Recommendations, oldest
// first (DEC-051, DB-RULE-123). The description is read by code without a
// status filter (FR-APP-010 rule 5).
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
		SELECT COALESCE(o.full_name, d.decided_by), d.decided_at, COALESCE(b.description, d.decision_code, ''), d.notes
		FROM fos.case_pre_screening_decision d
		LEFT JOIN fos.operator o ON o.user_id = d.decided_by
		LEFT JOIN fos.business_mapping_value b ON b.type = $2 AND b.sub_type = '' AND b.code = d.decision_code
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

// notesBanner is the latest Pre Screening decision, or nil (DB-RULE-036).
func (s *Service) notesBanner(ctx context.Context, caseID string) (*NotesBanner, error) {
	var b NotesBanner
	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(bmv.description, d.decision_code, ''), d.notes
		FROM fos.case_pre_screening_decision d
		LEFT JOIN fos.business_mapping_value bmv ON bmv.type = $2 AND bmv.sub_type = '' AND bmv.code = d.decision_code
		WHERE d.case_id = $1 AND d.recommendation_type = $3
		ORDER BY d.decided_at DESC, d.decision_id DESC LIMIT 1`, caseID, bmvDecision, recommendationPreScreening).
		Scan(&b.Decision, &b.Notes)
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Service) reviewPage(ctx context.Context, sess auth.Session, caseID string) (ReviewPage, error) {
	var (
		page ReviewPage
		err  error
	)
	if page.DecisionOptions, err = s.options(ctx, bmvDecision); err != nil {
		return ReviewPage{}, err
	}
	loc, err := s.operatorLocationOf(ctx, sess.UserID)
	if err != nil {
		return ReviewPage{}, err
	}
	page.History, err = s.recommendations(ctx, caseID, loc)
	return page, err
}

// operatorLocationOf is an operator's timezone (BR-UI-003).
func (s *Service) operatorLocationOf(ctx context.Context, userID string) (*time.Location, error) {
	var timezone string
	if err := s.db.QueryRow(ctx, `SELECT timezone FROM fos.operator WHERE user_id = $1`, userID).Scan(&timezone); err != nil {
		return nil, err
	}
	return operatorLocation(timezone)
}

// SubmitReview is Submit on Pre Screening (FR-APP-010): it validates
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

	decisionCode = strings.TrimSpace(decisionCode)
	notes = strings.ReplaceAll(strings.TrimSpace(notes), "\r\n", "\n")
	var errs []FieldError
	if decisionCode == "" {
		errs = append(errs, required(SectionForm, KeyDecision, "Keputusan"))
	} else {
		opts, err := optionsIn(ctx, tx, bmvDecision)
		if err != nil {
			return err
		}
		if !hasCode(opts, decisionCode) {
			return ErrUnsupported // not an option the page offered
		}
	}
	switch {
	case notes == "":
		errs = append(errs, required(SectionForm, KeyNotes, "Notes"))
	case utf8.RuneCountInString(notes) > maxNotesRunes:
		errs = append(errs, FieldError{SectionForm, KeyNotes, "Notes", "ERR-030", msgNotesLength})
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
		if _, err := tx.Exec(ctx, `UPDATE fos.work SET case_status = $2 WHERE case_id = $1`, caseID, CaseStatusPendingPrescreening); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE fos.work SET updated_at = $2, updated_by = $3 WHERE case_id = $1`, caseID, now, sess.UserID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
