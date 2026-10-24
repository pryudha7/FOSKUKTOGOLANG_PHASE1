package cases

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"fos/internal/auth"
)

// Pipeline Data Entry vocabulary (FR-APP-004, FR-ASSIGN-001 rules 14-16).
const (
	AssignmentPipelineDataEntry = "Pipeline Data Entry"
	AssignmentBusinessEntity    = "Badan Usaha"
	AssignmentIndividualData    = "Data Pribadi"

	CaseStatusPendingSubmission = "Pending-Submission"

	// submissionTakeOver is the SUBMISSION_TYPE code of a Take Over (rule 2, DEC-116).
	submissionTakeOver = "Take Over"

	// The Bukti Surat dari Bank document: category "Dokumen Inisiasi" and the
	// Document Mapping Value of its sub_category TO (rule 6, DB-RULE-090).
	categoryInitiationLabel = "Dokumen Inisiasi"
	docBankLetter           = "Bukti Surat Take Over dari Bank"

	// The identity document of each Tipe Nasabah in Dokumen Pre-Screening (Field 5).
	docKTP       = "KTP"
	docNPWPBadan = "NPWP Badan Usaha"

	maxResultsSetting = "ui/autocomplete/max_results"
)

// categoryInitiation is the codename of Dokumen Inisiasi (DB-RULE-087).
var categoryInitiation = categoryCode(categoryInitiationLabel)

// Field keys and labels of UI-024.
const (
	KeySubmissionType = "submissionType"
	KeyProduct        = "product"
	KeyBankName       = "bankName"

	labelBranchCode     = "Kode Branch"
	labelSubmissionType = "Jenis Pengajuan"
	labelProduct        = "Produk"
	labelBankName       = "Nama Bank"
)

// invalid is ERR-037 for a value not in its source (BR-UI-007).
func invalid(section, key, label string) FieldError {
	return FieldError{section, key, label, "ERR-037", "Harap masukkan " + label + " yang valid."}
}

// startProcess records that the case started a Financing Submission
// process, when its first assignment is created (TBD-034a, agreed).
func startProcess(ctx context.Context, tx pgx.Tx, caseID, process string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO fos.case_process_progress (case_id, process_id, first_started_at)
		SELECT $1, process_id, $4 FROM fos.lifecycle_process
		WHERE stage_name = $2 AND process_name = $3 ORDER BY sort_order LIMIT 1
		ON CONFLICT (case_id, process_id) DO NOTHING`,
		caseID, StageFinancingSubmission, process, now)
	return err
}

// newBranchOfficerAssignment creates a new worklist assignment for the
// case's branch_officer, starting its process.
func newBranchOfficerAssignment(ctx context.Context, tx pgx.Tx, caseID, name string, now time.Time) (int64, error) {
	if err := startProcess(ctx, tx, caseID, name, now); err != nil {
		return 0, err
	}
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
		SELECT case_id, $2, $3, branch_officer, $4, $5 FROM fos.work WHERE case_id = $1
		RETURNING assignment_id`,
		caseID, name, string(Worklist), string(AssignmentNew), now).Scan(&id)
	return id, err
}

