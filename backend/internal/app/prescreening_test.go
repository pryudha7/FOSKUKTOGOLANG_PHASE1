package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"fos/internal/apptest"
)

// sesuaiWith takes a case to the Team Lead, points Dukcapil at a fake with
// the given replies and submits Sesuai.
func sesuaiWith(t *testing.T, replies ...apptest.DukcapilReply) (*apptest.Harness, *apptest.FakeDukcapil, *apptest.Client, string) {
	t.Helper()
	h, _, tl, caseID, id := toReview(t)
	fake := apptest.StartFakeDukcapil(t, replies...)
	h.UseDukcapil(t, fake)
	h.UseDHN(t, apptest.StartFakeAPI(t, apptest.DHNNotFound))
	if r := review(t, tl, caseID, id, "SESUAI", "Data lengkap."); r.Status != http.StatusNoContent {
		t.Fatalf("sesuai = %d %s", r.Status, r.Raw)
	}
	return h, fake, tl, caseID
}

type runRow struct {
	State, Step string
	Attempt     int
}

func run(t *testing.T, h *apptest.Harness, caseID string) runRow {
	t.Helper()
	var r runRow
	if err := h.DB.QueryRow(context.Background(), `
		SELECT state, current_step, attempt FROM fos.prescreening_runs WHERE case_id = $1 ORDER BY run_id DESC LIMIT 1`, caseID).
		Scan(&r.State, &r.Step, &r.Attempt); err != nil {
		t.Fatalf("run: %v", err)
	}
	return r
}

// eventually waits for the background run to reach a condition.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

type apiLogRow struct {
	Name, Status, Request, Response, Method, URL string
	HTTPStatus                                   *int
	RequestHeaders, ResponseHeaders              map[string][]string
	Attempt                                      int
	Duration                                     *int
}

func apiLogs(t *testing.T, h *apptest.Harness, caseID string) []apiLogRow {
	t.Helper()
	rows, err := h.DB.Query(context.Background(), `
		SELECT api_name, api_status, COALESCE(request, ''), COALESCE(response, ''), COALESCE(http_method, ''),
		       COALESCE(request_url, ''), http_status_code, request_headers, response_headers, attempt_number, duration_milliseconds
		FROM fos.connect_api_log WHERE case_id = $1 AND api_type = 'ConnectRest' ORDER BY submitted_at, xmin::text::bigint, ctid`, caseID) // the test clock stands still: order by writing transaction (xmin), then ctid within one; ctid alone is not insertion order
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []apiLogRow
	for rows.Next() {
		var (
			r        apiLogRow
			req, res []byte
		)
		if err := rows.Scan(&r.Name, &r.Status, &r.Request, &r.Response, &r.Method, &r.URL, &r.HTTPStatus, &req, &res, &r.Attempt, &r.Duration); err != nil {
			t.Fatal(err)
		}
		_ = json.Unmarshal(req, &r.RequestHeaders)
		_ = json.Unmarshal(res, &r.ResponseHeaders)
		out = append(out, r)
	}
	return out
}

// AC-FR-PSAPI-001-01, -03, AC-FR-PSAPI-004-01: Sesuai creates a RUNNING run
// and returns at once; Dukcapil is called for the customer with the key;
// the attempt is logged whole with secrets masked; a match passes and the
// run goes on to the next check.
func TestSesuaiRunsDukcapilInTheBackground(t *testing.T) {
	h, fake, _, caseID := sesuaiWith(t, apptest.DukcapilMatch)

	eventually(t, "Dukcapil to finish", func() bool { return run(t, h, caseID).Step != "Dukcapil" })
	// The run goes on past Dukcapil (later steps are not configured here).
	if got := run(t, h, caseID); got.State == "STOPPED_REJECTED" || got.State == "STOPPED_FAILED" {
		t.Fatalf("run = %+v", got)
	}
	if got := caseStatus(t, h, caseID); got != "Pending-Prescreening" {
		t.Fatalf("case_status = %s", got)
	}
	reqs := fake.Requests()
	if len(reqs) != 1 || reqs[0].Path != "/internal/kyc/dukcapil/v1.0/checkEktp" || reqs[0].APIKey != apptest.DukcapilAPIKey ||
		reqs[0].ContentType != "application/json" {
		t.Fatalf("requests = %+v", reqs)
	}
	var body map[string]string
	if err := json.Unmarshal([]byte(reqs[0].Body), &body); err != nil {
		t.Fatal(err)
	}
	if body["nik"] != "3171011234567890" || body["namaLengkap"] != "Budi Santoso" || body["tanggalLahir"] != dmy(yearsAgo(h, 30, 0)) {
		t.Fatalf("body = %s", reqs[0].Body)
	}

	logs := dukcapilLogs(t, h, caseID)
	if len(logs) != 1 {
		t.Fatalf("logs = %+v", logs)
	}
	l := logs[0]
	if l.Name != "Dukcapil" || l.Status != "Success" || l.Request != reqs[0].Body || l.Response != apptest.DukcapilMatch.Body ||
		l.Method != "POST" || l.URL != fake.URL+"/internal/kyc/dukcapil/v1.0/checkEktp" || l.HTTPStatus == nil || *l.HTTPStatus != 200 ||
		l.Attempt != 1 || l.Duration == nil {
		t.Fatalf("log = %+v", l)
	}
	if got := l.RequestHeaders["X-Gateway-Apikey"]; len(got) != 1 || got[0] != "***" {
		t.Fatalf("request headers = %v", l.RequestHeaders)
	}
	if got := l.ResponseHeaders["Set-Cookie"]; len(got) != 1 || got[0] != "***" {
		t.Fatalf("response headers = %v", l.ResponseHeaders)
	}
}

// dukcapilLogs is the case's Dukcapil rows of the Connect API Log.
func dukcapilLogs(t *testing.T, h *apptest.Harness, caseID string) []apiLogRow {
	t.Helper()
	var out []apiLogRow
	for _, l := range apiLogs(t, h, caseID) {
		if l.Name == "Dukcapil" {
			out = append(out, l)
		}
	}
	return out
}

// dmy turns yyyy-mm-dd into dd-MM-yyyy.
func dmy(iso string) string { return iso[8:10] + "-" + iso[5:7] + "-" + iso[0:4] }
