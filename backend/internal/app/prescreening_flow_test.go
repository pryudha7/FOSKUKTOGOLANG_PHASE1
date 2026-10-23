package app_test

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"fos/internal/apptest"
	"fos/internal/config"
)

func statuses(logs []apiLogRow) []string {
	var out []string
	for _, l := range logs {
		out = append(out, l.Status)
	}
	return out
}

func currentStage(t *testing.T, h *apptest.Harness, caseID string) string {
	t.Helper()
	return deref(work(t, h, caseID).Stage)
}

// AC-FR-PSAPI-001-02: a birth-date mismatch rejects at once: the case goes
// to Rejected and no later check runs.
func TestDukcapilMismatchRejectsTheCase(t *testing.T) {
	h, fake, tl, caseID := sesuaiWith(t, apptest.DukcapilMismatch)

	eventually(t, "the run to stop", func() bool { return run(t, h, caseID).State != "RUNNING" })
	if got := run(t, h, caseID); got.State != "STOPPED_REJECTED" || got.Step != "Dukcapil" {
		t.Fatalf("run = %+v", got)
	}
	if got := statuses(apiLogs(t, h, caseID)); !reflect.DeepEqual(got, []string{"Rejected"}) {
		t.Fatalf("logs = %v", got)
	}
	if caseStatus(t, h, caseID) != "ResolvedRejected-Prescreening" || currentStage(t, h, caseID) != "Rejected" || len(fake.Requests()) != 1 {
		t.Fatalf("case = %s %s, calls %d", caseStatus(t, h, caseID), currentStage(t, h, caseID), len(fake.Requests()))
	}
	// The Stage Bar shows Rejected right after the stage it was entered from.
	names, current, _ := stageNames(t, tl.Case(t, caseID))
	if current != "Rejected" || len(names) < 2 || names[0] != "Financing Submission" || names[1] != "Rejected" {
		t.Fatalf("stage bar = %v, current %s", names, current)
	}
}

// AC-FR-PSAPI-001-04: "data not found" is a Success that passes.
func TestDukcapilNotFoundPasses(t *testing.T) {
	h, _, _, caseID := sesuaiWith(t, apptest.DukcapilNotFound)
	eventually(t, "the run to reach DHN", func() bool { return run(t, h, caseID).Step == "DHN" })
	if got := statuses(apiLogs(t, h, caseID)); !reflect.DeepEqual(got, []string{"Success"}) {
		t.Fatalf("logs = %v", got)
	}
}

// AC-FR-PSAPI-001-07, -12: Fail is retried; a 401 without body logs the
// SoT message; Fail, Fail, Success passes without the Team Lead.
func TestDukcapilFailIsRetried(t *testing.T) {
	h, fake, _, caseID := sesuaiWith(t, apptest.DukcapilReply{Status: 401}, apptest.DukcapilDown, apptest.DukcapilMatch)

	eventually(t, "the run to reach DHN", func() bool { return run(t, h, caseID).Step == "DHN" })
	logs := apiLogs(t, h, caseID)
	if got := statuses(logs); !reflect.DeepEqual(got, []string{"Fail", "Fail", "Success"}) || len(fake.Requests()) != 3 {
		t.Fatalf("logs = %v, calls %d", got, len(fake.Requests()))
	}
	if logs[0].Response != "the server responded with an HTTP 401 code, indicating an authorization problem." ||
		logs[1].Response != "the server responded with an HTTP 503 code." || logs[0].Attempt != 1 || logs[2].Attempt != 3 {
		t.Fatalf("logs = %+v", logs)
	}
	if caseStatus(t, h, caseID) != "Pending-Prescreening" {
		t.Fatalf("case_status = %s", caseStatus(t, h, caseID))
	}
}

