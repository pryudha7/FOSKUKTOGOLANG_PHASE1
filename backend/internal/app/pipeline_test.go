package app_test

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"fos/internal/apptest"
)

// pefindoNotFound ends Pefindo at once with nothing found, so the run
// completes (FR-PSAPI-015).
func pefindoNotFound() map[string][]apptest.FakeReply {
	return map[string][]apptest.FakeReply{
		apptest.PefindoToken:  {apptest.PefindoTokenOK},
		apptest.PefindoSearch: {apptest.PefindoNotFound},
	}
}

// assignmentsNamed are the case's assignments of one name, oldest first.
func assignmentsNamed(t *testing.T, h *apptest.Harness, caseID, name string) []assignmentRow {
	t.Helper()
	var out []assignmentRow
	for _, a := range assignments(t, h, caseID) {
		if a.Name == name {
			out = append(out, a)
		}
	}
	return out
}

func processStarted(t *testing.T, h *apptest.Harness, caseID, process string) bool {
	t.Helper()
	return count(t, h, `
		SELECT count(*) FROM fos.case_process_progress p JOIN fos.lifecycle_process l ON l.process_id = p.process_id
		WHERE p.case_id = $1 AND l.process_name = $2`, caseID, process) == 1
}

// AC-FR-APP-004-01: a completed run gives the Branch Officer a new
// Pipeline Data Entry worklist assignment and the case Pending-Submission.
func TestCompletedRunOpensPipelineDataEntry(t *testing.T) {
	h, _, caseID := pefindoIndividual(t, false, pefindoNotFound())
	if got := run(t, h, caseID); got.State != "COMPLETED" {
		t.Fatalf("run = %+v", got)
	}
	got := assignmentsNamed(t, h, caseID, "Pipeline Data Entry")
	if len(got) != 1 || got[0].Holder != "AO001" || got[0].Status != "baru" {
		t.Fatalf("Pipeline Data Entry = %+v", got)
	}
	if s := caseStatus(t, h, caseID); s != "Pending-Submission" {
		t.Fatalf("case_status = %s", s)
	}
	if !processStarted(t, h, caseID, "Pipeline Data Entry") {
		t.Fatal("process Pipeline Data Entry not started")
	}
}

// seedPipelineMasters adds the test master data of FR-APP-004: Jenis
// Pengajuan (Take Over listed second by sort_order), the §9.26 test
// products and banks.
func seedPipelineMasters(t *testing.T, h *apptest.Harness) {
	t.Helper()
	h.SeedBMV(t, "SUBMISSION_TYPE", "Take Over", "Take Over", 2, "Active")
	h.SeedBMV(t, "SUBMISSION_TYPE", "New", "Pengajuan Baru", 1, "Active")
	h.SeedBMV(t, "SUBMISSION_TYPE", "Old", "Tidak Dipakai", 0, "Inactive")
	exec(t, h, `INSERT INTO fos.product_management (code, name, financing_limit_start, financing_limit_end, tenure_limit, contract_type,
		created_at, created_by, updated_at, updated_by) VALUES
		('798', 'testing_mikro_aira_murabahah', 5000000000, 7000000000, 84, 'Murabahah', now(), 'test', now(), 'test'),
		('794', 'testing_mikro_aira_qardh', 0, 2000000000, 60, 'Qardh', now(), 'test', now(), 'test')`)
	h.SeedBMV(t, "BANK", "014", "BCA", 2, "Active")
	h.SeedBMV(t, "BANK", "008", "Bank Mandiri", 1, "Active")
	h.SeedBMV(t, "BANK", "536", "BCA Syariah", 3, "Inactive")
	h.SeedBMV(t, "BANK", "009", "Bank bca Digital", 4, "")
}

// pipeline takes a case through a completed run to Pipeline Data Entry and
// returns the Branch Officer's client and the assignment.
func pipeline(t *testing.T, businessEntity bool) (*apptest.Harness, *apptest.Client, string, int64) {
	t.Helper()
	var h *apptest.Harness
	var caseID string
	if businessEntity {
		h, _, caseID = pefindoBusiness(t, pefindoNotFound())
	} else {
		h, _, caseID = pefindoIndividual(t, false, pefindoNotFound())
	}
	seedPipelineMasters(t, h)
	got := assignmentsNamed(t, h, caseID, "Pipeline Data Entry")
	if len(got) != 1 {
		t.Fatalf("Pipeline Data Entry = %+v", got)
	}
	return h, relogin(t, h, "AO001"), caseID, got[0].ID
}

