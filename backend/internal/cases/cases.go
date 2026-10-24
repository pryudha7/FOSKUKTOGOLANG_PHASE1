// Package cases creates Financing Cases and serves Case View (FR-APP-001,
// FR-CASE-001 to FR-CASE-008, FR-ASSIGN-001).
package cases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"fos/internal/auth"
	"fos/internal/settings"
)

// CaseType is a parent case type an operator can create (FR-NAV-003). Code
// is the value stored in work.case_type; Label is what the UI shows.
type CaseType struct {
	Code  string
	Label string
}

// FinancingCase is the one parent case type today; its stored code is the
// SoT's "Loan Financing" and its label "Financing" (FR-CASE-002).
var FinancingCase = CaseType{Code: "Loan Financing", Label: "Financing"}

// caseTypes maps stored codes to case types.
var caseTypes = map[string]CaseType{FinancingCase.Code: FinancingCase}

// caseTypeOf returns the case type for a stored code; an unknown code is
// shown as itself.
func caseTypeOf(code string) CaseType {
	if t, ok := caseTypes[code]; ok {
		return t
	}
	return CaseType{Code: code, Label: code}
}

// CreatableCaseTypes lists the case types an operator may create. Only the
// AO and SAO roles create cases, matched on the stable role_code (§9.3), and
// never an operator in the Authors access group (FR-APP-001, DEC-029, DEC-030).
func CreatableCaseTypes(sess auth.Session) []CaseType {
	if sess.AccessGroup != nil && sess.AccessGroup.Name == "Authors" {
		return []CaseType{}
	}
	if sess.Role.Code == "AO" || sess.Role.Code == "SAO" {
		return []CaseType{FinancingCase}
	}
	return []CaseType{}
}

// Case and assignment vocabulary (SoT §8.1, §8.2, §9.5).
const (
	StageFinancingSubmission = "Financing Submission"
	CaseStatusNew            = "New"

	AssignmentPreScreeningSubmission = "Pre-Screening Submission"
	ProcessPreScreeningSubmission    = "Pre-Screening Submission"
)

// AssignmentType and AssignmentStatus hold the SoT's words (§9.5).
type (
	AssignmentType   string
	AssignmentStatus string
)

const (
	Worklist AssignmentType = "worklist"

	AssignmentNew       AssignmentStatus = "baru"
	AssignmentOpened    AssignmentStatus = "sudah dibuka"
	AssignmentCompleted AssignmentStatus = "sudah selesai dikerjakan"
)

// MainStages are WF-001 to WF-008 in order; AlternateStages are WF-009 and
// WF-010, shown only once entered (FR-CASE-004).
var (
	MainStages = []string{
		StageFinancingSubmission, "Financing Application", "Financing Checker", "Survey Analyst",
		"Recommendation", "Pre-Disbursement", "FAC Checker", "Disbursement",
	}
	AlternateStages = []string{"Rejected", "Financing Revision"}
)

var (
	// ErrNotPermitted is ERR-022.
	ErrNotPermitted = errors.New("only AO and SAO create cases")
	// ErrUnknownCaseType: the requested type is not a creatable parent type.
	ErrUnknownCaseType = errors.New("unknown case type")
	// ErrCaseIDNotFormed is ERR-005: no unique Case ID could be formed.
	ErrCaseIDNotFormed = errors.New("case id not formed")
	// ErrNotFound: no such case or assignment.
	ErrNotFound = errors.New("not found")
	// ErrForbidden: the operator may not open the assignment.
	ErrForbidden = errors.New("forbidden")
)

type Service struct {
	db       *pgxpool.Pool
	now      func() time.Time
	storage  Storage
	settings *settings.Settings
	runs     *Executor
}

func NewService(db *pgxpool.Pool, now func() time.Time, storage Storage, settings *settings.Settings) *Service {
	s := &Service{db: db, now: now, storage: storage, settings: settings}
	s.runs = newExecutor(s)
	return s
}

// StartRuns starts the Pre-Screening Check executor: it resumes runs whose
// executor died, now and every minute (FR-PSAPI-002 Process 5).
func (s *Service) StartRuns() { s.runs.Start() }

// Now is the service clock (UTC).
func (s *Service) Now() time.Time { return s.now() }

// Close stops the executor and waits for its runs to pause.
func (s *Service) Close() { s.runs.Close() }

