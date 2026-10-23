package app_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"fos/internal/apptest"
)

// person is a Data Pribadi / Data Pasangan block as the form sends it.
type person struct{ NIK, Name, BirthDate string }

func (p person) body() map[string]string {
	return map[string]string{"nik": p.NIK, "name": p.Name, "birthDate": p.BirthDate}
}

func preScreening(customerType string, personal, spouse person) map[string]any {
	return map[string]any{"customerType": customerType, "customer": personal.body(), "spouse": spouse.body()}
}

func submitPath(caseID string, assignmentID any, action string) string {
	return fmt.Sprintf("%s/api/cases/%s/assignments/%v/%s", apptest.EndpointPathDesktop, caseID, assignmentID, action)
}

// yearsAgo is the date n years before the harness clock's today at GMT+7,
// as yyyy-mm-dd, shifted by days.
func yearsAgo(h *apptest.Harness, n, days int) string {
	today := h.Clock.Now().In(time.FixedZone("GMT+7", 7*3600))
	return today.AddDate(-n, 0, days).Format("2006-01-02")
}

func validPersonal(h *apptest.Harness) person {
	return person{"3171011234567890", "Budi Santoso", yearsAgo(h, 30, 0)}
}

// newCase creates a case as AO001 and returns the client, case and its
// Pre-Screening Submission assignment.
func newCase(t *testing.T, h *apptest.Harness) (*apptest.Client, string, any) {
	t.Helper()
	var c *apptest.Client
	if count(t, h, `SELECT count(*) FROM fos.operator WHERE user_id = 'AO001'`) == 0 {
		c = branchOfficer(t, h, "AO001", "AO")
	} else {
		c = h.NewClient()
		if r := c.LoginDatabase(t, "AO001", "Rahasia#1"); r.Status != http.StatusOK {
			t.Fatalf("login = %d", r.Status)
		}
	}
	caseID, assignmentID := created(t, c.CreateCase(t, "Loan Financing"))
	return c, caseID, assignmentID
}

type customerRow struct {
	Type, NIK, Name string
	BirthDate       time.Time
}

func customers(t *testing.T, h *apptest.Harness, caseID string) map[string]customerRow {
	t.Helper()
	rows, err := h.DB.Query(context.Background(),
		`SELECT customer_type, id_number, full_name, birth_date FROM fos.customer WHERE case_id = $1`, caseID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]customerRow{}
	for rows.Next() {
		var r customerRow
		if err := rows.Scan(&r.Type, &r.NIK, &r.Name, &r.BirthDate); err != nil {
			t.Fatal(err)
		}
		if _, dup := out[r.Type]; dup {
			t.Fatalf("two %s rows", r.Type)
		}
		out[r.Type] = r
	}
	return out
}

type assignmentRow struct {
	ID                   int64
	Name, Status, Holder string
	Completed            *time.Time
}

