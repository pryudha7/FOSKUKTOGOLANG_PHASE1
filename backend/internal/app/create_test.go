package app_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"fos/internal/apptest"
)

// seedSLIKOptions adds one Active option to each SLIK Request Data dropdown.
func seedSLIKOptions(t *testing.T, h *apptest.Harness) {
	t.Helper()
	h.SeedBMV(t, "SLIK_TUJUANPERMINTAAN", "TP-1", "Penilaian Calon Debitur", 1, "Active")
	h.SeedBMV(t, "SLIK_PILIHANFASILITAS", "PF-1", "Seluruh", 1, "Active")
	h.SeedBMV(t, "SLIK_TIPEPENCARIAN", "TC-1", "Individual", 1, "Active")
	h.SeedBMV(t, "SLIK_SEGMENTASI", "SG-1", "Ritel", 1, "Active")
}

// slikOptions returns the descriptions of one dropdown on the page.
func slikOptions(t *testing.T, r apptest.Response, key string) []string {
	t.Helper()
	opts, ok := field(t, r, "page", "slik", "options", key).([]any)
	if !ok {
		t.Fatalf("no %s options in %s", key, r.Raw)
	}
	out := []string{}
	for _, o := range opts {
		m := o.(map[string]any)
		out = append(out, m["code"].(string)+"="+m["description"].(string))
	}
	return out
}

// AC-UI-011-11, -12: each dropdown lists its BMV type's Active rows by
// sort_order then code; a type without rows gives an empty list.
func TestSLIKRequestDataOptions(t *testing.T) {
	h := caseHarness(t)
	h.SeedBMV(t, "SLIK_TUJUANPERMINTAAN", "TP-2", "Restrukturisasi", 1, "Active")
	h.SeedBMV(t, "SLIK_TUJUANPERMINTAAN", "TP-1", "Penilaian Calon Debitur", 1, "Active")
	h.SeedBMV(t, "SLIK_TUJUANPERMINTAAN", "TP-0", "Lama", 0, "Inactive")
	h.SeedBMV(t, "SLIK_TUJUANPERMINTAAN", "TP-3", "Pemantauan", 0, "Active")
	h.SeedBMV(t, "SLIK_PILIHANFASILITAS", "PF-1", "Seluruh", 1, "Active")
	h.SeedBMV(t, "SLIK_TIPEPENCARIAN", "TC-1", "Individual", 1, "Active")
	c, caseID, id := underlying(t, h)

	r := c.OpenAssignment(t, caseID, id)
	if got := slikOptions(t, r, "requestPurpose"); !reflect.DeepEqual(got, []string{"TP-3=Pemantauan", "TP-1=Penilaian Calon Debitur", "TP-2=Restrukturisasi"}) {
		t.Fatalf("Tujuan Permintaan = %v", got)
	}
	if got := slikOptions(t, r, "facilityOption"); !reflect.DeepEqual(got, []string{"PF-1=Seluruh"}) {
		t.Fatalf("Pilihan Fasilitas = %v", got)
	}
	if got := slikOptions(t, r, "searchType"); !reflect.DeepEqual(got, []string{"TC-1=Individual"}) {
		t.Fatalf("Tipe Pencarian = %v", got)
	}
	if got := slikOptions(t, r, "segmentation"); len(got) != 0 {
		t.Fatalf("Segmentasi = %v", got)
	}
}

func validSLIK() map[string]string {
	return map[string]string{"requestPurpose": "TP-1", "facilityOption": "PF-1", "address": "Jl. Sudirman No. 1, Jakarta",
		"searchType": "TC-1", "segmentation": "SG-1"}
}

func createUnderlying(t *testing.T, c *apptest.Client, caseID string, id any, body any) apptest.Response {
	t.Helper()
	return c.Do(t, http.MethodPost, submitPath(caseID, id, "create"), body)
}

// readyToCreate is a case on Document Underlying Upload with SLIK options
// seeded and the three mandatory documents uploaded.
func readyToCreate(t *testing.T) (*apptest.Harness, *apptest.Client, string, any) {
	t.Helper()
	h, _ := uploadHarness(t)
	seedSLIKOptions(t, h)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	for _, d := range []string{"KK", "KTP", "Form Aplikasi Pembiayaan"} {
		if r := upload(t, c, caseID, id, d, d, "f.pdf", []byte("pdf")); r.Status != http.StatusCreated {
			t.Fatalf("upload %s = %d %s", d, r.Status, r.Raw)
		}
	}
	return h, c, caseID, id
}

type slikRow struct{ Purpose, Facility, Address, SearchType, Segmentation, Status *string }

func storedSLIK(t *testing.T, h *apptest.Harness, caseID string) slikRow {
	t.Helper()
	var r slikRow
	if err := h.DB.QueryRow(context.Background(), `
		SELECT slik_request_purpose_code, slik_facility_option_code, slik_identity_address, slik_search_type_code,
		       slik_segmentation_code, case_status
		FROM fos.work WHERE case_id = $1`, caseID).Scan(&r.Purpose, &r.Facility, &r.Address, &r.SearchType, &r.Segmentation, &r.Status); err != nil {
		t.Fatal(err)
	}
	return r
}

// AC-FR-FILE-004-01, -05, FR-APP-009: with the mandatory documents and SLIK
// Request Data, Create stores the data, completes the assignment and gives
// the Team Lead a new Pre Screening worklist assignment.
func TestCreateHandsTheCaseToTheTeamLead(t *testing.T) {
	h, c, caseID, id := readyToCreate(t)

	r := createUnderlying(t, c, caseID, id, validSLIK())
	if r.Status != http.StatusOK {
		t.Fatalf("create = %d %s", r.Status, r.Raw)
	}
	as := assignments(t, h, caseID)
	last, prev := as[len(as)-1], as[len(as)-2]
	if prev.Name != "Document Underlying Upload" || prev.Status != "sudah selesai dikerjakan" || prev.Completed == nil ||
		last.Name != "Pre Screening" || last.Status != "baru" || last.Holder != "TL01" || last.Completed != nil {
		t.Fatalf("assignments = %+v", as)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.assignments WHERE assignment_id = $1 AND assignment_type = 'worklist'
		AND first_opened_at IS NULL AND worked_by_user_id IS NULL`, last.ID); n != 1 {
		t.Fatal("Pre Screening is not a new worklist assignment")
	}
	got := storedSLIK(t, h, caseID)
	if deref(got.Purpose) != "TP-1" || deref(got.Facility) != "PF-1" || deref(got.Address) != "Jl. Sudirman No. 1, Jakarta" ||
		deref(got.SearchType) != "TC-1" || deref(got.Segmentation) != "SG-1" || deref(got.Status) != "New" {
		t.Fatalf("work = %+v", got)
	}
}
