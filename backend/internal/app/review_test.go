package app_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"fos/internal/apptest"
)

// toReview takes a Perorangan case through Create and returns the Branch
// Officer's client, the Team Lead's client, the case and the Pre Screening
// assignment.
func toReview(t *testing.T) (*apptest.Harness, *apptest.Client, *apptest.Client, string, any) {
	t.Helper()
	h, c, caseID, id := readyToCreate(t)
	r := createUnderlying(t, c, caseID, id, validSLIK())
	if r.Status != http.StatusOK {
		t.Fatalf("create = %d %s", r.Status, r.Raw)
	}
	// The Team Lead opens Pre Screening before deciding, as on UI-017.
	tl := relogin(t, h, "TL01")
	if p := tl.OpenAssignment(t, caseID, r.Body["assignmentId"]); p.Status != http.StatusOK {
		t.Fatalf("open Pre Screening = %d %s", p.Status, p.Raw)
	}
	return h, c, tl, caseID, r.Body["assignmentId"]
}

func review(t *testing.T, c *apptest.Client, caseID string, id any, code, notes string) apptest.Response {
	t.Helper()
	return c.Do(t, http.MethodPost, submitPath(caseID, id, "review"), map[string]string{"decisionCode": code, "notes": notes})
}

type decisionRow struct{ Type, Code, Notes, By, Role string }

func decisions(t *testing.T, h *apptest.Harness, caseID string) []decisionRow {
	t.Helper()
	rows, err := h.DB.Query(context.Background(), `
		SELECT recommendation_type, COALESCE(decision_code, ''), notes, decided_by, decided_by_role_code
		FROM fos.case_pre_screening_decision WHERE case_id = $1 ORDER BY decision_id`, caseID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []decisionRow
	for rows.Next() {
		var d decisionRow
		if err := rows.Scan(&d.Type, &d.Code, &d.Notes, &d.By, &d.Role); err != nil {
			t.Fatal(err)
		}
		out = append(out, d)
	}
	return out
}

func caseStatus(t *testing.T, h *apptest.Harness, caseID string) string {
	t.Helper()
	return deref(storedSLIK(t, h, caseID).Status)
}

// AC-FR-APP-010-08: Keputusan lists FAC_DECISION's Active rows; no history
// shows before the first decision.
func TestPreScreeningReviewPage(t *testing.T) {
	h, _, tl, caseID, id := toReview(t)
	h.SeedBMV(t, "FAC_DECISION", "TUNDA", "Tunda", 0, "Inactive")

	r := tl.OpenAssignment(t, caseID, id)
	if r.Status != http.StatusOK || r.Body["name"] != "Pre Screening" || field(t, r, "page", "editable") != true {
		t.Fatalf("open = %d %s", r.Status, r.Raw)
	}
	var got []string
	for _, o := range field(t, r, "page", "decisionOptions").([]any) {
		m := o.(map[string]any)
		got = append(got, m["code"].(string)+"="+m["description"].(string))
	}
	if !reflect.DeepEqual(got, []string{"SESUAI=Sesuai", "TIDAKSESUAI=Tidak Sesuai"}) {
		t.Fatalf("options = %v", got)
	}
	if hist, _ := field(t, r, "page", "history").([]any); len(hist) != 0 {
		t.Fatalf("history = %v", hist)
	}
}

// AC-FR-APP-010-03, -10, -04: Tidak Sesuai records the Team Lead, completes
// Pre Screening and gives the Branch Officer a new Pre-Screening Submission,
// whose page shows the Notes banner; case_status does not change.
func TestTidakSesuaiReturnsTheCaseToTheBranchOfficer(t *testing.T) {
	h, ao, tl, caseID, id := toReview(t)

	r := review(t, tl, caseID, id, "TIDAKSESUAI", "Terdapat typo pada nama.")
	if r.Status != http.StatusNoContent {
		t.Fatalf("review = %d %s", r.Status, r.Raw)
	}
	if got := decisions(t, h, caseID); !reflect.DeepEqual(got, []decisionRow{{"Pre Screening", "TIDAKSESUAI", "Terdapat typo pada nama.", "TL01", "TL"}}) {
		t.Fatalf("decisions = %+v", got)
	}
	as := assignments(t, h, caseID)
	prev, last := as[len(as)-2], as[len(as)-1]
	if prev.Name != "Pre Screening" || prev.Status != "sudah selesai dikerjakan" || prev.Completed == nil ||
		last.Name != "Pre-Screening Submission" || last.Status != "baru" || last.Holder != "AO001" {
		t.Fatalf("assignments = %+v", as)
	}
	if got := caseStatus(t, h, caseID); got != "New" {
		t.Fatalf("case_status = %s", got)
	}

	p := ao.OpenAssignment(t, caseID, last.ID)
	banner, _ := field(t, p, "page", "notesBanner").(map[string]any)
	if banner["decision"] != "Tidak Sesuai" || banner["notes"] != "Terdapat typo pada nama." || field(t, p, "page", "editable") != true ||
		field(t, p, "page", "customerType") != "Perorangan" {
		t.Fatalf("returned page = %s", p.Raw)
	}
}

// AC-FR-APP-010-09, FR-APP-010 step 5: Sesuai records the case's Branch
// Officer with their role and sets case_status Pending-Prescreening; nothing
// returns to the Branch Officer.
func TestSesuaiMovesTheCaseToPendingPrescreening(t *testing.T) {
	h, _, tl, caseID, id := toReview(t)

	if r := review(t, tl, caseID, id, "SESUAI", "Data lengkap."); r.Status != http.StatusNoContent {
		t.Fatalf("review = %d %s", r.Status, r.Raw)
	}
	if got := decisions(t, h, caseID); !reflect.DeepEqual(got, []decisionRow{{"Pre Screening", "SESUAI", "Data lengkap.", "AO001", "AO"}}) {
		t.Fatalf("decisions = %+v", got)
	}
	if got := caseStatus(t, h, caseID); got != "Pending-Prescreening" {
		t.Fatalf("case_status = %s", got)
	}
	as := assignments(t, h, caseID)
	if last := as[len(as)-1]; last.Name != "Pre Screening" || last.Status != "sudah selesai dikerjakan" {
		t.Fatalf("assignments = %+v", as)
	}
}