// AC-FR-PSAPI-001-06, -08, -14: four timeouts return the case to the Team
// Lead on a Retry view listing the failed attempts; Submit there sets
// Pending-Prescreening and calls Dukcapil again with a fresh cycle.
func TestDukcapilFailureReturnsToTheTeamLeadForRetry(t *testing.T) {
	h, fake, tl, caseID := sesuaiWith(t, apptest.DukcapilHang)

	eventually(t, "the run to fail", func() bool { return run(t, h, caseID).State == "STOPPED_FAILED" })
	logs := apiLogs(t, h, caseID)
	if got := statuses(logs); !reflect.DeepEqual(got, []string{"Fail", "Fail", "Fail", "Fail"}) {
		t.Fatalf("logs = %v", got)
	}
	if !strings.HasPrefix(logs[0].Response, "Connect to 127.0.0.1:") || !strings.HasSuffix(logs[0].Response, "timed out after 300 ms") || logs[0].HTTPStatus != nil {
		t.Fatalf("timeout log = %+v", logs[0])
	}
	if got := caseStatus(t, h, caseID); got != "Failed-PrescreeningConnection" {
		t.Fatalf("case_status = %s", got)
	}
	as := assignments(t, h, caseID)
	retry := as[len(as)-1]
	if retry.Name != "Pre Screening" || retry.Status != "baru" || retry.Holder != "TL01" {
		t.Fatalf("assignments = %+v", as)
	}

	tl = relogin(t, h, "TL01")
	p := tl.OpenAssignment(t, caseID, retry.ID)
	rows, _ := field(t, p, "page", "retry", "logs").([]any)
	if len(rows) != 4 || field(t, p, "page", "editable") != true {
		t.Fatalf("retry page = %s", p.Raw)
	}
	first := rows[0].(map[string]any)
	if first["name"] != "Dukcapil" || first["status"] != "Fail" || first["request"] != logs[0].Request || first["response"] != logs[0].Response {
		t.Fatalf("retry row = %v", first)
	}
	if r := review(t, tl, caseID, retry.ID, "SESUAI", "x"); r.Status != http.StatusBadRequest {
		t.Fatalf("a decision on the Retry view = %d", r.Status)
	}

	fake.Script(apptest.DukcapilDown, apptest.DukcapilMatch)
	if r := tl.Do(t, http.MethodPost, submitPath(caseID, retry.ID, "retry"), nil); r.Status != http.StatusNoContent {
		t.Fatalf("retry = %d %s", r.Status, r.Raw)
	}
	if got := caseStatus(t, h, caseID); got != "Pending-Prescreening" {
		t.Fatalf("case_status after retry = %s", got)
	}
	eventually(t, "the run to reach DHN", func() bool { return run(t, h, caseID).Step == "DHN" })
	logs = apiLogs(t, h, caseID)
	if got := statuses(logs); !reflect.DeepEqual(got, []string{"Fail", "Fail", "Fail", "Fail", "Fail", "Success"}) || logs[4].Attempt != 1 {
		t.Fatalf("logs after retry = %v", logs)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.case_pre_screening_decision WHERE case_id = $1`, caseID); n != 1 {
		t.Fatalf("retry added a Recommendation: %d", n)
	}
	if r := tl.Do(t, http.MethodPost, submitPath(caseID, retry.ID, "retry"), nil); r.Status != http.StatusConflict {
		t.Fatalf("second retry = %d", r.Status)
	}
}

// Retry is only for a case that failed its checks.
func TestRetryNeedsAFailedRun(t *testing.T) {
	_, _, tl, caseID, id := toReview(t)
	if r := tl.Do(t, http.MethodPost, submitPath(caseID, id, "retry"), nil); r.Status != http.StatusBadRequest {
		t.Fatalf("retry without failure = %d", r.Status)
	}
}

// AC-FR-PSAPI-001-13: with simulation on, nothing is called and the
// configured response is logged as Success.
func TestDukcapilSimulation(t *testing.T) {
	h, _, tl, caseID, id := toReview(t)
	fake := apptest.StartFakeDukcapil(t, apptest.DukcapilMismatch)
	h.UseDukcapil(t, fake)
	exec(t, h, `UPDATE fos.dynamic_system_settings SET value = 'true' WHERE purpose = 'api/dukcapil/simulate'`)
	review(t, tl, caseID, id, "SESUAI", "x")

	eventually(t, "the run to reach DHN", func() bool { return run(t, h, caseID).Step == "DHN" })
	logs := apiLogs(t, h, caseID)
	if len(fake.Requests()) != 0 || len(logs) != 1 || logs[0].Status != "Success" || !strings.Contains(logs[0].Response, `"TGL_LHR":"Sesuai"`) {
		t.Fatalf("calls %d, logs %+v", len(fake.Requests()), logs)
	}
}

// AC-FR-PSAPI-001-05: Dukcapil does not apply to a Badan Usaha case.
func TestDukcapilSkipsBusinessEntities(t *testing.T) {
	h, _, tl, caseID, id := toReview(t)
	fake := apptest.StartFakeDukcapil(t, apptest.DukcapilMatch)
	h.UseDukcapil(t, fake)
	exec(t, h, `UPDATE fos.work SET customer_type = 'Badan Usaha' WHERE case_id = $1`, caseID)
	review(t, tl, caseID, id, "SESUAI", "x")
	eventually(t, "the run to reach DHN", func() bool { return run(t, h, caseID).Step == "DHN" })
	if len(fake.Requests()) != 0 || len(apiLogs(t, h, caseID)) != 0 {
		t.Fatal("Dukcapil ran for a Badan Usaha case")
	}
}

// Agreed for ticket 09: a missing setting or API key stops the run broken
// without changing case_status or calling Dukcapil.
func TestMissingDukcapilConfigurationStopsTheRunBroken(t *testing.T) {
	h, _, tl, caseID, id := toReview(t)
	fake := apptest.StartFakeDukcapil(t, apptest.DukcapilMatch)
	h.UseDukcapil(t, fake)
	t.Setenv("DUKCAPIL_APIKEY", "")
	logs := apptest.CaptureLogs(t)
	review(t, tl, caseID, id, "SESUAI", "x")
	eventually(t, "the run to stop", func() bool { return run(t, h, caseID).State != "RUNNING" })
	if got := run(t, h, caseID); got.State != "STOPPED_BROKEN" || caseStatus(t, h, caseID) != "Pending-Prescreening" || len(fake.Requests()) != 0 {
		t.Fatalf("run %+v, status %s", got, caseStatus(t, h, caseID))
	}
	if !strings.Contains(logs.String(), "DUKCAPIL_APIKEY") {
		t.Fatalf("not logged: %s", logs)
	}
}

// AC-FR-PSAPI-001-09, -11: a run whose executor died is taken over by
// exactly one other executor and resumes at its step.
func TestRecoverySweepResumesAnAbandonedRun(t *testing.T) {
	h, _, _, caseID, _ := toReview(t)
	fake := apptest.StartFakeDukcapil(t, apptest.DukcapilMatch)
	h.UseDukcapil(t, fake)
	exec(t, h, `UPDATE fos.work SET case_status = 'Pending-Prescreening' WHERE case_id = $1`, caseID)
	exec(t, h, `INSERT INTO fos.prescreening_runs (case_id, state, current_step, attempt, owner_id, heartbeat_at, started_at, updated_at)
		VALUES ($1, 'RUNNING', 'Dukcapil', 2, 'dead-executor', $2, $2, $2)`, caseID, time.Now().UTC().Add(-2*time.Minute))

	// Two more backends on the same database start and sweep.
	for i := 0; i < 2; i++ {
		apptest.New(t, func(c *config.Config) { c.DatabaseURL = h.Config.DatabaseURL })
	}
	eventually(t, "the run to reach DHN", func() bool { return run(t, h, caseID).Step == "DHN" })
	logs := apiLogs(t, h, caseID)
	if len(fake.Requests()) != 1 || len(logs) != 1 || logs[0].Attempt != 2 {
		t.Fatalf("calls %d, logs %+v", len(fake.Requests()), logs)
	}
	var owner string
	if err := h.DB.QueryRow(context.Background(), `SELECT owner_id FROM fos.prescreening_runs WHERE case_id = $1`, caseID).Scan(&owner); err != nil || owner == "dead-executor" {
		t.Fatalf("owner = %s %v", owner, err)
	}
}

// DB-RULE-046: a case has at most one active run.
func TestOneActiveRunPerCase(t *testing.T) {
	h, _, _, caseID, _ := toReview(t)
	insert := `INSERT INTO fos.prescreening_runs (case_id, state, current_step, attempt, heartbeat_at, started_at, updated_at)
		VALUES ($1, 'RUNNING', 'DHN', 1, now(), now(), now())`
	exec(t, h, insert, caseID)
	if _, err := h.DB.Exec(context.Background(), insert, caseID); err == nil {
		t.Fatal("a second active run was stored")
	}
}

// DB-RULE-047: an executor that lost its run while an attempt was in flight
// records nothing and stops, so the new owner's step is not done twice.
func TestAnExecutorThatLostItsRunRecordsNothing(t *testing.T) {
	h, fake, _, caseID := sesuaiWith(t, apptest.DukcapilHang)
	eventually(t, "the first call", func() bool { return len(fake.Requests()) == 1 })
	exec(t, h, `UPDATE fos.prescreening_runs SET owner_id = 'other-executor' WHERE case_id = $1`, caseID)

	time.Sleep(time.Second) // past the 300 ms timeout and the 20 ms retry gap
	if logs := apiLogs(t, h, caseID); len(logs) != 0 || len(fake.Requests()) != 1 {
		t.Fatalf("logs %+v, calls %d", logs, len(fake.Requests()))
	}
	if got := run(t, h, caseID); got.State != "RUNNING" || got.Attempt != 1 {
		t.Fatalf("run = %+v", got)
	}
}

// AC-FR-PSAPI-004-01: two calls of the same API give two rows, each with
// its long response kept whole.
func TestLongResponsesAreLoggedWhole(t *testing.T) {
	long := `{"NIK":"Sesuai (100)","NAMA_LGKP":"Sesuai (100)","TGL_LHR":"Sesuai","ALAMAT":"` + strings.Repeat("Jalan Panjang ", 20000) + `"}`
	h, _, _, caseID := sesuaiWith(t, apptest.DukcapilReply{Status: 500, Body: strings.Repeat("x", 300000)}, apptest.DukcapilReply{Status: 200, Body: long})
	eventually(t, "the run to reach DHN", func() bool { return run(t, h, caseID).Step == "DHN" })
	logs := apiLogs(t, h, caseID)
	if len(logs) != 2 || len(logs[0].Response) != 300000 || logs[1].Response != long {
		t.Fatalf("rows %d, lengths %d / %d", len(logs), len(logs[0].Response), len(logs[1].Response))
	}
}

// Dukcapil checks the Individual Customer only, never the Spouse.
func TestDukcapilNeverSendsTheSpouse(t *testing.T) {
	h, _, tl, caseID, id := toReview(t)
	exec(t, h, `INSERT INTO fos.customer (case_id, customer_type, id_number, full_name, birth_date) VALUES ($1, 'Pasangan', '3171019876543210', 'Siti Aminah', '1990-01-01')`, caseID)
	fake := apptest.StartFakeDukcapil(t, apptest.DukcapilMatch)
	h.UseDukcapil(t, fake)
	review(t, tl, caseID, id, "SESUAI", "x")
	eventually(t, "the run to reach DHN", func() bool { return run(t, h, caseID).Step == "DHN" })
	reqs := fake.Requests()
	if len(reqs) != 1 || strings.Contains(reqs[0].Body, "3171019876543210") || strings.Contains(reqs[0].Body, "Siti") {
		t.Fatalf("requests = %+v", reqs)
	}
}

// RESPONSE_CODE 13 passes whether it comes as text or as a number.
func TestDukcapilNotFoundAsNumberPasses(t *testing.T) {
	h, _, _, caseID := sesuaiWith(t, apptest.DukcapilReply{Status: 200, Body: `{"RESPONSE_CODE":13,"RESPONSE_DESC":"Data Tidak Ditemukan"}`})
	eventually(t, "the run to reach DHN", func() bool { return run(t, h, caseID).Step == "DHN" })
}

// Agreed for ticket 09: missing request data is a Fail of each attempt,
// so the case returns to the Team Lead; nothing is sent.
func TestMissingRequestDataFailsTheAttempts(t *testing.T) {
	h, _, tl, caseID, id := toReview(t)
	fake := apptest.StartFakeDukcapil(t, apptest.DukcapilMatch)
	h.UseDukcapil(t, fake)
	exec(t, h, `UPDATE fos.customer SET birth_date = NULL WHERE case_id = $1 AND customer_type = 'Nasabah'`, caseID)
	review(t, tl, caseID, id, "SESUAI", "x")
	eventually(t, "the run to fail", func() bool { return run(t, h, caseID).State == "STOPPED_FAILED" })
	logs := apiLogs(t, h, caseID)
	if len(fake.Requests()) != 0 || len(logs) != 4 || !strings.HasPrefix(logs[0].Response, "Dukcapil request data missing") {
		t.Fatalf("calls %d, logs %+v", len(fake.Requests()), logs)
	}
}

// DEC-060: the API key header is masked even when the setting omits it.
func TestDukcapilKeyIsAlwaysMasked(t *testing.T) {
	h, _, tl, caseID, id := toReview(t)
	fake := apptest.StartFakeDukcapil(t, apptest.DukcapilMatch)
	h.UseDukcapil(t, fake)
	exec(t, h, `UPDATE fos.dynamic_system_settings SET value = 'Cookie' WHERE purpose = 'api/log/masked_headers'`)
	review(t, tl, caseID, id, "SESUAI", "x")
	eventually(t, "the run to reach DHN", func() bool { return run(t, h, caseID).Step == "DHN" })
	if n := count(t, h, `SELECT count(*) FROM fos.connect_api_log WHERE case_id = $1 AND request_headers::text LIKE '%'||$2||'%'`, caseID, apptest.DukcapilAPIKey); n != 0 {
		t.Fatal("the API key was stored")
	}
}