func pipelinePage(t *testing.T, c *apptest.Client, caseID string, id int64) map[string]any {
	t.Helper()
	r := c.OpenAssignment(t, caseID, id)
	if r.Status != http.StatusOK {
		t.Fatalf("open = %d %s", r.Status, r.Raw)
	}
	return r.Body["page"].(map[string]any)
}

// optionTexts are "code=description" of each option.
func optionTexts(v any) []string {
	var out []string
	for _, o := range v.([]any) {
		m := o.(map[string]any)
		out = append(out, m["code"].(string)+"="+m["description"].(string))
	}
	return out
}

type pipelineForm struct {
	SubmissionType string `json:"submissionType"`
	ProductCode    string `json:"productCode"`
	BankName       string `json:"bankName"`
}

func submitPipeline(t *testing.T, c *apptest.Client, caseID string, id int64, f pipelineForm) apptest.Response {
	t.Helper()
	return c.Do(t, http.MethodPost, submitPath(caseID, id, "pipeline"), f)
}

type pipelineWork struct{ Branch, SubmissionType, Product, Contract, Bank *string }

func storedPipeline(t *testing.T, h *apptest.Harness, caseID string) pipelineWork {
	t.Helper()
	var w pipelineWork
	if err := h.DB.QueryRow(context.Background(), `
		SELECT branch_code, submission_type, product_code, contract_type, bank_name FROM fos.work WHERE case_id = $1`, caseID).
		Scan(&w.Branch, &w.SubmissionType, &w.Product, &w.Contract, &w.Bank); err != nil {
		t.Fatal(err)
	}
	return w
}

// AC-FR-APP-004-02, -04, -05, -10: the page shows the case's Branch and Tipe
// Nasabah, the Active Jenis Pengajuan by sort_order then code, every product
// by code with its name, and the KTP of Dokumen Pre-Screening, which
// downloads.
func TestPipelinePageShowsTheCaseAndItsOptions(t *testing.T) {
	_, c, caseID, id := pipeline(t, false)
	p := pipelinePage(t, c, caseID, id)
	if p["branchCode"] != "032" || p["branchName"] != "Pekanbaru-Imam Munandar" || p["customerType"] != "Perorangan" || p["editable"] != true {
		t.Fatalf("page = %v", p)
	}
	if got := optionTexts(p["submissionTypeOptions"]); !reflect.DeepEqual(got, []string{"New=Pengajuan Baru", "Take Over=Take Over"}) {
		t.Fatalf("Jenis Pengajuan = %v", got)
	}
	if got := optionTexts(p["productOptions"]); !reflect.DeepEqual(got, []string{"794=testing_mikro_aira_qardh", "798=testing_mikro_aira_murabahah"}) {
		t.Fatalf("Produk = %v", got)
	}
	if p["submissionType"] != "" || p["productCode"] != "" || p["bankName"] != "" || p["bankLetter"] != nil {
		t.Fatalf("saved values = %v", p)
	}
	identity := p["identity"].(map[string]any)
	file := identity["file"].(map[string]any)
	if identity["label"] != "ID / KTP" || file["name"] != "KTP" || file["fileName"] != "KTP.pdf" || file["caption"] != "DokumenPreScreening • Dokumen Pre-Screening" {
		t.Fatalf("identity = %v", identity)
	}
	if r := download(t, c, caseID, file["fileId"].(string)); r.Status != http.StatusOK || r.Raw != "pdf" {
		t.Fatalf("download KTP = %d %s", r.Status, r.Raw)
	}
}

// FR-APP-004 Field 5: a Badan Usaha case shows its NPWP Badan Usaha.
func TestPipelinePageShowsTheBusinessNPWP(t *testing.T) {
	h, c, caseID, id := pipeline(t, true)
	exec(t, h, `INSERT INTO fos.case_document (file_id, case_id, document_category, document_type, original_file_name, file_name,
		bucket_name, object_name, content_type, file_size_bytes, created_at, created_by)
		VALUES (gen_random_uuid(), $1, 'DokumenPreScreening', 'NPWP Badan Usaha', 'npwp.pdf', 'NPWP.pdf', 'b', 'o', 'application/pdf', 3, now(), 'AO001')`, caseID)
	p := pipelinePage(t, c, caseID, id)
	identity := p["identity"].(map[string]any)
	if p["customerType"] != "Badan Usaha" || identity["label"] != "NPWP Badan Usaha" || identity["file"].(map[string]any)["name"] != "NPWP Badan Usaha" {
		t.Fatalf("page = %v", p)
	}
}

