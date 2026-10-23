package app_test

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"fos/internal/apptest"
)

// underlying creates a Perorangan case as AO001 and moves it to Document
// Underlying Upload, returning the client, case and that assignment.
func underlying(t *testing.T, h *apptest.Harness) (*apptest.Client, string, any) {
	t.Helper()
	c, caseID, first := newCase(t, h)
	return c, caseID, toUnderlying(t, h, c, caseID, first)
}

// toUnderlying submits a valid Perorangan Pre-Screening Submission and
// returns the Document Underlying Upload assignment.
func toUnderlying(t *testing.T, h *apptest.Harness, c *apptest.Client, caseID string, first any) any {
	t.Helper()
	r := c.Do(t, http.MethodPost, submitPath(caseID, first, "submit"), preScreening("Perorangan", validPersonal(h), person{}))
	if r.Status != http.StatusOK {
		t.Fatalf("submit pre-screening = %d %s", r.Status, r.Raw)
	}
	return r.Body["assignmentId"]
}

// documentRow is one row of the UI-011 document list as the page sends it.
type documentRow struct {
	Type      string
	Mandatory bool
	HasFile   bool
}

// documentList returns each category label with its rows, in display order.
func documentList(t *testing.T, r apptest.Response) (labels []string, rows map[string][]documentRow) {
	t.Helper()
	if r.Status != http.StatusOK {
		t.Fatalf("open = %d %s", r.Status, r.Raw)
	}
	page, _ := r.Body["page"].(map[string]any)
	cats, ok := page["categories"].([]any)
	if !ok {
		t.Fatalf("no categories in %s", r.Raw)
	}
	rows = map[string][]documentRow{}
	for _, c := range cats {
		cat := c.(map[string]any)
		label := cat["label"].(string)
		labels = append(labels, label)
		for _, d := range cat["documents"].([]any) {
			doc := d.(map[string]any)
			rows[label] = append(rows[label], documentRow{doc["documentType"].(string), doc["mandatory"].(bool), doc["file"] != nil})
		}
	}
	return labels, rows
}

var individualDocuments = []documentRow{
	{"KK", true, false}, {"KTP", true, false}, {"Buku Nikah", false, false},
	{"Form Aplikasi Pembiayaan", true, false}, {"Memo Usulan Pembiayaan", false, false},
	{"Laporan Kunjungan Nasabah", false, false}, {"Form Permohonan SLIK", false, false}, {"SLIK", false, false},
}