// openPipeline is the end of a completed run: the case enters Pipeline Data
// Entry, Pending-Submission (FR-APP-004 Assignment 1-2, AC-FR-APP-004-01).
func (e *Executor) openPipeline(ctx context.Context, tx pgx.Tx, caseID string) error {
	now := e.s.now()
	if _, err := newBranchOfficerAssignment(ctx, tx, caseID, AssignmentPipelineDataEntry, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE fos.work SET case_status = $2, updated_at = $3 WHERE case_id = $1`,
		caseID, CaseStatusPendingSubmission, now)
	return err
}

// PipelinePage is what UI-024 shows (FR-APP-004).
type PipelinePage struct {
	// BranchCode and BranchName are empty when the Branch is not Active.
	BranchCode, BranchName string
	CustomerType           string
	// The values saved on the case, for a reopened assignment.
	SubmissionType, ProductCode, BankName string
	SubmissionTypeOptions, ProductOptions []Option
	// IdentityLabel is "ID / KTP" or "NPWP Badan Usaha"; Identity its file.
	IdentityLabel string
	Identity      *DocumentItem
	BankLetter    *DocumentItem
	Editable      bool
}

// DocumentItem is a file item: Name is its document type, Caption
// "<codename> • <label>" of its category (UI-024, DB-RULE-087); FileName
// names the download.
type DocumentItem struct {
	FileID, FileName, Name, Caption string
}

// execer is the pool or a transaction, for statements.
type execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// activeBranch matches the case's branch_code to an Active BMV BRANCH row.
const activeBranch = `b.type = 'BRANCH' AND b.sub_type = '' AND b.code = w.branch_code
	AND (b.status = 'Active' OR COALESCE(b.status, '') = '')`

// clearInactiveBranch empties a branch_code that is not Active on BMV
// BRANCH (Field 1, DB-RULE-089).
func clearInactiveBranch(ctx context.Context, q execer, caseID string) error {
	_, err := q.Exec(ctx, `
		UPDATE fos.work w SET branch_code = NULL
		WHERE w.case_id = $1 AND w.branch_code IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM fos.business_mapping_value b WHERE `+activeBranch+`)`, caseID)
	return err
}

func (s *Service) pipelinePage(ctx context.Context, caseID string, editable bool) (PipelinePage, error) {
	page := PipelinePage{Editable: editable}
	if editable {
		if err := clearInactiveBranch(ctx, s.db, caseID); err != nil {
			return PipelinePage{}, err
		}
	}
	var branchCode, branchName, customerType, submission, product, bank *string
	if err := s.db.QueryRow(ctx, `
		SELECT b.code, b.description, w.customer_type, w.submission_type, w.product_code, w.bank_name
		FROM fos.work w LEFT JOIN fos.business_mapping_value b ON `+activeBranch+`
		WHERE w.case_id = $1`, caseID).Scan(&branchCode, &branchName, &customerType, &submission, &product, &bank); err != nil {
		return PipelinePage{}, err
	}
	page.BranchCode, page.BranchName, page.CustomerType = value(branchCode), value(branchName), value(customerType)
	page.SubmissionType, page.ProductCode, page.BankName = value(submission), value(product), value(bank)
	var err error
	if page.SubmissionTypeOptions, err = s.options(ctx, "SUBMISSION_TYPE"); err != nil {
		return PipelinePage{}, err
	}
	if page.ProductOptions, err = s.products(ctx); err != nil {
		return PipelinePage{}, err
	}
	identity := docKTP
	page.IdentityLabel = "ID / KTP"
	if page.CustomerType == customerTypeBusinessEntity {
		identity, page.IdentityLabel = docNPWPBadan, docNPWPBadan
	}
	if page.Identity, err = s.documentItem(ctx, caseID, categoryPreScreening, identity); err != nil {
		return PipelinePage{}, err
	}
	page.BankLetter, err = s.documentItem(ctx, caseID, categoryInitiation, docBankLetter)
	return page, err
}

// products are the Produk options: every Product Management row by code,
// shown by name (DB-RULE-088).
func (s *Service) products(ctx context.Context) ([]Option, error) {
	opts, err := collect(ctx, s, func(r pgx.Rows) (Option, error) {
		var o Option
		return o, r.Scan(&o.Code, &o.Description)
	}, `SELECT code, name FROM fos.product_management ORDER BY code`)
	if opts == nil {
		opts = []Option{}
	}
	return opts, err
}

// documentItem is the case's file of a category and type, if any. The
// category label is its Dokumen Pre-Screening BMV description, or Dokumen
// Inisiasi's.
func (s *Service) documentItem(ctx context.Context, caseID, category, docType string) (*DocumentItem, error) {
	var d DocumentItem
	var label string
	err := s.db.QueryRow(ctx, `
		SELECT d.file_id::text, d.file_name, COALESCE(b.description, CASE WHEN d.document_category = $4 THEN $5 END, d.document_category)
		FROM fos.case_document d
		JOIN fos.work w ON w.case_id = d.case_id
		LEFT JOIN fos.case_document_category c ON c.case_id = d.case_id AND c.document_category = d.document_category
		LEFT JOIN fos.business_mapping_value b
		  ON b.type = 'DOCUMENTPRESCREENING' AND b.sub_type = w.customer_type AND b.code = c.source_code
		WHERE d.case_id = $1 AND d.document_category = $2 AND d.document_type = $3`,
		caseID, category, docType, categoryInitiation, categoryInitiationLabel).Scan(&d.FileID, &d.FileName, &label)
	if isNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.Name, d.Caption = docType, category+" • "+label
	return &d, nil
}

// ownPipeline loads the session operator's open Pipeline Data Entry,
// locking it for the transaction.
func ownPipeline(ctx context.Context, tx pgx.Tx, sess auth.Session, caseID string, id int64) error {
	a, err := ownOpenAssignment(ctx, tx, sess, caseID, id)
	if err != nil {
		return err
	}
	if a.name != AssignmentPipelineDataEntry {
		return ErrUnsupported
	}
	return nil
}

// PipelineForm is UI-024's Submit: codes of Jenis Pengajuan and Produk, and
// the Nama Bank text.
type PipelineForm struct {
	SubmissionType, ProductCode, BankName string
}

// SubmitPipeline validates and stores Pipeline Data Entry, completes it and
// gives the Branch Officer Badan Usaha or Data Pribadi (FR-APP-004 Process
// 5, FR-ASSIGN-001 rules 15-16), returning the new assignment. case_status
// stays Pending-Submission (TBD-052a, agreed).
func (s *Service) SubmitPipeline(ctx context.Context, sess auth.Session, caseID string, id int64, f PipelineForm) (int64, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if err := ownPipeline(ctx, tx, sess, caseID, id); err != nil {
		return 0, err
	}
	if err := clearInactiveBranch(ctx, tx, caseID); err != nil {
		return 0, err
	}
	var branch, customerType *string
	if err := tx.QueryRow(ctx, `SELECT NULLIF(branch_code, ''), customer_type FROM fos.work WHERE case_id = $1`, caseID).
		Scan(&branch, &customerType); err != nil {
		return 0, err
	}
	next := map[string]string{customerTypeIndividual: AssignmentIndividualData, customerTypeBusinessEntity: AssignmentBusinessEntity}[value(customerType)]
	if next == "" {
		return 0, ErrUnsupported
	}

	f.SubmissionType, f.ProductCode, f.BankName = strings.TrimSpace(f.SubmissionType), strings.TrimSpace(f.ProductCode), strings.TrimSpace(f.BankName)
	var errs []FieldError
	if branch == nil {
		errs = append(errs, required(SectionForm, KeyBranch, labelBranchCode))
	}
	switch known, err := activeCode(ctx, tx, "SUBMISSION_TYPE", f.SubmissionType); {
	case err != nil:
		return 0, err
	case f.SubmissionType == "":
		errs = append(errs, required(SectionForm, KeySubmissionType, labelSubmissionType))
	case !known:
		errs = append(errs, invalid(SectionForm, KeySubmissionType, labelSubmissionType))
	}
	takeOver := f.SubmissionType == submissionTakeOver
	if takeOver && f.BankName == "" {
		errs = append(errs, required(SectionForm, KeyBankName, labelBankName))
	}
	var contract *string
	err = tx.QueryRow(ctx, `SELECT contract_type FROM fos.product_management WHERE code = $1`, f.ProductCode).Scan(&contract)
	switch {
	case err != nil && !isNotFound(err):
		return 0, err
	case f.ProductCode == "":
		errs = append(errs, required(SectionForm, KeyProduct, labelProduct))
	case contract == nil:
		errs = append(errs, invalid(SectionForm, KeyProduct, labelProduct))
	}
	if len(errs) > 0 {
		// A cleared Branch stays cleared (DB-RULE-089).
		if err := tx.Commit(ctx); err != nil {
			return 0, err
		}
		return 0, &ValidationError{errs}
	}

	now := s.now()
	var bank *string
	if takeOver {
		bank = &f.BankName
	}
	if _, err := tx.Exec(ctx, `
		UPDATE fos.work SET submission_type = $2, product_code = $3, contract_type = $4, bank_name = $5, updated_at = $6, updated_by = $7
		WHERE case_id = $1`, caseID, f.SubmissionType, f.ProductCode, *contract, bank, now, sess.UserID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE fos.assignments SET assignment_status = $2, completed_at = $3 WHERE assignment_id = $1`,
		id, string(AssignmentCompleted), now); err != nil {
		return 0, err
	}
	nextID, err := newBranchOfficerAssignment(ctx, tx, caseID, next, now)
	if err != nil {
		return 0, err
	}
	return nextID, tx.Commit(ctx)
}