// AC-FR-APP-004-03, DB-RULE-089: a Branch that is not Active is cleared on
// opening and Submit then asks for it.
func TestPipelineClearsAnInactiveBranch(t *testing.T) {
	h, c, caseID, id := pipeline(t, false)
	exec(t, h, `UPDATE fos.business_mapping_value SET status = 'Inactive' WHERE type = 'BRANCH' AND code = '032'`)
	p := pipelinePage(t, c, caseID, id)
	if p["branchCode"] != "" || p["branchName"] != "" || storedPipeline(t, h, caseID).Branch != nil {
		t.Fatalf("page = %v, work = %+v", p, storedPipeline(t, h, caseID))
	}
	got := fieldErrors(t, submitPipeline(t, c, caseID, id, pipelineForm{"New", "794", ""}))
	if !reflect.DeepEqual(got, []fieldError{{"form", "Kode Branch", "ERR-010", "Kode Branch wajib diisi"}}) {
		t.Fatalf("errors = %v", got)
	}
}

// AC-FR-APP-004-05, FR-ASSIGN-001 rule 16: Submit stores the codes and the
// product's contract, completes Pipeline Data Entry and gives the Branch
// Officer a new Data Pribadi assignment; case_status stays (agreed).
func TestPipelineSubmitMovesAnIndividualToDataPribadi(t *testing.T) {
	h, c, caseID, id := pipeline(t, false)
	r := submitPipeline(t, c, caseID, id, pipelineForm{"New", "798", "BCA"})
	if r.Status != http.StatusOK {
		t.Fatalf("submit = %d %s", r.Status, r.Raw)
	}
	w := storedPipeline(t, h, caseID)
	if deref(w.SubmissionType) != "New" || deref(w.Product) != "798" || deref(w.Contract) != "Murabahah" || w.Bank != nil {
		t.Fatalf("work = %+v", w)
	}
	if got := assignmentsNamed(t, h, caseID, "Pipeline Data Entry"); got[0].Status != "sudah selesai dikerjakan" || got[0].Completed == nil {
		t.Fatalf("Pipeline Data Entry = %+v", got)
	}
	next := assignmentsNamed(t, h, caseID, "Data Pribadi")
	if len(next) != 1 || next[0].Holder != "AO001" || next[0].Status != "baru" || r.Body["assignmentId"] != float64(next[0].ID) {
		t.Fatalf("Data Pribadi = %+v, response %s", next, r.Raw)
	}
	if !processStarted(t, h, caseID, "Data Pribadi") || caseStatus(t, h, caseID) != "Pending-Submission" {
		t.Fatalf("process or status: %s", caseStatus(t, h, caseID))
	}
	if r := submitPipeline(t, c, caseID, id, pipelineForm{"New", "794", ""}); r.Status != http.StatusConflict {
		t.Fatalf("second submit = %d", r.Status)
	}
}

// AC-FR-APP-004-05, -06, -09, FR-ASSIGN-001 rule 15: a Take Over stores the
// typed bank as it is; changing the product changes the contract; a Badan
// Usaha case moves to Badan Usaha.
func TestPipelineSubmitTakeOverForABusinessEntity(t *testing.T) {
	h, c, caseID, id := pipeline(t, true)
	if r := submitPipeline(t, c, caseID, id, pipelineForm{"Take Over", "794", "  Bank Lain Sekali "}); r.Status != http.StatusOK {
		t.Fatalf("submit = %d %s", r.Status, r.Raw)
	}
	w := storedPipeline(t, h, caseID)
	if deref(w.SubmissionType) != "Take Over" || deref(w.Product) != "794" || deref(w.Contract) != "Qardh" || deref(w.Bank) != "Bank Lain Sekali" {
		t.Fatalf("work = %+v", w)
	}
	if next := assignmentsNamed(t, h, caseID, "Badan Usaha"); len(next) != 1 || next[0].Status != "baru" || len(assignmentsNamed(t, h, caseID, "Data Pribadi")) != 0 {
		t.Fatalf("Badan Usaha = %+v", next)
	}
}

