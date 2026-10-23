package cases

import (
	"context"
	"errors"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"fos/internal/auth"
)

const (
	AssignmentDocumentUnderlyingUpload = "Document Underlying Upload"

	// Customer types of the customer rows (§9.28).
	customerNasabah  = "Nasabah"
	customerPasangan = "Pasangan"

	// The Tipe Nasabah description whose form this package implements
	// (Badan Usaha: ticket 31; other types: TBD-029a).
	tipePerorangan = "Perorangan"
)

var (
	// ErrUnsupported: a request this form does not accept (unknown or
	// unsupported Tipe Nasabah, a changed Tipe Nasabah, the wrong assignment).
	ErrUnsupported = errors.New("unsupported request")
	// ErrCompleted: the assignment is already completed.
	ErrCompleted = errors.New("assignment completed")
)

// Person is a Data Pribadi or Data Pasangan block; BirthDate is yyyy-mm-dd.
type Person struct {
	NIK       string
	Name      string
	BirthDate string
}

func (p Person) empty() bool { return p.NIK == "" && p.Name == "" && p.BirthDate == "" }

// PreScreeningForm is the Pre-Screening Submission form (FR-APP-002).
type PreScreeningForm struct {
	CustomerType string
	Personal     Person
	Spouse       Person
}

// FieldError is one validation failure: the form section, the field label
// as shown in the banner, the SoT error code and the message.
type FieldError struct {
	Section string // form, personal or spouse
	Field   string
	Code    string
	Message string
}

// ValidationError lists every field that failed; the operator stays on the
// assignment (FR-APP-003 rule 2).
type ValidationError struct{ Fields []FieldError }

func (e *ValidationError) Error() string { return "validation failed" }

// Messages (§12). Invalid and out-of-range dates are TBD-023f; these are
// agreed interim texts.
const (
	msgNIK      = "Harap masukkan NIK 16 digit yang valid"
	msgName     = "Harap masukkan nama yang valid"
	msgAge      = "Umur nasabah minimal 17 tahun"
	msgNameLen  = "Nama maksimal 30 karakter"
	msgSameNIK  = "NIK tidak boleh sama"
	msgBranch   = "Branch ID operator tidak ditemukan"
	msgBadDate  = "Harap masukkan tanggal yang valid"
	msgOutRange = "Tanggal Lahir di luar rentang yang diizinkan"
)

func required(section, field string) FieldError {
	return FieldError{section, field, "ERR-010", field + " wajib diisi"}
}

var (
	nikPattern = regexp.MustCompile(`^[0-9]{16}$`)
	// Letters (Unicode), space, straight and curly apostrophe, hyphen, dot, comma.
	namePattern = regexp.MustCompile(`^[\p{L} '’\-.,]+$`)
)

// validatePerson applies the Data Pribadi rules (FR-APP-002); today is the
// operator's date. Each field reports its first failure.
func validatePerson(section string, p Person, today time.Time) []FieldError {
	var errs []FieldError
	switch {
	case p.NIK == "":
		errs = append(errs, required(section, "NIK"))
	case !nikPattern.MatchString(p.NIK):
		errs = append(errs, FieldError{section, "NIK", "ERR-007", msgNIK})
	}
	switch {
	case p.Name == "":
		errs = append(errs, required(section, "Nama"))
	case utf8.RuneCountInString(p.Name) > 30:
		errs = append(errs, FieldError{section, "Nama", "ERR-013", msgNameLen})
	case !namePattern.MatchString(p.Name):
		errs = append(errs, FieldError{section, "Nama", "ERR-008", msgName})
	}
	if p.BirthDate == "" {
		return append(errs, required(section, "Tanggal Lahir"))
	}
	born, err := time.Parse("2006-01-02", p.BirthDate)
	switch {
	case err != nil:
		errs = append(errs, FieldError{section, "Tanggal Lahir", "DATE_INVALID", msgBadDate})
	// Range up to 100 years back and today at the latest (TBD-023g, agreed).
	case born.After(today) || born.Before(today.AddDate(-100, 0, 0)):
		errs = append(errs, FieldError{section, "Tanggal Lahir", "DATE_OUT_OF_RANGE", msgOutRange})
	// At least 17 full years on today's date.
	case born.After(today.AddDate(-17, 0, 0)):
		errs = append(errs, FieldError{section, "Tanggal Lahir", "ERR-009", msgAge})
	}
	return errs
}