// activeCode reports whether code is an Active row of a BMV type.
func activeCode(ctx context.Context, q querier, bmvType, code string) (bool, error) {
	opts, err := optionsIn(ctx, q, bmvType)
	for _, o := range opts {
		if o.Code == code {
			return true, err
		}
	}
	return false, err
}

// BankSuggestions are the Nama Bank suggestions for text (FR-APP-004 Field
// 6, BR-UI-005): Active BMV BANK rows whose description contains it,
// ignoring case, by sort_order then code, at most
// ui/autocomplete/max_results, queried from the database each time.
func (s *Service) BankSuggestions(ctx context.Context, sess auth.Session, caseID string, id int64, text string) ([]Option, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := ownPipeline(ctx, tx, sess, caseID, id); err != nil {
		return nil, err
	}
	limit, err := s.settings.Int(ctx, maxResultsSetting)
	if err != nil {
		return nil, err
	}
	opts, err := collectIn(ctx, tx, func(r pgx.Rows) (Option, error) {
		var o Option
		return o, r.Scan(&o.Code, &o.Description)
	}, `
		SELECT code, description FROM fos.business_mapping_value
		WHERE type = 'BANK' AND sub_type = '' AND (status = 'Active' OR COALESCE(status, '') = '')
		  AND strpos(lower(description), lower($1)) > 0
		ORDER BY sort_order, code LIMIT $2`, text, limit)
	if opts == nil {
		opts = []Option{}
	}
	return opts, err
}

