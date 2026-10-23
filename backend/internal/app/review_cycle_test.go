package app_test

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

// AC-FR-APP-010-01, -02: Keputusan and Notes are required and Notes is at
// most 1000 characters, CRLF counting as one; a refused Submit stores nothing
// and keeps the assignment.
func TestPreScreeningReviewValidation(t *testing.T) {
	h, _, tl, caseID, id := toReview(t)
	before := assignments(t, h, caseID)

	r := review(t, tl, caseID, id, "", "  ")
	if got := createErrors(t, r); !reflect.DeepEqual(got, []string{"ERR-010 form/decision", "ERR-010 form/notes"}) {
		t.Fatalf("errors = %v", got)
	}
	if es := createFieldErrors(t, r); es[0].Message != "Keputusan wajib diisi" || es[1].Message != "Notes wajib diisi" {
		t.Fatalf("messages = %+v", es)
	}
	r = review(t, tl, caseID, id, "SESUAI", strings.Repeat("é", 1001))
	if es := createFieldErrors(t, r); len(es) != 1 || es[0].Code != "ERR-030" || es[0].Message != "Notes maksimal 1000 karakter" {
		t.Fatalf("1001 characters = %s", r.Raw)
	}
	if r := review(t, tl, caseID, id, "TUNDA", "x"); r.Status != http.StatusBadRequest {
		t.Fatalf("code not offered = %d", r.Status)
	}
	if len(decisions(t, h, caseID)) != 0 || !reflect.DeepEqual(assignments(t, h, caseID), before) {
		t.Fatal("a refused Submit changed the case")
	}

	notes := strings.Repeat("a", 499) + "\r\n" + strings.Repeat("b", 500)
	if r := review(t, tl, caseID, id, "TIDAKSESUAI", notes); r.Status != http.StatusNoContent {
		t.Fatalf("1000 characters = %d %s", r.Status, r.Raw)
	}
	if got := decisions(t, h, caseID)[0].Notes; got != strings.Repeat("a", 499)+"\n"+strings.Repeat("b", 500) {
		t.Fatalf("stored notes = %q", got)
	}
}

// Only the assigned Team Lead submits, once.
func TestOnlyTheTeamLeadReviews(t *testing.T) {
	_, ao, tl, caseID, id := toReview(t)
	if r := review(t, ao, caseID, id, "SESUAI", "x"); r.Status != http.StatusForbidden {
		t.Fatalf("Branch Officer = %d", r.Status)
	}
	if r := review(t, tl, caseID, id, "SESUAI", "x"); r.Status != http.StatusNoContent {
		t.Fatalf("review = %d", r.Status)
	}
	if r := review(t, tl, caseID, id, "SESUAI", "x"); r.Status != http.StatusConflict {
		t.Fatalf("second review = %d", r.Status)
	}
	if r := tl.OpenAssignment(t, caseID, id); field(t, r, "page", "editable") != false {
		t.Fatalf("completed review page = %s", r.Raw)
	}
}

