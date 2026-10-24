package app_test

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"fos/internal/apptest"
)

// qcaData takes a case through Data Permohonan Fasilitas to its QCA Data,
// with two Active answers per question ("Q<n>A" sorted after "Q<n>B" by
// sort_order) and one Inactive "Q<n>X".
func qcaData(t *testing.T) (*apptest.Harness, *apptest.Client, string, int64) {
	t.Helper()
	h, c, caseID, fid := facilityRequest(t)
	for n := 1; n <= 10; n++ {
		bmv := fmt.Sprintf("QCA%d", n)
		h.SeedBMV(t, bmv, fmt.Sprintf("Q%dA", n), fmt.Sprintf("Jawaban %d A", n), 2, "Active")
		h.SeedBMV(t, bmv, fmt.Sprintf("Q%dB", n), fmt.Sprintf("Jawaban %d B", n), 1, "Active")
		h.SeedBMV(t, bmv, fmt.Sprintf("Q%dX", n), fmt.Sprintf("Jawaban %d X", n), 0, "Inactive")
	}
	return h, c, caseID, movedTo(t, h, caseID, submitFacilityRequest(t, c, caseID, fid, "next", validFacilityRequestForm()), "QCA Data")
}

// allAnswers answers every question with its "A" option.
func allAnswers() []string {
	out := make([]string, 10)
	for i := range out {
		out[i] = fmt.Sprintf("Q%dA", i+1)
	}
	return out
}

func submitQCA(t *testing.T, c *apptest.Client, caseID string, id int64, to string, answers []string) apptest.Response {
	t.Helper()
	return c.Do(t, http.MethodPost, prospectPath(caseID, id, "qca", to), map[string]any{"answers": answers})
}