// Created identifies a new case and its first assignment.
type Created struct {
	CaseID       string
	AssignmentID int64
}

// Create makes a Financing Case for the session's operator (FR-APP-001):
// the Work row with its initial values, the stage and process progress, and
// the opened Pre-Screening Submission assignment, in one transaction.
func (s *Service) Create(ctx context.Context, creator auth.Session, caseType string) (Created, error) {
	allowed := CreatableCaseTypes(creator)
	if len(allowed) == 0 {
		return Created{}, ErrNotPermitted
	}
	if !containsCode(allowed, caseType) {
		return Created{}, ErrUnknownCaseType
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Created{}, err
	}
	defer tx.Rollback(ctx)

	var branchCode, reportTo *string
	var timezone string
	if err := tx.QueryRow(ctx,
		`SELECT branch_code, report_to, timezone FROM fos.operator WHERE user_id = $1`, creator.UserID).
		Scan(&branchCode, &reportTo, &timezone); err != nil {
		return Created{}, err
	}
	loc, err := operatorLocation(timezone)
	if err != nil {
		return Created{}, err
	}

	// FR-CASE-001: the number comes from the prefix's sequence, atomically.
	var n int64
	if err := tx.QueryRow(ctx, `SELECT nextval('fos.case_id_lf_seq')`).Scan(&n); err != nil {
		slog.Error("case id sequence failed", "error", err)
		return Created{}, ErrCaseIDNotFormed
	}
	caseID := "LF-" + strconv.FormatInt(n, 10)
	now := s.now()

	// DB-RULE-020 then DB-RULE-022.
	if _, err := tx.Exec(ctx, `
		INSERT INTO fos.work (case_id, case_type, case_status, current_stage, branch_code, urgency_work_class,
			branch_officer, branch_officer_team_lead, customer_code, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, 10, $6, $7, $8, $9, $9, $6, $6)`,
		caseID, FinancingCase.Code, CaseStatusNew, StageFinancingSubmission, branchCode,
		creator.UserID, reportTo, customerCode(now.In(loc), n), now); err != nil {
		return Created{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fos.case_stage_progress (case_id, stage_name, first_entered_at) VALUES ($1, $2, $3)`,
		caseID, StageFinancingSubmission, now); err != nil {
		return Created{}, err
	}
	if err := startProcess(ctx, tx, caseID, ProcessPreScreeningSubmission, now); err != nil {
		return Created{}, err
	}
	// FR-ASSIGN-001 rule 1: a worklist assignment for the creator, already opened.
	var assignmentID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id,
			assignment_status, created_at, first_opened_at, worked_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $6, $4) RETURNING assignment_id`,
		caseID, AssignmentPreScreeningSubmission, string(Worklist), creator.UserID, string(AssignmentOpened), now).
		Scan(&assignmentID); err != nil {
		return Created{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Created{}, err
	}
	return Created{CaseID: caseID, AssignmentID: assignmentID}, nil
}

func containsCode(types []CaseType, code string) bool {
	for _, t := range types {
		if t.Code == code {
			return true
		}
	}
	return false
}

// customerCode is "SMSME" + two-digit year + the Case ID number padded to
// eight digits (DB-RULE-022).
func customerCode(at time.Time, n int64) string {
	return fmt.Sprintf("SMSME%02d%08d", at.Year()%100, n)
}

// operatorLocation reads the operator's timezone: the ENT-005 form "GMT+7"
// (also GMT-3, GMT+5:30) or an IANA name (TBD-030).
func operatorLocation(tz string) (*time.Location, error) {
	rest, ok := strings.CutPrefix(tz, "GMT")
	if !ok {
		return time.LoadLocation(tz)
	}
	if rest == "" {
		return time.UTC, nil
	}
	sign := 1
	switch rest[0] {
	case '-':
		sign = -1
	case '+':
	default:
		return nil, fmt.Errorf("operator timezone %q", tz)
	}
	hh, mm, _ := strings.Cut(rest[1:], ":")
	h, err := strconv.Atoi(hh)
	if err != nil {
		return nil, fmt.Errorf("operator timezone %q", tz)
	}
	m := 0
	if mm != "" {
		if m, err = strconv.Atoi(mm); err != nil {
			return nil, fmt.Errorf("operator timezone %q", tz)
		}
	}
	return time.FixedZone(tz, sign*(h*3600+m*60)), nil
}

func isNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