// FR-APP-004 Validation, AC-FR-APP-004-09: required fields (Nama Bank only
// for Take Over) are ERR-010; codes not in their source are ERR-037. A
// refused Submit changes nothing.
func TestPipelineSubmitValidation(t *testing.T) {
	h, c, caseID, id := pipeline(t, false)
	for _, tc := range []struct {
		form pipelineForm
		want []fieldError
	}{
		{pipelineForm{}, []fieldError{
			{"form", "Jenis Pengajuan", "ERR-010", "Jenis Pengajuan wajib diisi"},
			{"form", "Produk", "ERR-010", "Produk wajib diisi"},
		}},
		{pipelineForm{"Take Over", "794", "  "}, []fieldError{{"form", "Nama Bank", "ERR-010", "Nama Bank wajib diisi"}}},
		{pipelineForm{"Old", "999", ""}, []fieldError{
			{"form", "Jenis Pengajuan", "ERR-037", "Harap masukkan Jenis Pengajuan yang valid."},
			{"form", "Produk", "ERR-037", "Harap masukkan Produk yang valid."},
		}},
	} {
		if got := fieldErrors(t, submitPipeline(t, c, caseID, id, tc.form)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%+v: errors %v, want %v", tc.form, got, tc.want)
		}
	}
	if w := storedPipeline(t, h, caseID); w.SubmissionType != nil || w.Product != nil || w.Contract != nil || w.Bank != nil {
		t.Fatalf("refused submits stored %+v", w)
	}
	if got := assignmentsNamed(t, h, caseID, "Pipeline Data Entry"); got[0].Status == "sudah selesai dikerjakan" {
		t.Fatal("refused submit completed the assignment")
	}
}

func bankSuggestions(t *testing.T, c *apptest.Client, caseID string, id int64, q string) []string {
	t.Helper()
	r := c.Do(t, http.MethodGet, submitPath(caseID, id, "banks?q="+url.QueryEscape(q)), nil)
	if r.Status != http.StatusOK {
		t.Fatalf("banks = %d %s", r.Status, r.Raw)
	}
	return optionTexts(r.Body["options"])
}

// AC-FR-APP-004-06, BR-UI-005: Nama Bank suggests Active banks whose
// description contains the text, ignoring case, by sort_order then code, at
// most ui/autocomplete/max_results.
func TestPipelineBankSuggestions(t *testing.T) {
	h, c, caseID, id := pipeline(t, false)
	if got := bankSuggestions(t, c, caseID, id, "bca"); !reflect.DeepEqual(got, []string{"014=BCA", "009=Bank bca Digital"}) {
		t.Fatalf("bca = %v", got)
	}
	if got := bankSuggestions(t, c, caseID, id, "%"); len(got) != 0 {
		t.Fatalf("percent = %v", got)
	}
	exec(t, h, `UPDATE fos.dynamic_system_settings SET value = '2' WHERE purpose = 'ui/autocomplete/max_results'`)
	h.Clock.Advance(2 * time.Minute) // past the settings cache
	if got := bankSuggestions(t, c, caseID, id, ""); !reflect.DeepEqual(got, []string{"008=Bank Mandiri", "014=BCA"}) {
		t.Fatalf("empty = %v", got)
	}
}

const bankLetter = "Bukti Surat Take Over dari Bank"

func uploadBankLetter(t *testing.T, c *apptest.Client, caseID string, id int64, fileName string, data []byte) apptest.Response {
	t.Helper()
	return c.PostMultipart(t, submitPath(caseID, id, "bank-letter"), apptest.Upload{FileName: fileName, ContentType: "application/pdf", Data: data})
}

// AC-FR-APP-004-07, FR-APP-004 rules 6-8: the letter is stored when chosen
// under its fixed name, shown with its category, downloads, and a second
// file replaces it.
func TestPipelineBankLetterUpload(t *testing.T) {
	h, c, caseID, id := pipeline(t, false)
	r := uploadBankLetter(t, c, caseID, id, "surat.PDF", []byte("letter-1"))
	if r.Status != http.StatusCreated {
		t.Fatalf("upload = %d %s", r.Status, r.Raw)
	}
	first, ok := storedDocument(t, h, caseID, "DokumenInisiasi", bankLetter)
	if !ok || first.Name != bankLetter+".PDF" || first.Original != "surat.PDF" || string(first.Data) != "letter-1" || first.CreatedBy != "AO001" {
		t.Fatalf("stored = %+v", first)
	}
	if r.Body["fileId"] != first.FileID || r.Body["fileName"] != first.Name || r.Body["name"] != bankLetter || r.Body["caption"] != "DokumenInisiasi • Dokumen Inisiasi" {
		t.Fatalf("upload answer = %s", r.Raw)
	}
	letter := pipelinePage(t, c, caseID, id)["bankLetter"].(map[string]any)
	if letter["fileId"] != first.FileID || letter["name"] != bankLetter || letter["caption"] != "DokumenInisiasi • Dokumen Inisiasi" {
		t.Fatalf("bankLetter = %v", letter)
	}
	if d := download(t, c, caseID, first.FileID); d.Status != http.StatusOK || d.Raw != "letter-1" {
		t.Fatalf("download = %d %s", d.Status, d.Raw)
	}

	if r := uploadBankLetter(t, c, caseID, id, "baru.png", []byte("letter-2")); r.Status != http.StatusCreated {
		t.Fatalf("second upload = %d %s", r.Status, r.Raw)
	}
	second, _ := storedDocument(t, h, caseID, "DokumenInisiasi", bankLetter)
	if second.FileID == first.FileID || second.Name != bankLetter+".png" || string(second.Data) != "letter-2" {
		t.Fatalf("replaced = %+v", second)
	}
	for key := range fakeGCSOf[h].Objects() {
		if strings.HasSuffix(key, first.Object) {
			t.Fatal("the replaced object is still in GCS")
		}
	}
}

