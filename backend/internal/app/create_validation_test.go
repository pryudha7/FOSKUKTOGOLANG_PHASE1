package app_test

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"fos/internal/apptest"
)

// createError is one error of a refused Create, with its stable key.
type createError struct{ Section, Key, Field, Code, Message string }

func createFieldErrors(t *testing.T, r apptest.Response) []createError {
	t.Helper()
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d %s, want 422", r.Status, r.Raw)
	}
	var out []createError
	for _, e := range r.Body["errors"].([]any) {
		m := e.(map[string]any)
		out = append(out, createError{m["section"].(string), m["key"].(string), m["field"].(string), m["code"].(string), m["message"].(string)})
	}
	return out
}

// createErrors is "<code> <section>/<key>" for each error of a refused Create.
func createErrors(t *testing.T, r apptest.Response) []string {
	t.Helper()
	var out []string
	for _, e := range createFieldErrors(t, r) {
		out = append(out, e.Code+" "+e.Section+"/"+e.Key)
	}
	return out
}

func with(base map[string]string, key, value string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	out[key] = value
	return out
}

// AC-UI-011-06, -15, ERR-010, ERR-029: every SLIK field is required;
// Alamat Nasabah is at most 350 characters, CRLF counting as one.
func TestCreateValidatesSLIKRequestData(t *testing.T) {
	h, c, caseID, id := readyToCreate(t)
	before := assignments(t, h, caseID)

	empty := map[string]string{"requestPurpose": "", "facilityOption": " ", "address": "  ", "searchType": "", "segmentation": ""}
	r := createUnderlying(t, c, caseID, id, empty)
	want := []string{"ERR-010 slik/requestPurpose", "ERR-010 slik/facilityOption", "ERR-010 slik/address",
		"ERR-010 slik/searchType", "ERR-010 slik/segmentation"}
	if got := createErrors(t, r); !reflect.DeepEqual(got, want) {
		t.Fatalf("errors = %v", got)
	}
	if e := createFieldErrors(t, r)[2]; e.Field != "Alamat Nasabah" || e.Message != "Alamat Nasabah wajib diisi" {
		t.Fatalf("address error = %+v", e)
	}

	long := strings.Repeat("é", 351)
	r = createUnderlying(t, c, caseID, id, with(validSLIK(), "address", long))
	if got := createErrors(t, r); !reflect.DeepEqual(got, []string{"ERR-029 slik/address"}) ||
		createFieldErrors(t, r)[0].Message != "Alamat Nasabah maksimal 350 karakter" {
		t.Fatalf("351 characters: %s", r.Raw)
	}
	if !reflect.DeepEqual(assignments(t, h, caseID), before) || storedSLIK(t, h, caseID).Purpose != nil {
		t.Fatal("a refused Create changed the case")
	}

	// 350 characters with a CRLF line break pass and are stored with LF.
	address := strings.Repeat("a", 174) + "\r\n" + strings.Repeat("b", 175)
	if r := createUnderlying(t, c, caseID, id, with(validSLIK(), "address", address)); r.Status != http.StatusOK {
		t.Fatalf("350 characters = %d %s", r.Status, r.Raw)
	}
	if got := deref(storedSLIK(t, h, caseID).Address); got != strings.Repeat("a", 174)+"\n"+strings.Repeat("b", 175) {
		t.Fatalf("stored address = %q", got)
	}
}

// A code the dropdown does not offer is not a request the page sends.
func TestCreateRefusesCodesNotOffered(t *testing.T) {
	h, c, caseID, id := readyToCreate(t)
	h.SeedBMV(t, "SLIK_SEGMENTASI", "SG-0", "Lama", 0, "Inactive")
	for _, body := range []map[string]string{with(validSLIK(), "segmentation", "SG-0"), with(validSLIK(), "requestPurpose", "XX")} {
		if r := createUnderlying(t, c, caseID, id, body); r.Status != http.StatusBadRequest {
			t.Fatalf("%v = %d", body, r.Status)
		}
	}
	if r := createUnderlying(t, c, caseID, id, map[string]any{"requestPurpose": "TP-1", "other": "x"}); r.Status != http.StatusBadRequest {
		t.Fatalf("unknown field = %d", r.Status)
	}
}