// AC-FR-FILE-005-01, AC-FR-FILE-005-06, AC-UI-011-03: the first opening
// copies the master list onto the case and the page shows it from there.
func TestFirstOpeningInitialisesTheDocumentList(t *testing.T) {
	h := caseHarness(t)
	c, caseID, id := underlying(t, h)

	labels, rows := documentList(t, c.OpenAssignment(t, caseID, id))
	if !reflect.DeepEqual(labels, []string{"Dokumen Pre-Screening"}) {
		t.Fatalf("categories = %v", labels)
	}
	if !reflect.DeepEqual(rows["Dokumen Pre-Screening"], individualDocuments) {
		t.Fatalf("documents = %+v", rows["Dokumen Pre-Screening"])
	}
	if n := count(t, h, `SELECT count(*) FROM fos.case_document_category WHERE case_id = $1 AND list_source = 'PRESCREENING'
		AND document_category = 'DokumenPreScreening' AND source_code = 'DOCPRE-2'`, caseID); n != 1 {
		t.Fatalf("case_document_category rows = %d", n)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.case_document_requirement WHERE case_id = $1 AND list_source = 'PRESCREENING'
		AND source_row_uuid IS NOT NULL`, caseID); n != 8 {
		t.Fatalf("case_document_requirement rows = %d", n)
	}
}

func exec(t *testing.T, h *apptest.Harness, sql string, args ...any) {
	t.Helper()
	if _, err := h.DB.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

// seedCategory adds a BMV DOCUMENTPRESCREENING row for Perorangan.
func seedCategory(t *testing.T, h *apptest.Harness, code, label string, sortOrder int, status string) {
	t.Helper()
	exec(t, h, `INSERT INTO fos.business_mapping_value (code, type, sub_type, description, sort_order, status, created_at, created_by, updated_at, updated_by)
		VALUES ($1, 'DOCUMENTPRESCREENING', 'Perorangan', $2, $3, $4, now(), 'test', now(), 'test')`, code, label, sortOrder, status)
}

// seedDocument adds a Document Mapping Value row for Perorangan.
func seedDocument(t *testing.T, h *apptest.Harness, category, description string, sortOrder int, mandatory bool) {
	t.Helper()
	exec(t, h, `INSERT INTO fos.document_mapping_value (category, sub_category, description, sort_order, is_mandatory, created_at, created_by, updated_at, updated_by)
		VALUES ($1, 'Perorangan', $2, $3, $4, now(), 'test', now(), 'test')`, category, description, sortOrder, mandatory)
}

// AC-FR-FILE-005-03, -04, -07: once initialised the case keeps its own list.
func TestDocumentListIsASnapshotPerCase(t *testing.T) {
	h := caseHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))

	seedDocument(t, h, "Dokumen Pre-Screening", "NPWP", 9, true)
	exec(t, h, `UPDATE fos.document_mapping_value SET is_mandatory = FALSE WHERE description = 'KK'`)

	_, rows := documentList(t, c.OpenAssignment(t, caseID, id))
	if !reflect.DeepEqual(rows["Dokumen Pre-Screening"], individualDocuments) {
		t.Fatalf("reopened documents = %+v", rows["Dokumen Pre-Screening"])
	}
	if n := count(t, h, `SELECT count(*) FROM fos.case_document_requirement WHERE case_id = $1`, caseID); n != 8 {
		t.Fatalf("requirements after reopening = %d", n)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.case_document_category WHERE case_id = $1`, caseID); n != 1 {
		t.Fatalf("categories after reopening = %d", n)
	}

	c2, other, id2 := underlying(t, h)
	_, rows = documentList(t, c2.OpenAssignment(t, other, id2))
	got := rows["Dokumen Pre-Screening"]
	if len(got) != 9 || got[0] != (documentRow{"KK", false, false}) || got[8] != (documentRow{"NPWP", true, false}) {
		t.Fatalf("new case documents = %+v", got)
	}
}

// AC-FR-FILE-005-05, -08, -10: categories are added when new; a category
// still without documents runs Query 2 again; inactive categories are skipped.
func TestDocumentListGrowsWithNewMasterCategories(t *testing.T) {
	h := caseHarness(t)
	seedCategory(t, h, "DOCPRE-9", "Dokumen Tambahan", 2, "Active")
	seedCategory(t, h, "DOCPRE-8", "Dokumen Lama", 3, "Inactive")
	seedDocument(t, h, "Dokumen Lama", "Surat Lama", 1, false)
	c, caseID, id := underlying(t, h)

	labels, rows := documentList(t, c.OpenAssignment(t, caseID, id))
	if !reflect.DeepEqual(labels, []string{"Dokumen Pre-Screening", "Dokumen Tambahan"}) || len(rows["Dokumen Tambahan"]) != 0 {
		t.Fatalf("categories = %v %v", labels, rows)
	}

	seedDocument(t, h, "Dokumen Tambahan", "Surat Keterangan Usaha", 1, true)
	seedCategory(t, h, "DOCPRE-7", "Dokumen Pendukung", 0, "")
	seedDocument(t, h, "Dokumen Pendukung", "Foto Usaha", 1, false)
	labels, rows = documentList(t, c.OpenAssignment(t, caseID, id))
	if !reflect.DeepEqual(labels, []string{"Dokumen Pendukung", "Dokumen Pre-Screening", "Dokumen Tambahan"}) {
		t.Fatalf("categories = %v", labels)
	}
	if !reflect.DeepEqual(rows["Dokumen Tambahan"], []documentRow{{"Surat Keterangan Usaha", true, false}}) ||
		!reflect.DeepEqual(rows["Dokumen Pendukung"], []documentRow{{"Foto Usaha", false, false}}) {
		t.Fatalf("documents = %+v", rows)
	}
	if !reflect.DeepEqual(rows["Dokumen Pre-Screening"], individualDocuments) {
		t.Fatalf("existing category changed: %+v", rows["Dokumen Pre-Screening"])
	}
}

