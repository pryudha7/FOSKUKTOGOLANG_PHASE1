package app_test

import (
	"net/http"
	"reflect"
	"testing"

	"fos/internal/apptest"
)

// checksWith takes a case to the Team Lead, fakes Dukcapil and DHN, and
// submits Sesuai.
func checksWith(t *testing.T, dukcapil []apptest.FakeReply, dhn []apptest.FakeReply, before func(h *apptest.Harness, caseID string)) (*apptest.Harness, *apptest.FakeDukcapil, *apptest.FakeAPI, string) {
	t.Helper()
	h, _, tl, caseID, id := toReview(t)
	fd := apptest.StartFakeDukcapil(t, dukcapil...)
	h.UseDukcapil(t, fd)
	fh := apptest.StartFakeAPI(t, dhn...)
	h.UseDHN(t, fh)
	if before != nil {
		before(h, caseID)
	}
	if r := review(t, tl, caseID, id, "SESUAI", "Data lengkap."); r.Status != http.StatusNoContent {
		t.Fatalf("sesuai = %d %s", r.Status, r.Raw)
	}
	return h, fd, fh, caseID
}

func apiNames(logs []apiLogRow) []string {
	var out []string
	for _, l := range logs {
		out = append(out, l.Name+" "+l.Status)
	}
	return out
}

// AC-FR-PSAPI-006-01, -03, AC-FR-PSAPI-007-02: after Dukcapil, DHN is a GET
// on the customer's NIK with x-api-key, logged with the NIK as request; even
// DN-01 passes; the Rejected List finds nothing and logs nothing; the run
// then waits on SLIK (ticket 11).
func TestDHNAndRejectedListPass(t *testing.T) {
	h, _, dhn, caseID := checksWith(t, []apptest.FakeReply{apptest.DukcapilMatch}, []apptest.FakeReply{apptest.DHNFound}, nil)

	eventually(t, "the run to reach SLIK", func() bool { return run(t, h, caseID).Step == "SLIK" })
	if got := run(t, h, caseID); got.State != "RUNNING" {
		t.Fatalf("run = %+v", got)
	}
	calls := dhn.Calls()
	if len(calls) != 1 || calls[0].Method != http.MethodGet || calls[0].Path != "/v1/customer/dhn/check/3171011234567890" ||
		calls[0].Header.Get("x-api-key") != apptest.DHNAPIKey || calls[0].Header.Get("Accept") != "application/json" || calls[0].Body != "" {
		t.Fatalf("DHN calls = %+v", calls)
	}
	logs := apiLogs(t, h, caseID)
	if got := apiNames(logs); !reflect.DeepEqual(got, []string{"Dukcapil Success", "DHN Success"}) {
		t.Fatalf("logs = %v", got)
	}
	d := logs[1]
	if d.Request != "3171011234567890" || d.Method != "GET" || d.Response != apptest.DHNFound.Body {
		t.Fatalf("DHN log = %+v", d)
	}
	if got := d.RequestHeaders["X-Api-Key"]; len(got) != 1 || got[0] != "***" {
		t.Fatalf("DHN request headers = %v", d.RequestHeaders)
	}
	if caseStatus(t, h, caseID) != "Pending-Prescreening" {
		t.Fatalf("case_status = %s", caseStatus(t, h, caseID))
	}
}
