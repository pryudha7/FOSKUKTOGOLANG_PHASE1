package cases

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"fos/internal/auth"
)

// Actor is the operator who created or last updated a case, shown by full
// name read from the operator data at display time (DB-RULE-027).
type Actor struct {
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
	CaseStatus      string
	Created         Actor
	Updated         Actor
	Stages          []StageItem
	Lifecycle       []LifecycleStage
	AttachmentCount int
	Attachments     []Attachment
}

// Attachment is one file of the attachments panel (FR-CASE-006).
type Attachment struct {
	FileID   string
	FileName string
	// CategoryLabel is the category's label, or its codename when it has none
	// (TBD-033).
	CategoryLabel string
	// UploadedBy is the uploader's full name from the operator data (DEC-032).
	UploadedBy string
	CreatedAt  time.Time
}

// Frame returns the Case View frame of a case.
func (s *Service) Frame(ctx context.Context, caseID string) (Frame, error) {
	var (
		f                  Frame
		caseType           string
		status, currentPtr *string
	)
	err := s.db.QueryRow(ctx, `
		SELECT w.case_id, w.case_type, w.urgency_work_class, w.case_status, w.current_stage,
		       w.created_by, COALESCE(oc.full_name, w.created_by), w.created_at,
		       w.updated_by, COALESCE(ou.full_name, w.updated_by), w.updated_at
		FROM fos.work w
		LEFT JOIN fos.operator oc ON oc.user_id = w.created_by
		LEFT JOIN fos.operator ou ON ou.user_id = w.updated_by
		WHERE w.case_id = $1`, caseID).Scan(
		&f.CaseID, &caseType, &f.Priority, &status, &currentPtr,
		&f.Created.UserID, &f.Created.Name, &f.Created.At,
		&f.Updated.UserID, &f.Updated.Name, &f.Updated.At)
	if isNotFound(err) {
		return Frame{}, ErrNotFound
	}
	if err != nil {
		return Frame{}, err
	}
	f.CaseType = caseTypeOf(caseType)
	f.CaseStatus = value(status)
	current := value(currentPtr)

	type progress struct{ stage, from string }
	stages, err := collect(ctx, s, func(r pgx.Rows) (progress, error) {
		var p progress
		return p, r.Scan(&p.stage, &p.from)
	}, `SELECT stage_name, COALESCE(entered_from_stage, '') FROM fos.case_stage_progress WHERE case_id = $1`, caseID)
	if err != nil {
		return Frame{}, err
	}
	entered := map[string]bool{}
	enteredFrom := map[string]string{}
	for _, p := range stages {
		entered[p.stage], enteredFrom[p.stage] = true, p.from
	}
	for _, name := range stageOrder(entered, enteredFrom) {
		f.Stages = append(f.Stages, StageItem{Name: name, Current: name == current, Entered: entered[name] && name != current})
	}

	type process struct {
		stage, name string
		started     bool
	}
	processes, err := collect(ctx, s, func(r pgx.Rows) (process, error) {
		var p process
		return p, r.Scan(&p.stage, &p.name, &p.started)
	}, `
		SELECT lp.stage_name, lp.process_name, cpp.process_id IS NOT NULL
		FROM fos.lifecycle_process lp
		LEFT JOIN fos.case_process_progress cpp ON cpp.process_id = lp.process_id AND cpp.case_id = $1
		ORDER BY lp.stage_name, lp.sort_order`, caseID)
	if err != nil {
		return Frame{}, err
	}
	byStage := map[string][]LifecycleProcess{}
	for _, p := range processes {
		byStage[p.stage] = append(byStage[p.stage], LifecycleProcess{Name: p.name, Started: p.started})
	}
	for _, stage := range MainStages {
		f.Lifecycle = append(f.Lifecycle, LifecycleStage{
			Stage: stage, Entered: entered[stage] && stage != current, Processes: byStage[stage],
		})
	}
	f.Attachments, err = collect(ctx, s, func(r pgx.Rows) (Attachment, error) {
		var a Attachment
		return a, r.Scan(&a.FileID, &a.FileName, &a.CategoryLabel, &a.UploadedBy, &a.CreatedAt)
	}, `
		SELECT d.file_id::text, d.file_name, COALESCE(b.description, d.document_category, ''),
		       COALESCE(o.full_name, d.created_by), d.created_at
		FROM fos.case_document d
		JOIN fos.work w ON w.case_id = d.case_id
		LEFT JOIN fos.case_document_category c ON c.case_id = d.case_id AND c.document_category = d.document_category
		LEFT JOIN fos.business_mapping_value b
		  ON b.type = 'DOCUMENTPRESCREENING' AND b.sub_type = w.customer_type AND b.code = c.source_code
		LEFT JOIN fos.operator o ON o.user_id = d.created_by
		WHERE d.case_id = $1
		ORDER BY d.created_at DESC, d.file_name`, caseID)
	if err != nil {
		return Frame{}, err
	}
	if f.Attachments == nil {
		f.Attachments = []Attachment{}
	}
	f.AttachmentCount = len(f.Attachments)
	return f, nil
}