// AC-FR-FILE-005-09: a failing query gives ERR-027, is logged, and leaves
// the case's rows as they were.
func TestFailedDocumentListInitialisationGivesERR027(t *testing.T) {
	h := caseHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	seedCategory(t, h, "DOCPRE-9", "Dokumen Tambahan", 2, "Active")
	logs := apptest.CaptureLogs(t)
	exec(t, h, `ALTER TABLE fos.document_mapping_value RENAME TO document_mapping_value_gone`)

	r := c.OpenAssignment(t, caseID, id)
	labels, rows := documentList(t, r)
	if e, _ := field(t, r, "page", "listError").(map[string]any); e["code"] != "ERR-027" ||
		e["message"] != "Daftar dokumen gagal dimuat. Silakan coba lagi." {
		t.Fatalf("listError = %v", e)
	}
	if !reflect.DeepEqual(labels, []string{"Dokumen Pre-Screening"}) || len(rows["Dokumen Pre-Screening"]) != 8 {
		t.Fatalf("list after failure = %v %v", labels, rows)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.case_document_category WHERE case_id = $1`, caseID); n != 1 {
		t.Fatalf("categories = %d, want the failed run rolled back", n)
	}
	if !strings.Contains(logs.String(), "ERR-027") {
		t.Fatalf("not logged: %s", logs)
	}
}

// Only the owner's opening initialises; an Authors look changes nothing.
func TestAuthorsLookDoesNotInitialiseTheDocumentList(t *testing.T) {
	h := caseHarness(t)
	_, caseID, id := underlying(t, h)
	h.SeedMapping(t, apptest.Mapping{Role: "AUD", AccessGroup: "Authors", WorkGroup: "Audit"})
	auditor := loggedIn(t, h, apptest.Operator{UserID: "AUD01", RoleCode: "AUD"})

	r := auditor.OpenAssignment(t, caseID, id)
	labels, _ := documentList(t, r)
	if len(labels) != 0 || field(t, r, "page", "editable") != false {
		t.Fatalf("authors look = %s", r.Raw)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.case_document_category WHERE case_id = $1`, caseID); n != 0 {
		t.Fatalf("categories = %d", n)
	}
}

// memoNumber is the Nomor Memo the page shows and the one stored on the case.
func memoNumber(t *testing.T, h *apptest.Harness, r apptest.Response, caseID string) (shown, stored string) {
	t.Helper()
	if r.Status != http.StatusOK {
		t.Fatalf("open = %d %s", r.Status, r.Raw)
	}
	shown, _ = field(t, r, "page", "memoNumber").(string)
	return shown, storedMemo(t, h, caseID)
}

func storedMemo(t *testing.T, h *apptest.Harness, caseID string) string {
	t.Helper()
	var p *string
	if err := h.DB.QueryRow(context.Background(), `SELECT memo_number FROM fos.work WHERE case_id = $1`, caseID).Scan(&p); err != nil {
		t.Fatal(err)
	}
	if p == nil {
		return ""
	}
	return *p
}