func assignments(t *testing.T, h *apptest.Harness, caseID string) []assignmentRow {
	t.Helper()
	rows, err := h.DB.Query(context.Background(), `SELECT assignment_id, assignment_name, assignment_status,
		COALESCE(assigned_user_id, ''), completed_at FROM fos.assignments WHERE case_id = $1 ORDER BY assignment_id`, caseID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []assignmentRow
	for rows.Next() {
		var r assignmentRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Status, &r.Holder, &r.Completed); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

// AC-FR-APP-003-02, AC-FR-ASSIGN-001-05, AC-FR-APP-002-24: a valid submit
// stores the data, completes Pre-Screening Submission and opens Document
// Underlying Upload for the same operator.
func TestValidPreScreeningSubmissionMovesToDocumentUnderlyingUpload(t *testing.T) {
	h := caseHarness(t)
	c, caseID, assignmentID := newCase(t, h)
	spouse := person{"3171019876543210", "Siti Aminah", yearsAgo(h, 28, 0)}

	r := c.Do(t, http.MethodPost, submitPath(caseID, assignmentID, "submit"), preScreening("Perorangan", validPersonal(h), spouse))
	if r.Status != http.StatusOK {
		t.Fatalf("submit = %d %s", r.Status, r.Raw)
	}
	as := assignments(t, h, caseID)
	if len(as) != 2 || as[0].Status != "sudah selesai dikerjakan" || as[0].Completed == nil ||
		as[1].Name != "Document Underlying Upload" || as[1].Status != "sudah dibuka" || as[1].Holder != "AO001" ||
		r.Body["assignmentId"] != float64(as[1].ID) {
		t.Fatalf("assignments = %+v, response %s", as, r.Raw)
	}
	if got := deref(workCustomerType(t, h, caseID)); got != "Perorangan" {
		t.Fatalf("work.customer_type = %s, want the description Perorangan", got)
	}
	cs := customers(t, h, caseID)
	if n := cs["Nasabah"]; n.NIK != "3171011234567890" || n.Name != "Budi Santoso" || n.BirthDate.Format("2006-01-02") != yearsAgo(h, 30, 0) {
		t.Fatalf("Nasabah = %+v", n)
	}
	if p := cs["Pasangan"]; p.NIK != spouse.NIK || p.Name != spouse.Name {
		t.Fatalf("Pasangan = %+v", p)
	}
	if len(cs) != 2 {
		t.Fatalf("customers = %v", cs)
	}
}

func workCustomerType(t *testing.T, h *apptest.Harness, caseID string) *string {
	t.Helper()
	var v *string
	if err := h.DB.QueryRow(context.Background(), `SELECT customer_type FROM fos.work WHERE case_id = $1`, caseID).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

type fieldError struct{ Section, Field, Code, Message string }

func fieldErrors(t *testing.T, r apptest.Response) []fieldError {
	t.Helper()
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d %s, want 422", r.Status, r.Raw)
	}
	var out []fieldError
	for _, e := range r.Body["errors"].([]any) {
		m := e.(map[string]any)
		out = append(out, fieldError{m["section"].(string), m["field"].(string), m["code"].(string), m["message"].(string)})
	}
	return out
}

// AC-FR-APP-002-06/07/08/10/11/12/21, AC-FR-APP-003-03: validation on Next.
// A refused submit changes nothing.
func TestPreScreeningSubmissionValidation(t *testing.T) {
	h := caseHarness(t)
	ok := validPersonal(h)
	none := person{}
	const (
		required = "ERR-010"
		badNIK   = "Harap masukkan NIK 16 digit yang valid"
		badName  = "Harap masukkan nama yang valid"
		tooLong  = "Nama maksimal 30 karakter"
		tooYoung = "Umur nasabah minimal 17 tahun"
		badDate  = "Harap masukkan tanggal yang valid"
		outRange = "Tanggal Lahir di luar rentang yang diizinkan"
		sameNIK  = "NIK tidak boleh sama"
	)
	cases := []struct {
		name             string
		customerType     string
		personal, spouse person
		want             []fieldError
	}{
		// Without Tipe Nasabah no data section is shown, so only it is reported.
		{"no Tipe Nasabah", "", none, none, []fieldError{{"form", "Tipe Nasabah", required, "Tipe Nasabah wajib diisi"}}},
		{"Data Pribadi empty", "Perorangan", none, none, []fieldError{
			{"customer", "NIK", required, "NIK wajib diisi"},
			{"customer", "Nama", required, "Nama wajib diisi"},
			{"customer", "Tanggal Lahir", required, "Tanggal Lahir wajib diisi"},
		}},
		{"NIK of 15 digits", "Perorangan", person{"317101123456789", ok.Name, ok.BirthDate}, none,
			[]fieldError{{"customer", "NIK", "ERR-007", badNIK}}},
		{"NIK with a letter", "Perorangan", person{"317101123456789A", ok.Name, ok.BirthDate}, none,
			[]fieldError{{"customer", "NIK", "ERR-007", badNIK}}},
		{"name with digits", "Perorangan", person{ok.NIK, "John123", ok.BirthDate}, none,
			[]fieldError{{"customer", "Nama", "ERR-008", badName}}},
		{"name of 31 characters", "Perorangan", person{ok.NIK, strings.Repeat("a", 31), ok.BirthDate}, none,
			[]fieldError{{"customer", "Nama", "ERR-013", tooLong}}},
		{"aged 16", "Perorangan", person{ok.NIK, ok.Name, yearsAgo(h, 17, 1)}, none,
			[]fieldError{{"customer", "Tanggal Lahir", "ERR-009", tooYoung}}},
		{"not a date", "Perorangan", person{ok.NIK, ok.Name, "1999-02-30"}, none,
			[]fieldError{{"customer", "Tanggal Lahir", "DATE_INVALID", badDate}}},
		{"born tomorrow", "Perorangan", person{ok.NIK, ok.Name, yearsAgo(h, 0, 1)}, none,
			[]fieldError{{"customer", "Tanggal Lahir", "DATE_OUT_OF_RANGE", outRange}}},
		{"over 100 years ago", "Perorangan", person{ok.NIK, ok.Name, yearsAgo(h, 100, -1)}, none,
			[]fieldError{{"customer", "Tanggal Lahir", "DATE_OUT_OF_RANGE", outRange}}},
		{"spouse name only", "Perorangan", ok, person{"", "Siti Aminah", ""}, []fieldError{
			{"spouse", "NIK", required, "NIK wajib diisi"},
			{"spouse", "Tanggal Lahir", required, "Tanggal Lahir wajib diisi"},
		}},
		{"spouse NIK equals own NIK", "Perorangan", ok, person{ok.NIK, "Siti Aminah", yearsAgo(h, 28, 0)}, []fieldError{
			{"customer", "NIK", "ERR-019", sameNIK},
			{"spouse", "NIK", "ERR-019", sameNIK},
		}},
		// ERR-019 needs only both NIKs valid, whatever else fails on the spouse.
		{"spouse NIK equal and spouse name invalid", "Perorangan", ok, person{ok.NIK, "Siti123", yearsAgo(h, 28, 0)}, []fieldError{
			{"customer", "NIK", "ERR-019", sameNIK},
			{"spouse", "NIK", "ERR-019", sameNIK},
			{"spouse", "Nama", "ERR-008", badName},
		}},
		// Spaces alone are empty: required for Data Pribadi, absent for Data Pasangan.
		{"name of spaces", "Perorangan", person{ok.NIK, "   ", ok.BirthDate}, person{" ", "  ", ""},
			[]fieldError{{"customer", "Nama", required, "Nama wajib diisi"}}},
		{"spouse aged 16", "Perorangan", ok, person{"3171019876543210", "Siti Aminah", yearsAgo(h, 16, 0)},
			[]fieldError{{"spouse", "Tanggal Lahir", "ERR-009", tooYoung}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, caseID, assignmentID := newCase(t, h)
			got := fieldErrors(t, c.Do(t, http.MethodPost, submitPath(caseID, assignmentID, "submit"),
				preScreening(tc.customerType, tc.personal, tc.spouse)))
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("errors\n got %v\nwant %v", got, tc.want)
			}
			if as := assignments(t, h, caseID); len(as) != 1 || as[0].Status != "sudah dibuka" {
				t.Fatalf("a refused submit moved the case: %+v", as)
			}
			if cs := customers(t, h, caseID); len(cs) != 0 || workCustomerType(t, h, caseID) != nil {
				t.Fatalf("a refused submit stored data: %v", cs)
			}
		})
	}
}

// Valid edge values are accepted: the name example of AC-FR-APP-002-07,
// exactly 30 characters, exactly 17 years old, born exactly 100 years ago.
func TestPreScreeningSubmissionAcceptsEdgeValues(t *testing.T) {
	h := caseHarness(t)
	for name, p := range map[string]person{
		"punctuated name": {"3171011234567890", "Anne-Marie O’Neil, Jr.", yearsAgo(h, 30, 0)},
		"30 characters":   {"3171011234567890", strings.Repeat("a", 30), yearsAgo(h, 30, 0)},
		"17 today":        {"3171011234567890", "Budi Santoso", yearsAgo(h, 17, 0)},
		"100 years ago":   {"3171011234567890", "Budi Santoso", yearsAgo(h, 100, 0)},
	} {
		t.Run(name, func(t *testing.T) {
			c, caseID, assignmentID := newCase(t, h)
			if r := c.Do(t, http.MethodPost, submitPath(caseID, assignmentID, "submit"), preScreening("Perorangan", p, person{})); r.Status != http.StatusOK {
				t.Fatalf("submit = %d %s", r.Status, r.Raw)
			}
		})
	}
}

// AC-FR-APP-002-04: without a branch the case cannot be submitted.
func TestPreScreeningSubmissionWithoutBranchIsRefused(t *testing.T) {
	h := caseHarness(t)
	c := loggedIn(t, h, apptest.Operator{UserID: "AO009", RoleCode: "AO"})
	caseID, assignmentID := created(t, c.CreateCase(t, "Loan Financing"))
	got := fieldErrors(t, c.Do(t, http.MethodPost, submitPath(caseID, assignmentID, "submit"), preScreening("Perorangan", validPersonal(h), person{})))
	if fmt.Sprint(got) != fmt.Sprint([]fieldError{{"form", "Nama Branch", "ERR-006", "Branch ID operator tidak ditemukan"}}) {
		t.Fatalf("errors = %v", got)
	}
}

// Agreed: Badan Usaha arrives in ticket 31 and other customer types are
// TBD-029a, so they cannot be submitted; nor can fields of another form
// (BR-UI-010).
func TestPreScreeningSubmissionRefusesUnsupportedRequests(t *testing.T) {
	h := caseHarness(t)
	h.SeedBMV(t, "CUSTOMER_TYPE", "Koperasi", "Koperasi", 3, "Active")
	c, caseID, assignmentID := newCase(t, h)
	for name, body := range map[string]any{
		"Badan Usaha":   preScreening("Badan Usaha", validPersonal(h), person{}),
		"Koperasi":      preScreening("Koperasi", validPersonal(h), person{}),
		"not an option": preScreening("Lainnya", validPersonal(h), person{}),
		"foreign field": map[string]any{"customerType": "Perorangan", "customer": validPersonal(h).body(), "spouse": person{}.body(), "npwp": "1"},
	} {
		if r := c.Do(t, http.MethodPost, submitPath(caseID, assignmentID, "submit"), body); r.Status != http.StatusBadRequest {
			t.Fatalf("%s: status = %d %s, want 400", name, r.Status, r.Raw)
		}
	}
}

// Agreed: only the worklist owner submits; an Authors operator may look but
// not submit, and a completed assignment cannot be submitted again.
func TestOnlyTheOwnerSubmitsAnOpenAssignment(t *testing.T) {
	h := caseHarness(t)
	c, caseID, assignmentID := newCase(t, h)
	h.SeedMapping(t, apptest.Mapping{Role: "Ops Authors", AccessGroup: "Authors", WorkGroup: "Default"})
	authors := loggedIn(t, h, apptest.Operator{UserID: "AU01", RoleCode: "Ops Authors"})
	body := preScreening("Perorangan", validPersonal(h), person{})
	if r := authors.Do(t, http.MethodPost, submitPath(caseID, assignmentID, "submit"), body); r.Status != http.StatusForbidden {
		t.Fatalf("Authors submit = %d, want 403", r.Status)
	}
	if r := c.Do(t, http.MethodPost, submitPath(caseID, assignmentID, "submit"), body); r.Status != http.StatusOK {
		t.Fatalf("owner submit = %d %s", r.Status, r.Raw)
	}
	if r := c.Do(t, http.MethodPost, submitPath(caseID, assignmentID, "submit"), body); r.Status != http.StatusConflict {
		t.Fatalf("second submit = %d, want 409", r.Status)
	}
}

// AC-FR-ASSIGN-001-06, AC-FR-APP-002-23: going back creates a new
// Pre-Screening Submission row showing the saved data with Tipe Nasabah
// fixed; resubmitting updates the customer rows instead of adding more.
func TestBackToPreScreeningSubmissionAndResubmit(t *testing.T) {
	h := caseHarness(t)
	c, caseID, first := newCase(t, h)
	spouse := person{"3171019876543210", "Siti Aminah", yearsAgo(h, 28, 0)}
	duu := c.Do(t, http.MethodPost, submitPath(caseID, first, "submit"), preScreening("Perorangan", validPersonal(h), spouse)).Body["assignmentId"]

	back := c.Do(t, http.MethodPost, submitPath(caseID, duu, "back"), nil)
	if back.Status != http.StatusOK {
		t.Fatalf("back = %d %s", back.Status, back.Raw)
	}
	as := assignments(t, h, caseID)
	if len(as) != 3 || as[1].Status != "sudah selesai dikerjakan" || as[2].Name != "Pre-Screening Submission" ||
		as[2].Status != "sudah dibuka" || as[0].Name != "Pre-Screening Submission" {
		t.Fatalf("assignments = %+v", as)
	}
	again := back.Body["assignmentId"]
	page := c.OpenAssignment(t, caseID, again)
	if field(t, page, "page", "customerType") != "Perorangan" || field(t, page, "page", "customer", "nik") != "3171011234567890" ||
		field(t, page, "page", "customer", "birthDate") != yearsAgo(h, 30, 0) || field(t, page, "page", "spouse", "name") != "Siti Aminah" {
		t.Fatalf("page = %s", page.Raw)
	}

	// Tipe Nasabah is fixed after the first submit.
	if r := c.Do(t, http.MethodPost, submitPath(caseID, again, "submit"), preScreening("Badan Usaha", validPersonal(h), person{})); r.Status != http.StatusBadRequest {
		t.Fatalf("changing Tipe Nasabah = %d, want 400", r.Status)
	}
	changed := person{"3171011234567890", "Budi Santoso Baru", yearsAgo(h, 30, 0)}
	if r := c.Do(t, http.MethodPost, submitPath(caseID, again, "submit"), preScreening("Perorangan", changed, person{})); r.Status != http.StatusOK {
		t.Fatalf("resubmit = %d %s", r.Status, r.Raw)
	}
	cs := customers(t, h, caseID)
	if len(cs) != 1 || cs["Nasabah"].Name != "Budi Santoso Baru" {
		t.Fatalf("customers after resubmit without spouse = %v", cs)
	}
}

// Back only applies to an open Document Underlying Upload of the owner.
func TestBackIsOnlyFromDocumentUnderlyingUpload(t *testing.T) {
	h := caseHarness(t)
	c, caseID, assignmentID := newCase(t, h)
	if r := c.Do(t, http.MethodPost, submitPath(caseID, assignmentID, "back"), nil); r.Status != http.StatusBadRequest {
		t.Fatalf("back from Pre-Screening Submission = %d, want 400", r.Status)
	}
}

// The page tells the form whether it may submit and the birth-date bounds
// in the operator's dates (agreed after review).
func TestPreScreeningPageEditabilityAndDateBounds(t *testing.T) {
	h := caseHarness(t)
	c, caseID, assignmentID := newCase(t, h)
	page := c.OpenAssignment(t, caseID, assignmentID)
	if field(t, page, "page", "editable") != true || field(t, page, "page", "today") != yearsAgo(h, 0, 0) ||
		field(t, page, "page", "earliestBirthDate") != yearsAgo(h, 100, 0) {
		t.Fatalf("page = %s", page.Raw)
	}
	h.SeedMapping(t, apptest.Mapping{Role: "Ops Authors", AccessGroup: "Authors", WorkGroup: "Default"})
	authors := loggedIn(t, h, apptest.Operator{UserID: "AU01", RoleCode: "Ops Authors"})
	if got := field(t, authors.OpenAssignment(t, caseID, assignmentID), "page", "editable"); got != false {
		t.Fatalf("Authors editable = %v, want false", got)
	}
}