// querier is the pool or a transaction.
type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

// execer is the pool or a transaction, for statements.
type execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// collect runs a query and scans every row.
func collect[T any](ctx context.Context, s *Service, scan func(pgx.Rows) (T, error), sql string, args ...any) ([]T, error) {
	return collectIn(ctx, s.db, scan, sql, args...)
}

// collectIn is collect on q, e.g. inside a transaction.
func collectIn[T any](ctx context.Context, q querier, scan func(pgx.Rows) (T, error), sql string, args ...any) ([]T, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

const dateISO = "2006-01-02"

func value(p *string) string {
	if p == nil {
		return ""
	}
	return *p
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
	Status       AssignmentStatus
	// PreScreening is set for a Pre-Screening Submission assignment.
	PreScreening *PreScreeningPage
	// Underlying is set for a Document Underlying Upload assignment.
	Underlying *UnderlyingPage
	// Review is set for the Team Lead's Pre-Screening Review assignment.
	Review *ReviewPage
	// WaitingSLIK is set for the Branch Officer's waiting assignment (UI-023).
	WaitingSLIK *WaitingSLIKPage
	// Pipeline is set for Pipeline Data Entry (UI-024).
	Pipeline *PipelinePage
	// IndividualData, Spouse and CloseRelative are set for Data Pribadi, Data Pasangan
	// and Data Keluarga Dekat (UI-026 to UI-028).
	IndividualData *IndividualDataPage
	Spouse         *SpousePage
	CloseRelative  *CloseRelativePage
	// BusinessEntity is set for Badan Usaha (UI-025).
	BusinessEntity *BusinessEntityPage
	// BusinessData and OtherFinancing are set for Data Usaha Nasabah and
	// Data Pembiayaan Lain (UI-030, UI-031).
	BusinessData   *BusinessDataPage
	OtherFinancing *OtherFinancingPage
	// FacilityRequest is set for Data Permohonan Fasilitas (UI-032).
	FacilityRequest *FacilityRequestPage
}

// PreScreeningPage is the data UI-010 shows today: Nama Branch and the Tipe
// Nasabah options (FR-APP-002). The form body arrives in tickets 30 and 31.
type PreScreeningPage struct {
	BranchName string
	// BranchMissing: the case's Branch ID is empty or not in BMV BRANCH (ERR-006).
	BranchMissing       bool
	CustomerType        string
	CustomerTypeOptions []Option
	// Customer and Spouse are the saved Data Pribadi and Data Pasangan.
	Customer, Spouse Person
	// Editable: the operator may submit (the worklist owner on an open
	// assignment); an Authors look is read-only (agreed).
	Editable bool
	// Today and EarliestBirthDate bound the birth-date pickers, as the
	// operator's dates (yyyy-mm-dd).
	Today, EarliestBirthDate string
	// NotesBanner is the latest Pre-Screening Review decision, once there is one.
	NotesBanner *Recommendation
}

// Option is a BMV dropdown option.
type Option struct {
	Code        string
	Description string
}

// OpenAssignment opens an assignment of a case for the session's operator.
// The worklist owner, and any Authors operator, may open it (FR-CASE-009 Go
// rules, agreed). Only the owner's first opening marks it opened and records
// the worker (FR-ASSIGN-001 rules 6 and 18); an Authors look changes nothing.
func (s *Service) OpenAssignment(ctx context.Context, sess auth.Session, caseID string, assignmentID int64) (AssignmentPage, error) {
	var (
		p            AssignmentPage
		status, kind string
		assignee     *string
	)
	err := s.db.QueryRow(ctx, `
		SELECT assignment_id, assignment_name, assignment_status, assignment_type, assigned_user_id
		FROM fos.assignments WHERE assignment_id = $1 AND case_id = $2`, assignmentID, caseID).
		Scan(&p.AssignmentID, &p.Name, &status, &kind, &assignee)
	if isNotFound(err) {
		return AssignmentPage{}, ErrNotFound
	}
	if err != nil {
		return AssignmentPage{}, err
	}
	p.Status = AssignmentStatus(status)
	owner, may := mayOpen(sess, AssignmentType(kind), value(assignee))
	if !may {
		return AssignmentPage{}, ErrForbidden
	}

	if owner && p.Status == AssignmentNew {
		if err := s.db.QueryRow(ctx, `
			UPDATE fos.assignments
			SET assignment_status = $2, first_opened_at = COALESCE(first_opened_at, $3),
			    worked_by_user_id = COALESCE(worked_by_user_id, $4)
			WHERE assignment_id = $1 RETURNING assignment_status`,
			assignmentID, string(AssignmentOpened), s.now(), sess.UserID).Scan(&status); err != nil {
			return AssignmentPage{}, err
		}
		p.Status = AssignmentStatus(status)
	}

	editable := owner && p.Status != AssignmentCompleted
	if p.Name == AssignmentPreScreeningSubmission {
		page, err := s.preScreeningPage(ctx, caseID)
		if err != nil {
			return AssignmentPage{}, err
		}
		page.Editable = editable
		today, err := s.operatorToday(ctx, s.db, sess.UserID)
		if err != nil {
			return AssignmentPage{}, err
		}
		page.Today, page.EarliestBirthDate = today.Format(dateISO), EarliestBirthDate(today).Format(dateISO)
		p.PreScreening = &page
	}
	if p.Name == AssignmentUnderlyingDocumentUpload {
		page, err := s.underlyingPage(ctx, sess.UserID, caseID, editable)
		if err != nil {
			return AssignmentPage{}, err
		}
		p.Underlying = &page
	}
	if p.Name == AssignmentWaitingSLIK {
		page, err := s.waitingSLIKPage(ctx, caseID, editable)
		if err != nil {
			return AssignmentPage{}, err
		}
		p.WaitingSLIK = &page
	}
	if p.Name == AssignmentPipelineDataEntry {
		page, err := s.pipelinePage(ctx, caseID, editable)
		if err != nil {
			return AssignmentPage{}, err
		}
		p.Pipeline = &page
	}
	switch p.Name {
	case AssignmentIndividualData:
		page, err := s.individualDataPage(ctx, caseID, editable)
		if err != nil {
			return AssignmentPage{}, err
		}
		p.IndividualData = &page
	case AssignmentBusinessData:
		page, err := s.businessDataPage(ctx, caseID, editable)
		if err != nil {
			return AssignmentPage{}, err
		}
		p.BusinessData = &page
	case AssignmentOtherFinancingData:
		page, err := s.otherFinancingPage(ctx, caseID, editable)
		if err != nil {
			return AssignmentPage{}, err
		}
		p.OtherFinancing = &page
	case AssignmentFacilityRequest:
		page, err := s.facilityRequestPage(ctx, caseID, editable)
		if err != nil {
			return AssignmentPage{}, err
		}
		p.FacilityRequest = &page
	case AssignmentBusinessEntity:
		page, err := s.businessEntityPage(ctx, sess, caseID, editable)
		if err != nil {
			return AssignmentPage{}, err
		}
		p.BusinessEntity = &page
	case AssignmentSpouseData:
		page, err := s.spousePage(ctx, sess, caseID, editable)
		if err != nil {
			return AssignmentPage{}, err
		}
		p.Spouse = &page
	case AssignmentCloseRelativeData:
		page, err := s.closeRelativePage(ctx, caseID, editable)
		if err != nil {
			return AssignmentPage{}, err
		}
		p.CloseRelative = &page
	}
	if p.Name == AssignmentPreScreeningReview {
		page, err := s.reviewPage(ctx, sess, caseID)
		if err != nil {
			return AssignmentPage{}, err
		}
		page.Editable = editable
		p.Review = &page
	}
	return p, nil
}

func (s *Service) preScreeningPage(ctx context.Context, caseID string) (PreScreeningPage, error) {
	var (
		page                 PreScreeningPage
		branch, customerType *string
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
	page.BranchMissing = branch == nil
	page.BranchName = value(branch)
	page.CustomerType = value(customerType)
	if page.CustomerTypeOptions, err = s.options(ctx, "CUSTOMER_TYPE"); err != nil {
		return PreScreeningPage{}, err
	}
	if page.Customer, err = s.savedParty(ctx, caseID, partyCustomer); err != nil {
		return PreScreeningPage{}, err
	}
	if page.Spouse, err = s.savedParty(ctx, caseID, partySpouse); err != nil {
		return PreScreeningPage{}, err
	}
	page.NotesBanner, err = s.latestRecommendation(ctx, caseID)
	return page, err
}

// options lists a BMV type's Active rows (blank status counts as Active,
// DB-RULE-092), by sort_order then code as text (§9.6).
func (s *Service) options(ctx context.Context, bmvType string) ([]Option, error) {
	return optionsIn(ctx, s.db, bmvType)
}

// optionsIn is options on q, e.g. inside a transaction.
func optionsIn(ctx context.Context, q querier, bmvType string) ([]Option, error) {
	return optionList(collectIn(ctx, q, scanOption, `
		SELECT code, description FROM fos.business_mapping_value
		WHERE type = $1 AND sub_type = '' AND `+activeStatus+`
		ORDER BY sort_order, code`, bmvType))
}

// activeStatus keeps Active BMV rows; a blank status counts as Active
// (DB-RULE-092). activeStatusOfB is the same on the alias b.
const (
	activeStatus    = `(status = 'Active' OR COALESCE(status, '') = '')`
	activeStatusOfB = `(b.status = 'Active' OR COALESCE(b.status, '') = '')`
)

// scanOption reads a (code, text) row as an Option.
func scanOption(r pgx.Rows) (Option, error) {
	var o Option
	return o, r.Scan(&o.Code, &o.Description)
}

// optionList is a query's options, never nil, so an empty list encodes as [].
func optionList(opts []Option, err error) ([]Option, error) {
	if opts == nil {
		opts = []Option{}
	}
	return opts, err
}