// AC-FR-APP-004-11, FR-APP-004 rule 8: a file over
// validation/upload/max_file_bytes is ERR-041, a wrong type ERR-021; nothing
// is stored.
func TestPipelineBankLetterRefused(t *testing.T) {
	h, c, caseID, id := pipeline(t, false)
	exec(t, h, `UPDATE fos.dynamic_system_settings SET value = '1000' WHERE purpose = 'validation/upload/max_file_bytes'`)
	h.Clock.Advance(2 * time.Minute)
	if got := uploadErrors(t, uploadBankLetter(t, c, caseID, id, "s.pdf", make([]byte, 1001)), http.StatusUnprocessableEntity); !reflect.DeepEqual(got, []string{"ERR-041"}) {
		t.Fatalf("too large = %v", got)
	}
	if got := uploadErrors(t, uploadBankLetter(t, c, caseID, id, "s.exe", []byte("x")), http.StatusUnprocessableEntity); !reflect.DeepEqual(got, []string{"ERR-021"}) {
		t.Fatalf("type = %v", got)
	}
	if _, ok := storedDocument(t, h, caseID, "DokumenInisiasi", bankLetter); ok {
		t.Fatal("a refused letter was stored")
	}
}

// AC-FR-APP-004-08, FR-APP-004 rule 3, DB-RULE-090: changing Jenis
// Pengajuan clears Nama Bank and deletes the letter, row and object, at
// once; the new choice itself waits for Submit.
func TestPipelineChangingSubmissionTypeDeletesTheLetter(t *testing.T) {
	h, c, caseID, id := pipeline(t, false)
	if r := uploadBankLetter(t, c, caseID, id, "s.pdf", []byte("letter")); r.Status != http.StatusCreated {
		t.Fatalf("upload = %d %s", r.Status, r.Raw)
	}
	letter, _ := storedDocument(t, h, caseID, "DokumenInisiasi", bankLetter)
	exec(t, h, `UPDATE fos.work SET bank_name = 'BCA', submission_type = 'Take Over', updated_by = 'someone' WHERE case_id = $1`, caseID)
	if r := c.Do(t, http.MethodPost, submitPath(caseID, id, "submission-type-change"), nil); r.Status != http.StatusNoContent {
		t.Fatalf("change = %d %s", r.Status, r.Raw)
	}
	if _, ok := storedDocument(t, h, caseID, "DokumenInisiasi", bankLetter); ok {
		t.Fatal("letter row kept")
	}
	for key := range fakeGCSOf[h].Objects() {
		if strings.HasSuffix(key, letter.Object) {
			t.Fatal("letter object kept")
		}
	}
	if w := storedPipeline(t, h, caseID); w.Bank != nil || deref(w.SubmissionType) != "Take Over" {
		t.Fatalf("work = %+v", w)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.work WHERE case_id = $1 AND updated_by = 'AO001'`, caseID); n != 1 {
		t.Fatal("the change did not record its operator")
	}
	if p := pipelinePage(t, c, caseID, id); p["bankLetter"] != nil || p["bankName"] != "" {
		t.Fatalf("page = %v", p)
	}
	// Without a letter the change still succeeds.
	if r := c.Do(t, http.MethodPost, submitPath(caseID, id, "submission-type-change"), nil); r.Status != http.StatusNoContent {
		t.Fatalf("second change = %d", r.Status)
	}
}

// Only the owner of the open Pipeline Data Entry may change it.
func TestPipelineActionsBelongToTheOwner(t *testing.T) {
	h, _, caseID, id := pipeline(t, false)
	tl := relogin(t, h, "TL01")
	for _, r := range []apptest.Response{
		submitPipeline(t, tl, caseID, id, pipelineForm{"New", "794", ""}),
		tl.Do(t, http.MethodPost, submitPath(caseID, id, "submission-type-change"), nil),
		uploadBankLetter(t, tl, caseID, id, "s.pdf", []byte("x")),
	} {
		if r.Status != http.StatusForbidden {
			t.Errorf("status = %d %s", r.Status, r.Raw)
		}
	}
}