// AC-FR-APP-008-01, -02, -03: the Nomor Memo is formed once, from the
// opening operator's Branch Code, UserID and local time (24-hour clock).
func TestNomorMemoIsFormedOnceOnOpening(t *testing.T) {
	h := caseHarness(t)
	h.SeedBMV(t, "BRANCH", "002", "Jakarta-Thamrin", 2, "Active")
	h.SeedOperator(t, apptest.Operator{UserID: "TL01", Password: "Rahasia#1", RoleCode: "TL"})
	c := loggedIn(t, h, apptest.Operator{UserID: "00801808", RoleCode: "AO", BranchCode: "002", ReportTo: "TL01"})
	caseID, first := created(t, c.CreateCase(t, "Loan Financing"))
	id := toUnderlying(t, h, c, caseID, first)
	if before := storedMemo(t, h, caseID); before != "" {
		t.Fatalf("memo before opening = %q", before)
	}

	h.Clock.Set(time.Date(2026, 10, 7, 11, 40, 1, 0, time.UTC)) // 18:40:01 at GMT+7
	c = relogin(t, h, "00801808")
	shown, stored := memoNumber(t, h, c.OpenAssignment(t, caseID, id), caseID)
	if shown != "002/00801808/20261007184001" || stored != shown {
		t.Fatalf("memo shown %q, stored %q", shown, stored)
	}

	h.Clock.Advance(time.Hour)
	c = relogin(t, h, "00801808")
	back := c.Do(t, http.MethodPost, submitPath(caseID, id, "back"), nil)
	again := toUnderlying(t, h, c, caseID, back.Body["assignmentId"])
	if shown, stored := memoNumber(t, h, c.OpenAssignment(t, caseID, again), caseID); shown != "002/00801808/20261007184001" || stored != shown {
		t.Fatalf("memo after reopening: shown %q, stored %q", shown, stored)
	}

	// AC-FR-APP-008-03: two-digit fields with leading zeros, 24-hour clock.
	h.Clock.Set(time.Date(2026, 1, 1, 23, 5, 9, 0, time.UTC)) // 2026-01-02 06:05:09 at GMT+7
	c = relogin(t, h, "00801808")
	other, first := created(t, c.CreateCase(t, "Loan Financing"))
	id = toUnderlying(t, h, c, other, first)
	if shown, _ := memoNumber(t, h, c.OpenAssignment(t, other, id), other); shown != "002/00801808/20260102060509" {
		t.Fatalf("memo = %q", shown)
	}
}

// AC-FR-APP-008-04: Badan Usaha cases get a Nomor Memo too.
func TestNomorMemoIsStoredForBusinessEntityCases(t *testing.T) {
	h := caseHarness(t)
	c, caseID, id := underlying(t, h)
	exec(t, h, `UPDATE fos.work SET customer_type = 'Badan Usaha' WHERE case_id = $1`, caseID)
	if _, stored := memoNumber(t, h, c.OpenAssignment(t, caseID, id), caseID); !strings.HasPrefix(stored, "032/AO001/") {
		t.Fatalf("stored memo = %q", stored)
	}
}

// Agreed for TBD-027: without a Branch Code no Nomor Memo is formed; the
// failure is logged and the next opening tries again.
func TestNomorMemoWaitsForTheBranchCode(t *testing.T) {
	h := caseHarness(t)
	c, caseID, id := underlying(t, h)
	exec(t, h, `UPDATE fos.operator SET branch_code = NULL WHERE user_id = 'AO001'`)
	logs := apptest.CaptureLogs(t)
	if shown, stored := memoNumber(t, h, c.OpenAssignment(t, caseID, id), caseID); shown != "" || stored != "" {
		t.Fatalf("memo without branch: shown %q, stored %q", shown, stored)
	}
	if !strings.Contains(logs.String(), "Nomor Memo") {
		t.Fatalf("not logged: %s", logs)
	}
	exec(t, h, `UPDATE fos.operator SET branch_code = '032' WHERE user_id = 'AO001'`)
	if shown, _ := memoNumber(t, h, c.OpenAssignment(t, caseID, id), caseID); !strings.HasPrefix(shown, "032/AO001/") {
		t.Fatalf("memo after branch set = %q", shown)
	}
}

// relogin starts a new session for a seeded operator, e.g. after the clock
// jumped past the idle timeout.
func relogin(t *testing.T, h *apptest.Harness, userID string) *apptest.Client {
	t.Helper()
	c := h.NewClient()
	if r := c.LoginDatabase(t, userID, "Rahasia#1"); r.Status != http.StatusOK {
		t.Fatalf("login %s = %d", userID, r.Status)
	}
	return c
}