func qcaRows(t *testing.T, h *apptest.Harness, caseID string) []string {
	t.Helper()
	rows, err := h.DB.Query(context.Background(), `SELECT question_number || '|' || answer_code FROM fos.case_qca_answer WHERE case_id = $1 ORDER BY question_number`, caseID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

// AC-FR-APP-020-01, -03: QCA Data opens with ten unanswered questions; each
// dropdown lists its own BMV QCA<n>, Active only, by sort_order then code.
func TestQCADataPage(t *testing.T) {
	_, c, caseID, id := qcaData(t)
	p := pipelinePage(t, c, caseID, id)
	if !reflect.DeepEqual(p["values"], map[string]any{"answers": []any{"", "", "", "", "", "", "", "", "", ""}}) || p["editable"] != true ||
		p["facilityRequested"] != true {
		t.Fatalf("page = %v", p)
	}
	opts := p["options"].(map[string]any)
	if got := optionTexts(opts["qca1"]); !reflect.DeepEqual(got, []string{"Q1B=Jawaban 1 B", "Q1A=Jawaban 1 A"}) {
		t.Errorf("qca1 = %v", got)
	}
	if got := optionTexts(opts["qca10"]); !reflect.DeepEqual(got, []string{"Q10B=Jawaban 10 B", "Q10A=Jawaban 10 A"}) {
		t.Errorf("qca10 = %v", got)
	}
}

// AC-FR-APP-020-02, -05, FR-APP-020 Validation, DEC-151: unanswered is
// ERR-010 "Pertanyaan <n> wajib diisi" top to bottom; a code outside BMV
// QCA<n> (another question's, or Inactive) is ERR-037; a refused Next
// stores nothing; the form takes exactly the ten answers (BR-UI-010).
func TestQCADataValidation(t *testing.T) {
	h, c, caseID, id := qcaData(t)
	var fields []string
	for _, e := range fieldErrors(t, submitQCA(t, c, caseID, id, "Collateral Data", make([]string, 10))) {
		fields = append(fields, e.Field+" "+e.Code)
	}
	if len(fields) != 10 || fields[0] != "Pertanyaan 1 ERR-010" || fields[9] != "Pertanyaan 10 ERR-010" {
		t.Fatalf("errors = %v", fields)
	}
	third := allAnswers()
	third[2] = ""
	if got := fieldErrors(t, submitQCA(t, c, caseID, id, "Collateral Data", third)); !reflect.DeepEqual(got, []fieldError{{"form", "Pertanyaan 3", "ERR-010", "Pertanyaan 3 wajib diisi"}}) {
		t.Fatalf("AC-02 = %v", got)
	}
	for name, code := range map[string]string{"another question's": "Q1A", "inactive": "Q2X"} {
		a := allAnswers()
		a[1] = code
		if got := fieldErrors(t, submitQCA(t, c, caseID, id, "Collateral Data", a)); !reflect.DeepEqual(got, []fieldError{{"form", "Pertanyaan 2", "ERR-037", "Harap masukkan Pertanyaan 2 yang valid."}}) {
			t.Errorf("%s = %v", name, got)
		}
	}
	if got := qcaRows(t, h, caseID); len(got) != 0 {
		t.Fatalf("refused Next stored %v", got)
	}
	if r := submitQCA(t, c, caseID, id, "Collateral Data", allAnswers()[:9]); r.Status != http.StatusBadRequest {
		t.Fatalf("nine answers = %d", r.Status)
	}
	if r := c.Do(t, http.MethodPost, prospectPath(caseID, id, "qca", "Collateral Data"), map[string]any{"answers": allAnswers(), "notes": "x"}); r.Status != http.StatusBadRequest {
		t.Fatalf("another field = %d", r.Status)
	}
}

// AC-FR-APP-020-04, -06, DB-RULE-126, BR-UI-012: Next stores ten rows and
// opens Collateral Data; reopened, the answers come back, one Inactive since
// still shown and accepted as the saved answer.
func TestQCADataNext(t *testing.T) {
	h, c, caseID, id := qcaData(t)
	a := allAnswers()
	a[4] = "Q5B"
	movedTo(t, h, caseID, submitQCA(t, c, caseID, id, "Collateral Data", a), "Collateral Data")
	if !processStarted(t, h, caseID, "Collateral Data") {
		t.Fatal("process Collateral Data not started")
	}
	want := []string{"1|Q1A", "2|Q2A", "3|Q3A", "4|Q4A", "5|Q5B", "6|Q6A", "7|Q7A", "8|Q8A", "9|Q9A", "10|Q10A"}
	if got := qcaRows(t, h, caseID); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %v", got)
	}
	exec(t, h, `UPDATE fos.business_mapping_value SET status = 'Inactive' WHERE type = 'QCA1' AND code = 'Q1A'`)
	exec(t, h, `UPDATE fos.assignments SET assignment_status = 'sudah dibuka', completed_at = NULL WHERE assignment_id = $1`, id)
	p := pipelinePage(t, c, caseID, id)
	if got := p["values"].(map[string]any)["answers"].([]any); got[0] != "Q1A" || got[4] != "Q5B" {
		t.Fatalf("reopened = %v", got)
	}
	if got := optionTexts(p["options"].(map[string]any)["qca1"]); !reflect.DeepEqual(got, []string{"Q1B=Jawaban 1 B", "Q1A=Jawaban 1 A"}) {
		t.Fatalf("qca1 with the saved Inactive answer = %v", got)
	}
	movedTo(t, h, caseID, submitQCA(t, c, caseID, id, "Collateral Data", a), "Collateral Data")
}

// Agreed comments 1 and 2: Save stores the answered questions only and
// stays, codes still checked; Back to Previous Step validates all ten,
// saves and opens Data Permohonan Fasilitas; a step is no target of itself.
func TestQCADataSaveAndMoves(t *testing.T) {
	h, c, caseID, id := qcaData(t)
	partial := make([]string, 10)
	partial[0], partial[4] = "Q1A", "Q5B"
	if r := submitQCA(t, c, caseID, id, "save", partial); r.Status != http.StatusOK {
		t.Fatalf("save = %d %s", r.Status, r.Raw)
	}
	if got := qcaRows(t, h, caseID); !reflect.DeepEqual(got, []string{"1|Q1A", "5|Q5B"}) || len(assignmentsNamed(t, h, caseID, "Collateral Data")) != 0 {
		t.Fatalf("after save %v", got)
	}
	bad := make([]string, 10)
	bad[3] = "nope"
	if got := fieldErrors(t, submitQCA(t, c, caseID, id, "save", bad)); len(got) != 1 || got[0].Field != "Pertanyaan 4" || got[0].Code != "ERR-037" {
		t.Fatalf("save codes = %v", got)
	}
	one := make([]string, 10)
	one[9] = "Q10B"
	if r := submitQCA(t, c, caseID, id, "save", one); r.Status != http.StatusOK {
		t.Fatalf("save = %d", r.Status)
	}
	if got := qcaRows(t, h, caseID); !reflect.DeepEqual(got, []string{"10|Q10B"}) {
		t.Fatalf("save did not replace: %v", got)
	}
	if r := submitQCA(t, c, caseID, id, "QCA Data", allAnswers()); r.Status != http.StatusBadRequest {
		t.Fatalf("to itself = %d", r.Status)
	}
	if got := fieldErrors(t, submitQCA(t, c, caseID, id, "Data Permohonan Fasilitas", one)); len(got) != 9 {
		t.Fatalf("back with one answer = %v", got)
	}
	movedTo(t, h, caseID, submitQCA(t, c, caseID, id, "Data Permohonan Fasilitas", allAnswers()), "Data Permohonan Fasilitas")
	if got := qcaRows(t, h, caseID); len(got) != 10 {
		t.Fatalf("back did not save: %v", got)
	}
}