const mb = 1024 * 1024

// AC-FR-FILE-004-02, -06, -07: the total of the screen's files over 10 MB is
// ERR-020 with the total rounded up to two decimals; a missing mandatory
// document is ERR-026 on its row; both show together. Uploading itself never
// checks the total.
func TestCreateChecksTotalSizeAndMandatoryDocuments(t *testing.T) {
	h, _ := uploadHarness(t)
	seedSLIKOptions(t, h)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	for _, f := range []struct {
		doc  string
		size int
	}{{"KK", 5 * mb}, {"KTP", 10894704 - 5*mb}} {
		if r := upload(t, c, caseID, id, f.doc, f.doc, "f.pdf", make([]byte, f.size)); r.Status != http.StatusCreated {
			t.Fatalf("upload %s = %d", f.doc, r.Status)
		}
	}

	r := createUnderlying(t, c, caseID, id, validSLIK())
	if got := createErrors(t, r); !reflect.DeepEqual(got, []string{"ERR-020 form/totalSize", "ERR-026 documents/DokumenPreScreening/Form Aplikasi Pembiayaan"}) {
		t.Fatalf("errors = %v", got)
	}
	es := createFieldErrors(t, r)
	if es[0].Field != "Notes" || es[0].Message != "Maksimal total ukuran file upload: 10 MB (Total: 10.39 MB)" {
		t.Fatalf("ERR-020 = %+v", es[0])
	}
	if es[1].Field != "Form Aplikasi Pembiayaan" || es[1].Message != "Harap upload dokumen wajib" {
		t.Fatalf("ERR-026 = %+v", es[1])
	}
}

// AC-FR-FILE-004-03, -04 and the limit's edge: replaced files do not count;
// exactly 10 MB passes; one byte more reads 10.01 MB.
func TestCreateCountsOnlyCurrentFiles(t *testing.T) {
	h, _ := uploadHarness(t)
	seedSLIKOptions(t, h)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	up := func(doc string, size int) {
		t.Helper()
		if r := upload(t, c, caseID, id, doc, doc, "f.pdf", make([]byte, size)); r.Status != http.StatusCreated {
			t.Fatalf("upload %s = %d", doc, r.Status)
		}
	}
	up("KK", 9*mb)
	up("KK", 2*mb)
	up("Form Aplikasi Pembiayaan", 3*mb)

	r := createUnderlying(t, c, caseID, id, validSLIK())
	if got := createErrors(t, r); !reflect.DeepEqual(got, []string{"ERR-026 documents/DokumenPreScreening/KTP"}) {
		t.Fatalf("errors = %v (the replaced KK must not count)", got)
	}

	up("KTP", 5*mb+1)
	r = createUnderlying(t, c, caseID, id, validSLIK())
	if es := createFieldErrors(t, r); len(es) != 1 || es[0].Message != "Maksimal total ukuran file upload: 10 MB (Total: 10.01 MB)" {
		t.Fatalf("one byte over = %s", r.Raw)
	}
	up("KTP", 5*mb)
	if r := createUnderlying(t, c, caseID, id, validSLIK()); r.Status != http.StatusOK {
		t.Fatalf("exactly 10 MB = %d %s", r.Status, r.Raw)
	}
}