// openAssignment is an assignment loaded for a submit by its owner.
type openAssignment struct {
	id   int64
	name string
}

// ownOpenAssignment loads an unfinished assignment of the case that the
// session's operator owns, locking its row for the transaction.
func ownOpenAssignment(ctx context.Context, tx pgx.Tx, sess auth.Session, caseID string, id int64) (openAssignment, error) {
	var (
		a            openAssignment
		status, kind string
		assignee     *string
	)
	err := tx.QueryRow(ctx, `
		SELECT assignment_id, assignment_name, assignment_status, assignment_type, assigned_user_id
		FROM fos.assignments WHERE assignment_id = $1 AND case_id = $2 FOR UPDATE`, id, caseID).
		Scan(&a.id, &a.name, &status, &kind, &assignee)
	switch {
	case isNotFound(err):
		return a, ErrNotFound
	case err != nil:
		return a, err
	case AssignmentType(kind) != Worklist || value(assignee) != sess.UserID:
		return a, ErrForbidden
	case AssignmentStatus(status) == AssignmentCompleted:
		return a, ErrCompleted
	}
	return a, nil
}

// moveTo completes the active assignment and opens a new one for the same
// operator, as a tabbed-flow move does (FR-ASSIGN-001 rules 3-5, 17).
func (s *Service) moveTo(ctx context.Context, tx pgx.Tx, sess auth.Session, caseID string, from int64, to string) (int64, error) {
	now := s.now()
	if _, err := tx.Exec(ctx, `UPDATE fos.assignments SET assignment_status = $2, completed_at = $3 WHERE assignment_id = $1`,
		from, string(AssignmentCompleted), now); err != nil {
		return 0, err
	}
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id,
			assignment_status, created_at, first_opened_at, worked_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $6, $4) RETURNING assignment_id`,
		caseID, to, string(Worklist), sess.UserID, string(AssignmentOpened), now).Scan(&id)
	if err != nil {
		return 0, err
	}
	_, err = tx.Exec(ctx, `UPDATE fos.work SET updated_at = $2, updated_by = $3 WHERE case_id = $1`, caseID, now, sess.UserID)
	return id, err
}

// SubmitPreScreening validates the Pre-Screening Submission form and, when it
// passes, stores it and moves to Document Underlying Upload, returning the
// new assignment (FR-APP-002, FR-APP-003).
func (s *Service) SubmitPreScreening(ctx context.Context, sess auth.Session, caseID string, assignmentID int64, f PreScreeningForm) (int64, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	a, err := ownOpenAssignment(ctx, tx, sess, caseID, assignmentID)
	if err != nil {
		return 0, err
	}
	if a.name != AssignmentPreScreeningSubmission {
		return 0, ErrUnsupported
	}

	var (
		stored, branch *string
		timezone       string
	)
	if err := tx.QueryRow(ctx, `
		SELECT w.customer_type, b.description, o.timezone
		FROM fos.work w
		JOIN fos.operator o ON o.user_id = $2
		LEFT JOIN fos.business_mapping_value b ON b.type = 'BRANCH' AND b.sub_type = '' AND b.code = w.branch_code
		WHERE w.case_id = $1`, caseID, sess.UserID).Scan(&stored, &branch, &timezone); err != nil {
		return 0, err
	}
	loc, err := operatorLocation(timezone)
	if err != nil {
		return 0, err
	}
	y, m, d := s.now().In(loc).Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)

	var errs []FieldError
	if branch == nil {
		errs = append(errs, FieldError{"form", "Nama Branch", "ERR-006", msgBranch})
	}
	switch {
	case f.CustomerType == "":
		errs = append(errs, required("form", "Tipe Nasabah"))
		return 0, &ValidationError{errs}
	// Tipe Nasabah is fixed after the first submit (FR-APP-002).
	case stored != nil && *stored != f.CustomerType:
		return 0, ErrUnsupported
	}
	options, err := s.options(ctx, "CUSTOMER_TYPE")
	if err != nil {
		return 0, err
	}
	if !hasDescription(options, f.CustomerType) || f.CustomerType != tipePerorangan {
		return 0, ErrUnsupported
	}

	errs = append(errs, validatePerson("personal", f.Personal, today)...)
	if !f.Spouse.empty() {
		spouseErrs := validatePerson("spouse", f.Spouse, today)
		if len(spouseErrs) == 0 && f.Personal.NIK == f.Spouse.NIK {
			errs = append(errs, FieldError{"personal", "NIK", "ERR-019", msgSameNIK})
			spouseErrs = append(spouseErrs, FieldError{"spouse", "NIK", "ERR-019", msgSameNIK})
		}
		errs = append(errs, spouseErrs...)
	}
	if len(errs) > 0 {
		return 0, &ValidationError{errs}
	}

	if _, err := tx.Exec(ctx, `UPDATE fos.work SET customer_type = $2 WHERE case_id = $1`, caseID, f.CustomerType); err != nil {
		return 0, err
	}
	if err := saveParty(ctx, tx, caseID, customerNasabah, f.Personal); err != nil {
		return 0, err
	}
	if f.Spouse.empty() {
		if _, err := tx.Exec(ctx, `DELETE FROM fos.customer WHERE case_id = $1 AND customer_type = $2`, caseID, customerPasangan); err != nil {
			return 0, err
		}
	} else if err := saveParty(ctx, tx, caseID, customerPasangan, f.Spouse); err != nil {
		return 0, err
	}

	next, err := s.moveTo(ctx, tx, sess, caseID, a.id, AssignmentDocumentUnderlyingUpload)
	if err != nil {
		return 0, err
	}
	return next, tx.Commit(ctx)
}

// BackToPreScreening moves from Document Underlying Upload back to a new
// Pre-Screening Submission (FR-ASSIGN-001 rules 3-4). Validation before Back
// is TBD-027a; until then it always passes (agreed).
func (s *Service) BackToPreScreening(ctx context.Context, sess auth.Session, caseID string, assignmentID int64) (int64, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	a, err := ownOpenAssignment(ctx, tx, sess, caseID, assignmentID)
	if err != nil {
		return 0, err
	}
	if a.name != AssignmentDocumentUnderlyingUpload {
		return 0, ErrUnsupported
	}
	next, err := s.moveTo(ctx, tx, sess, caseID, a.id, AssignmentPreScreeningSubmission)
	if err != nil {
		return 0, err
	}
	return next, tx.Commit(ctx)
}

// saveParty writes a party's Pre-Screening data to its one row per case
// (§9.28, DB-RULE-101), creating the row the first time.
func saveParty(ctx context.Context, tx pgx.Tx, caseID, customerType string, p Person) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO fos.customer (case_id, customer_type, id_number, full_name, birth_date)
		VALUES ($1, $2, $3, $4, $5::date)
		ON CONFLICT (case_id, customer_type) WHERE customer_type <> 'Pengurus'
		DO UPDATE SET id_number = EXCLUDED.id_number, full_name = EXCLUDED.full_name, birth_date = EXCLUDED.birth_date`,
		caseID, customerType, p.NIK, p.Name, p.BirthDate)
	return err
}

func hasDescription(opts []Option, description string) bool {
	for _, o := range opts {
		if o.Description == description {
			return true
		}
	}
	return false
}

// savedParty reads a party's Pre-Screening data, or an empty Person.
func (s *Service) savedParty(ctx context.Context, caseID, customerType string) (Person, error) {
	var (
		p    Person
		born *time.Time
		nik  *string
		name *string
	)
	err := s.db.QueryRow(ctx, `SELECT id_number, full_name, birth_date FROM fos.customer
		WHERE case_id = $1 AND customer_type = $2`, caseID, customerType).Scan(&nik, &name, &born)
	if isNotFound(err) {
		return Person{}, nil
	}
	if err != nil {
		return Person{}, err
	}
	p.NIK, p.Name = value(nik), value(name)
	if born != nil {
		p.BirthDate = born.Format("2006-01-02")
	}
	return p, nil
}
