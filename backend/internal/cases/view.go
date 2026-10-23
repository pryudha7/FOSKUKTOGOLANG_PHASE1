package cases

import (
	"context"
	"slices"
	"time"

	"fos/internal/auth"
)

// Person is an operator shown by full name, read from the operator data at
// display time (DB-RULE-027).
type Person struct {
	UserID string
	Name   string
	At     time.Time
}

// StageItem is one Stage Bar entry (FR-CASE-004).
type StageItem struct {
	Name    string
	Current bool
	// Entered: the case has entered the stage and it is not the current one.
	Entered bool
}

// LifecycleStage is one column of the Case lifecycle popup (FR-CASE-007).
type LifecycleStage struct {
	Stage     string
	Entered   bool
	Processes []LifecycleProcess
}

type LifecycleProcess struct {
	Name    string
	Started bool
}

// Frame is what Case View shows around the main area in both modes.
type Frame struct {
	CaseID          string
	CaseType        CaseType
	Priority        int
	Status          string
	Created         Person
	Updated         Person
	Stages          []StageItem
	Lifecycle       []LifecycleStage
	AttachmentCount int
}

// Frame returns the Case View frame of a case.
func (s *Service) Frame(ctx context.Context, caseID string) (Frame, error) {
	var (
		f            Frame
		caseType     string
		status, curr *string
	)
	err := s.db.QueryRow(ctx, `
		SELECT w.case_id, w.case_type, w.urgency_work_class, w.case_status, w.current_stage,
		       w.created_by, COALESCE(oc.full_name, w.created_by), w.created_at,
		       w.updated_by, COALESCE(ou.full_name, w.updated_by), w.updated_at
		FROM fos.work w
		LEFT JOIN fos.operator oc ON oc.user_id = w.created_by
		LEFT JOIN fos.operator ou ON ou.user_id = w.updated_by
		WHERE w.case_id = $1`, caseID).Scan(
		&f.CaseID, &caseType, &f.Priority, &status, &curr,
		&f.Created.UserID, &f.Created.Name, &f.Created.At,
		&f.Updated.UserID, &f.Updated.Name, &f.Updated.At)
	if isNotFound(err) {
		return Frame{}, ErrNotFound
	}
	if err != nil {
		return Frame{}, err
	}
	f.CaseType = CaseType{Code: caseType, Label: caseType}
	if caseType == LoanFinancing.Code {
		f.CaseType = LoanFinancing
	}
	if status != nil {
		f.Status = *status
	}
	current := ""
	if curr != nil {
		current = *curr
	}

	entered := map[string]bool{}
	enteredFrom := map[string]string{}
	rows, err := s.db.Query(ctx,
		`SELECT stage_name, COALESCE(entered_from_stage, '') FROM fos.case_stage_progress WHERE case_id = $1`, caseID)
	if err != nil {
		return Frame{}, err
	}
	for rows.Next() {
		var name, from string
		if err := rows.Scan(&name, &from); err != nil {
			rows.Close()
			return Frame{}, err
		}
		entered[name] = true
		enteredFrom[name] = from
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Frame{}, err
	}
	for _, name := range stageOrder(entered, enteredFrom) {
		f.Stages = append(f.Stages, StageItem{Name: name, Current: name == current, Entered: entered[name] && name != current})
	}

	started := map[int64]bool{}
	rows, err = s.db.Query(ctx, `SELECT process_id FROM fos.case_process_progress WHERE case_id = $1`, caseID)
	if err != nil {
		return Frame{}, err
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return Frame{}, err
		}
		started[id] = true
	}
	rows.Close()

	byStage := map[string][]LifecycleProcess{}
	rows, err = s.db.Query(ctx, `SELECT process_id, stage_name, process_name FROM fos.lifecycle_process ORDER BY stage_name, sort_order`)
	if err != nil {
		return Frame{}, err
	}
	for rows.Next() {
		var (
			id          int64
			stage, name string
		)
		if err := rows.Scan(&id, &stage, &name); err != nil {
			rows.Close()
			return Frame{}, err
		}
		byStage[stage] = append(byStage[stage], LifecycleProcess{Name: name, Started: started[id]})
	}
	rows.Close()
	for _, stage := range MainStages {
		f.Lifecycle = append(f.Lifecycle, LifecycleStage{
			Stage: stage, Entered: entered[stage] && stage != current, Processes: byStage[stage],
		})
	}
	return f, rows.Err()
}

// stageOrder is the Stage Bar order (FR-CASE-004): the main stages, with
// each entered alternate stage placed right after the stage it was last
// entered from.
func stageOrder(entered map[string]bool, enteredFrom map[string]string) []string {
	order := slices.Clone(MainStages)
	for _, alt := range AlternateStages {
		if !entered[alt] {
			continue
		}
		at := slices.Index(order, enteredFrom[alt])
		if at < 0 {
			order = append(order, alt)
			continue
		}
		order = slices.Insert(order, at+1, alt)
	}
	return order
}