// AC-FR-APP-010-05, -06, -07, AC-FR-APP-009-03: after Tidak Sesuai the
// Branch Officer resubmits and creates again, which gives the Team Lead a new
// Pre Screening with the history oldest first; the Notes banner shows the
// latest decision; case_status stays New through the cycle.
func TestReviewCycleKeepsHistory(t *testing.T) {
	h, ao, tl, caseID, id := toReview(t)
	h.Clock.Set(time.Date(2026, 10, 13, 2, 3, 0, 0, time.UTC)) // 09:03 at GMT+7
	tl = relogin(t, h, "TL01")
	if r := review(t, tl, caseID, id, "TIDAKSESUAI", "Terdapat typo pada nama."); r.Status != http.StatusNoContent {
		t.Fatalf("first review = %d %s", r.Status, r.Raw)
	}

	// The Branch Officer opens the returned Pre-Screening Submission, fixes it and creates again.
	h.Clock.Advance(time.Hour)
	ao = relogin(t, h, "AO001")
	returned := assignments(t, h, caseID)[len(assignments(t, h, caseID))-1].ID
	ao.OpenAssignment(t, caseID, returned)
	fixed := person{"3171011234567890", "Budi Santosa", yearsAgo(h, 30, 0)}
	next := ao.Do(t, http.MethodPost, submitPath(caseID, returned, "submit"), preScreening("Perorangan", fixed, person{}))
	if next.Status != http.StatusOK {
		t.Fatalf("resubmit = %d %s", next.Status, next.Raw)
	}
	duu := next.Body["assignmentId"]
	documentList(t, ao.OpenAssignment(t, caseID, duu))
	upload(t, ao, caseID, duu, "KK", "KK baru", "kk.pdf", []byte("new"))
	created := createUnderlying(t, ao, caseID, duu, with(validSLIK(), "address", "Jl. Baru"))
	if created.Status != http.StatusOK {
		t.Fatalf("create again = %d %s", created.Status, created.Raw)
	}
	if got := caseStatus(t, h, caseID); got != "New" {
		t.Fatalf("case_status = %s", got)
	}

	second := created.Body["assignmentId"]
	tl = relogin(t, h, "TL01")
	r := tl.OpenAssignment(t, caseID, second)
	if r.Body["name"] != "Pre Screening" || field(t, r, "page", "editable") != true {
		t.Fatalf("second Pre Screening = %s", r.Raw)
	}
	hist := field(t, r, "page", "history").([]any)
	first := hist[0].(map[string]any)
	if len(hist) != 1 || first["decidedBy"] != "Tini Lestari" || first["submittedAt"] != "13/10/26 9:03" ||
		first["decision"] != "Tidak Sesuai" || first["notes"] != "Terdapat typo pada nama." {
		t.Fatalf("history = %v", hist)
	}

	h.Clock.Advance(time.Hour)
	tl = relogin(t, h, "TL01")
	review(t, tl, caseID, second, "TIDAKSESUAI", "Foto KTP buram.")
	ao = relogin(t, h, "AO001")
	third := assignments(t, h, caseID)[len(assignments(t, h, caseID))-1].ID
	p := ao.OpenAssignment(t, caseID, third)
	if banner, _ := field(t, p, "page", "notesBanner").(map[string]any); banner["notes"] != "Foto KTP buram." {
		t.Fatalf("banner = %s", p.Raw)
	}
	r = relogin(t, h, "TL01").OpenAssignment(t, caseID, second)
	var order []string
	for _, e := range field(t, r, "page", "history").([]any) {
		order = append(order, e.(map[string]any)["notes"].(string))
	}
	if !reflect.DeepEqual(order, []string{"Terdapat typo pada nama.", "Foto KTP buram."}) {
		t.Fatalf("history order = %v", order)
	}
}

// The Notes banner is absent before the first decision.
func TestNoNotesBannerBeforeADecision(t *testing.T) {
	h := caseHarness(t)
	c, caseID, id := newCase(t, h)
	if r := c.OpenAssignment(t, caseID, id); field(t, r, "page", "notesBanner") != nil {
		t.Fatalf("banner = %s", r.Raw)
	}
}

// Review is only on Pre Screening; a Sesuai whose Branch Officer is not in
// the operator data records nothing.
func TestReviewRefusesOtherAssignmentsAndUnknownDeciders(t *testing.T) {
	h, c, caseID, duu := readyToCreate(t)
	if r := review(t, c, caseID, duu, "SESUAI", "x"); r.Status != http.StatusBadRequest {
		t.Fatalf("review on Document Underlying Upload = %d", r.Status)
	}
	r := createUnderlying(t, c, caseID, duu, validSLIK())
	tl := relogin(t, h, "TL01")
	exec(t, h, `UPDATE fos.work SET branch_officer = 'GONE' WHERE case_id = $1`, caseID)
	if r := review(t, tl, caseID, r.Body["assignmentId"], "SESUAI", "x"); r.Status != http.StatusInternalServerError {
		t.Fatalf("unknown Branch Officer = %d", r.Status)
	}
	if len(decisions(t, h, caseID)) != 0 || caseStatus(t, h, caseID) != "New" {
		t.Fatal("a failed review changed the case")
	}
}