// Agreed for TBD-029: without a known Team Lead, Create is refused and logged.
func TestCreateWithoutTeamLeadIsRefused(t *testing.T) {
	h, c, caseID, id := readyToCreate(t)
	exec(t, h, `UPDATE fos.work SET branch_officer_team_lead = 'TL99' WHERE case_id = $1`, caseID)
	logs := apptest.CaptureLogs(t)
	before := assignments(t, h, caseID)

	r := createUnderlying(t, c, caseID, id, validSLIK())
	if es := createFieldErrors(t, r); len(es) != 1 || es[0].Key != "teamLead" || es[0].Message != "Team Lead case tidak ditemukan" {
		t.Fatalf("errors = %s", r.Raw)
	}
	if !reflect.DeepEqual(assignments(t, h, caseID), before) {
		t.Fatal("assignments changed")
	}
	if !strings.Contains(logs.String(), "TL99") {
		t.Fatalf("not logged: %s", logs)
	}
}

// FR-APP-009 step 3: an empty case_status becomes New; a set one stays.
func TestCreateSetsCaseStatusOnlyWhenEmpty(t *testing.T) {
	h, c, caseID, id := readyToCreate(t)
	exec(t, h, `UPDATE fos.work SET case_status = NULL WHERE case_id = $1`, caseID)
	if r := createUnderlying(t, c, caseID, id, validSLIK()); r.Status != http.StatusOK {
		t.Fatalf("create = %d", r.Status)
	}
	if got := deref(storedSLIK(t, h, caseID).Status); got != "New" {
		t.Fatalf("case_status = %s", got)
	}
	h2, c2, other, id2 := readyToCreate(t)
	exec(t, h2, `UPDATE fos.work SET case_status = 'Pending' WHERE case_id = $1`, other)
	createUnderlying(t, c2, other, id2, validSLIK())
	if got := deref(storedSLIK(t, h2, other).Status); got != "Pending" {
		t.Fatalf("case_status = %s", got)
	}
}

// Only the owner creates, on an open Document Underlying Upload; the saved
// SLIK Request Data shows when the assignment is opened again.
func TestCreateIsOnlyByTheOwnerOnAnOpenAssignment(t *testing.T) {
	h, c, caseID, id := readyToCreate(t)
	other := loggedIn(t, h, apptest.Operator{UserID: "AO002", RoleCode: "AO", BranchCode: "032"})
	if r := createUnderlying(t, other, caseID, id, validSLIK()); r.Status != http.StatusForbidden {
		t.Fatalf("other operator = %d", r.Status)
	}
	if r := createUnderlying(t, c, caseID, id, validSLIK()); r.Status != http.StatusOK {
		t.Fatalf("create = %d", r.Status)
	}
	if r := createUnderlying(t, c, caseID, id, validSLIK()); r.Status != http.StatusConflict {
		t.Fatalf("second create = %d", r.Status)
	}
	r := c.OpenAssignment(t, caseID, id)
	values, _ := field(t, r, "page", "slik", "values").(map[string]any)
	if values["requestPurpose"] != "TP-1" || values["address"] != "Jl. Sudirman No. 1, Jakarta" || field(t, r, "page", "editable") != false {
		t.Fatalf("reopened = %s", r.Raw)
	}
	first := assignments(t, h, caseID)[0].ID
	if r := createUnderlying(t, c, caseID, first, validSLIK()); r.Status != http.StatusConflict {
		t.Fatalf("completed Pre-Screening Submission = %d", r.Status)
	}
}

// Badan Usaha has no SLIK Request Data and its Create is ticket 31.
func TestBusinessEntityHasNoSLIKRequestData(t *testing.T) {
	h, c, caseID, id := readyToCreate(t)
	exec(t, h, `UPDATE fos.work SET customer_type = 'Badan Usaha' WHERE case_id = $1`, caseID)
	if r := c.OpenAssignment(t, caseID, id); r.Status != http.StatusOK || field(t, r, "page", "slik") != nil {
		t.Fatalf("Badan Usaha page = %s", r.Raw)
	}
	if r := createUnderlying(t, c, caseID, id, validSLIK()); r.Status != http.StatusBadRequest {
		t.Fatalf("Badan Usaha create = %d", r.Status)
	}
}