// ChangeSubmissionType is a change of Jenis Pengajuan (FR-APP-004 rule 3,
// DB-RULE-090): Nama Bank is cleared and the Bukti Surat row deleted in one
// transaction, then its object. The new choice is saved by Submit.
func (s *Service) ChangeSubmissionType(ctx context.Context, sess auth.Session, caseID string, id int64) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := ownPipeline(ctx, tx, sess, caseID, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE fos.work SET bank_name = NULL WHERE case_id = $1`, caseID); err != nil {
		return err
	}
	var letter storedObject
	err = tx.QueryRow(ctx, `
		DELETE FROM fos.case_document WHERE case_id = $1 AND document_category = $2 AND document_type = $3
		RETURNING bucket_name, object_name`, caseID, categoryInitiation, docBankLetter).Scan(&letter.bucket, &letter.object)
	found := err == nil
	if err != nil && !isNotFound(err) {
		return err
	}
	if err := tx.Commit(ctx); err != nil || !found {
		return err
	}
	// A failure here leaves the object behind; the case is consistent (DB-RULE-031).
	if err := s.storage.Delete(ctx, letter.bucket, letter.object); err != nil {
		slog.Error("deleting the Bukti Surat object failed", "case_id", caseID, "object", letter.object, "error", err)
	}
	return nil
}

// UploadBankLetter stores Bukti Surat dari Bank when it is chosen, one per
// case, named "Bukti Surat Take Over dari Bank" plus the original extension
// (FR-APP-004 rules 6-8), with the Upload Document rules for type and size.
func (s *Service) UploadBankLetter(ctx context.Context, sess auth.Session, caseID string, id int64, u UploadRequest) (StoredFile, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return StoredFile{}, err
	}
	err = ownPipeline(ctx, tx, sess, caseID, id)
	tx.Rollback(ctx)
	if err != nil {
		return StoredFile{}, err
	}
	u.Category, u.DocumentType, u.Name = categoryInitiation, docBankLetter, docBankLetter
	return s.storeUpload(ctx, sess, caseID, id, u)
}