// AssignmentPage is an assignment opened in mode Assignment (FR-CASE-008).
type AssignmentPage struct {
	AssignmentID int64
	Name         string
	Status       string
	// PreScreening is set for a Pre-Screening Submission assignment.
	PreScreening *PreScreeningPage
}

// PreScreeningPage is the data UI-010 shows today: Nama Branch and the Tipe
// Nasabah options (FR-APP-002). The form body arrives in tickets 30 and 31.
type PreScreeningPage struct {
	BranchName string
	// BranchMissing: the case's Branch ID is empty or not in BMV BRANCH (ERR-006).
	BranchMissing       bool
	CustomerType        string
	CustomerTypeOptions []Option
}

// Option is a BMV dropdown option.
type Option struct {
	Code        string
	Description string
}

// OpenAssignment opens an assignment of a case for the session's operator.
// The worklist owner, and any Authors operator, may open it (FR-CASE-009 Go
// rules, agreed); its first opening marks it opened (FR-ASSIGN-001 rule 6).
func (s *Service) OpenAssignment(ctx context.Context, sess auth.Session, caseID string, assignmentID int64) (AssignmentPage, error) {
	var (
		p        AssignmentPage
		typ      string
		assignee *string
	)
	err := s.db.QueryRow(ctx, `
		SELECT assignment_id, assignment_name, assignment_status, assignment_type, assigned_user_id
		FROM fos.assignments WHERE assignment_id = $1 AND case_id = $2`, assignmentID, caseID).
		Scan(&p.AssignmentID, &p.Name, &p.Status, &typ, &assignee)
	if isNotFound(err) {
		return AssignmentPage{}, ErrNotFound
	}
	if err != nil {
		return AssignmentPage{}, err
	}
	owner := typ == TypeWorklist && assignee != nil && *assignee == sess.UserID
	authors := sess.AccessGroup != nil && sess.AccessGroup.Name == "Authors"
	if !owner && !authors {
		return AssignmentPage{}, ErrForbidden
	}

	if p.Status == StatusAssignmentNew {
		if err := s.db.QueryRow(ctx, `
			UPDATE fos.assignments
			SET assignment_status = $2, first_opened_at = COALESCE(first_opened_at, $3),
			    worked_by_user_id = COALESCE(worked_by_user_id, $4)
			WHERE assignment_id = $1 RETURNING assignment_status`,
			assignmentID, StatusAssignmentOpened, s.now(), sess.UserID).Scan(&p.Status); err != nil {
			return AssignmentPage{}, err
		}
	}

	if p.Name == AssignmentPreScreeningSubmission {
		page, err := s.preScreeningPage(ctx, caseID)
		if err != nil {
			return AssignmentPage{}, err
		}
		p.PreScreening = &page
	}
	return p, nil
}

func (s *Service) preScreeningPage(ctx context.Context, caseID string) (PreScreeningPage, error) {
	var (
		page         PreScreeningPage
		branch       *string
		customerType *string
	)
	// Nama Branch is read from BMV BRANCH by code, never stored (DEC-127).
	err := s.db.QueryRow(ctx, `
		SELECT w.customer_type, b.description
		FROM fos.work w
		LEFT JOIN fos.business_mapping_value b
		  ON b.type = 'BRANCH' AND b.sub_type = '' AND b.code = w.branch_code
		WHERE w.case_id = $1`, caseID).Scan(&customerType, &branch)
	if err != nil {
		return PreScreeningPage{}, err
	}
	if branch == nil {
		page.BranchMissing = true
	} else {
		page.BranchName = *branch
	}
	if customerType != nil {
		page.CustomerType = *customerType
	}
	page.CustomerTypeOptions, err = s.options(ctx, "CUSTOMER_TYPE")
	return page, err
}

// options lists a BMV type's Active rows (blank status counts as Active,
// DB-RULE-092), by sort_order then code as text (§9.6).
func (s *Service) options(ctx context.Context, bmvType string) ([]Option, error) {
	rows, err := s.db.Query(ctx, `
		SELECT code, description FROM fos.business_mapping_value
		WHERE type = $1 AND sub_type = '' AND (status = 'Active' OR COALESCE(status, '') = '')
		ORDER BY sort_order, code`, bmvType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	opts := []Option{}
	for rows.Next() {
		var o Option
		if err := rows.Scan(&o.Code, &o.Description); err != nil {
			return nil, err
		}
		opts = append(opts, o)
	}
	return opts, rows.Err()
}
