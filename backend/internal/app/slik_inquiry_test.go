package app_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"fos/internal/apptest"
)

// SLIKInquiry replies (FR-PSAPI-013).
var (
	inquiryNotFound = apptest.FakeReply{Status: 404, Body: `{"code":"4040","message":"data not found"}`}
	inquiryFound    = apptest.FakeReply{Status: 200, Body: `{"code":"1100","message":"success"}`}
)

// waitingAssignment is the case's SLIKWQFlowAct assignment.
func waitingAssignment(t *testing.T, h *apptest.Harness, caseID string) assignmentRow {
	t.Helper()
	for _, a := range assignments(t, h, caseID) {
		if a.Name == "SLIKWQFlowAct" {
			return a
		}
	}
	t.Fatal("no SLIKWQFlowAct assignment")
	return assignmentRow{}
}

func checkSLIK(t *testing.T, c *apptest.Client, caseID string, id any) apptest.Response {
	t.Helper()
	return c.Do(t, http.MethodPost, submitPath(caseID, id, "slik-status"), nil)
}

// statusMessages are the answer's subject: message pairs.
func statusMessages(r apptest.Response) []string {
	var out []string
	list, _ := r.Body["statuses"].([]any)
	for _, s := range list {
		m := s.(map[string]any)
		out = append(out, m["subject"].(string)+": "+m["message"].(string))
	}
	return out
}

// inquiryLogs are the case's SLIKInquiry rows as "status request".
func inquiryLogs(t *testing.T, h *apptest.Harness, caseID string) []string {
	t.Helper()
	var out []string
	for _, l := range apiLogs(t, h, caseID) {
		if l.Name == "SLIKInquiry" {
			out = append(out, l.Status+" "+l.Request)
		}
	}
	return out
}

// AC-FR-PSAPI-013-01, -02, -04: entering WAITING_SLIK gives the case's
// Branch Officer the SLIKWQFlowAct worklist item, due in the SLA's minutes;
// Check SLIK Status is one GET /v1/slik/status/{KodeReff} with the key, no
// retry, logged SLIKInquiry Fail with the code as request, the message
// shown; each press adds a row; nothing else changes.
func TestWaitingSLIKAndCheckStatus(t *testing.T) {
	h, fs, caseID := slikCase(t, false)
	kode := referenceCode(t, h, caseID, "CUSTOMER")
	a := waitingAssignment(t, h, caseID)
	if a.Status != "baru" || a.Holder != "AO001" {
		t.Fatalf("assignment = %+v", a)
	}
	ao := relogin(t, h, "AO001")
	p := ao.OpenAssignment(t, caseID, a.ID)
	// The SLA is 60 minutes; the stored deadline keeps microseconds, so a
	// fraction under 60 rounds down to 59.
	if due := field(t, p, "page", "dueMinutes"); field(t, p, "name") != "SLIKWQFlowAct" || (due != float64(60) && due != float64(59)) || field(t, p, "page", "editable") != true {
		t.Fatalf("page = %s", p.Raw)
	}

	fs.Script(inquiryNotFound)
	before := len(fs.Calls())
	r := checkSLIK(t, ao, caseID, a.ID)
	if r.Status != http.StatusOK || strings.Join(statusMessages(r), "|") != "CUSTOMER: data not found" {
		t.Fatalf("check = %d %s", r.Status, r.Raw)
	}
	calls := fs.Calls()[before:]
	if len(calls) != 1 || calls[0].Method != http.MethodGet || calls[0].Path != "/v1/slik/status/"+url.PathEscape(kode) || calls[0].Header.Get("x-api-key") != apptest.SLIKAPIKey {
		t.Fatalf("calls = %+v", calls)
	}
	checkSLIK(t, ao, caseID, a.ID)
	if got := inquiryLogs(t, h, caseID); strings.Join(got, "|") != "Fail "+kode+"|Fail "+kode {
		t.Fatalf("logs = %v", got)
	}
	if got := run(t, h, caseID); got.State != "WAITING_SLIK" || caseStatus(t, h, caseID) != "Pending-Prescreening" ||
		count(t, h, `SELECT count(*) FROM fos.slik_report WHERE case_id = $1 AND status = 'REQUESTED'`, caseID) != 1 {
		t.Fatalf("check changed the case: run %+v", got)
	}
	if r := checkSLIK(t, relogin(t, h, "TL01"), caseID, a.ID); r.Status != http.StatusForbidden {
		t.Fatalf("another operator = %d", r.Status)
	}
}

// AC-FR-PSAPI-013-03: with a Spouse, the Customer then the Spouse are asked
// one after the other; the Spouse's timeout does not stop the answer.
func TestCheckSLIKStatusAsksTheSpouseToo(t *testing.T) {
	h, fs, caseID := slikCase(t, true)
	kc, ks := referenceCode(t, h, caseID, "CUSTOMER"), referenceCode(t, h, caseID, "SPOUSE")
	a := waitingAssignment(t, h, caseID)
	ao := relogin(t, h, "AO001")
	ao.OpenAssignment(t, caseID, a.ID)
	fs.Script(inquiryFound, apptest.FakeReply{Hang: true})
	r := checkSLIK(t, ao, caseID, a.ID)
	got := statusMessages(r)
	if len(got) != 2 || got[0] != "CUSTOMER: success" || !strings.HasPrefix(got[1], "SPOUSE: Connect to 127.0.0.1:") || !strings.HasSuffix(got[1], "timed out after 300 ms") {
		t.Fatalf("statuses = %v", got)
	}
	if logs := inquiryLogs(t, h, caseID); strings.Join(logs, "|") != "Success "+kc+"|Fail "+ks {
		t.Fatalf("logs = %v", logs)
	}
}

// AC-FR-PSAPI-013-05, DEC-089: the result arriving completes the waiting
// assignment, also while it is open; a later check is refused and the page
// opens read-only. The timeout completes it too.
func TestWaitingSLIKCompletes(t *testing.T) {
	h, _, caseID := slikCase(t, false)
	kode := referenceCode(t, h, caseID, "CUSTOMER")
	a := waitingAssignment(t, h, caseID)
	ao := relogin(t, h, "AO001")
	ao.OpenAssignment(t, caseID, a.ID)
	if r := callPBS(t, h, kode, slikZip(t, kode, "3171011234567890", facility("1"))); r.Body["Code"] != "0" {
		t.Fatalf("result = %s", r.Raw)
	}
	if got := waitingAssignment(t, h, caseID); got.Status != "sudah selesai dikerjakan" {
		t.Fatalf("assignment = %+v", got)
	}
	if r := checkSLIK(t, ao, caseID, a.ID); r.Status != http.StatusConflict {
		t.Fatalf("check after completion = %d", r.Status)
	}
	if p := ao.OpenAssignment(t, caseID, a.ID); field(t, p, "page", "editable") != false {
		t.Fatalf("page = %s", p.Raw)
	}

	h2, fs2, other := slikCase(t, false)
	timedOut(t, h2, other)
	if got := waitingAssignment(t, h2, other); got.Status != "sudah selesai dikerjakan" {
		t.Fatalf("assignment after timeout = %+v", got)
	}
	// Check SLIK Status belongs to the waiting assignment only.
	as := assignments(t, h2, other)
	pss := as[len(as)-1]
	ao2 := relogin(t, h2, "AO001")
	ao2.OpenAssignment(t, other, pss.ID)
	before := len(fs2.Calls())
	if r := checkSLIK(t, ao2, other, pss.ID); r.Status != http.StatusBadRequest || len(fs2.Calls()) != before {
		t.Fatalf("check on %s = %d", pss.Name, r.Status)
	}
}

// A one-subject result while the other is awaited leaves the assignment
// open; a SLIK reject completes it (FR-PSAPI-013 Assignment 2).
func TestWaitingSLIKStaysOpenForTheOtherResult(t *testing.T) {
	h, _, caseID := slikCase(t, true)
	kc, ks := referenceCode(t, h, caseID, "CUSTOMER"), referenceCode(t, h, caseID, "SPOUSE")
	callPBS(t, h, kc, slikZip(t, kc, "3171011234567890", facility("1")))
	if got := waitingAssignment(t, h, caseID); got.Status == "sudah selesai dikerjakan" {
		t.Fatal("completed with a result still awaited")
	}
	callPBS(t, h, ks, slikZip(t, ks, spouseNIK, facility("2", repeat(7, month{"2", "10"})...)))
	if got := waitingAssignment(t, h, caseID); got.Status != "sudah selesai dikerjakan" || caseStatus(t, h, caseID) != "ResolvedRejected-Prescreening" {
		t.Fatalf("after reject: %+v", got)
	}
}

// Agreed for ticket 36: a simulated SLIKInquiry calls nothing and is judged
// by its configured answer.
func TestSLIKInquirySimulation(t *testing.T) {
	h, fs, caseID := slikCase(t, false)
	exec(t, h, `UPDATE fos.dynamic_system_settings SET value = 'true' WHERE purpose = 'api/slikinquiry/simulate'`)
	a := waitingAssignment(t, h, caseID)
	ao := relogin(t, h, "AO001")
	ao.OpenAssignment(t, caseID, a.ID)
	before := len(fs.Calls())
	r := checkSLIK(t, ao, caseID, a.ID)
	if strings.Join(statusMessages(r), "|") != "CUSTOMER: data not found" || len(fs.Calls()) != before ||
		!strings.HasPrefix(strings.Join(inquiryLogs(t, h, caseID), "|"), "Fail ") {
		t.Fatalf("simulated check = %s", r.Raw)
	}
}

// Review fix (agreed): a broken SLIKInquiry setting fails the press before
// any call or log row.
func TestCheckSLIKStatusWithABrokenSettingCallsNothing(t *testing.T) {
	h, fs, caseID := slikCase(t, true)
	exec(t, h, `UPDATE fos.dynamic_system_settings SET value = '/v1/slik/status' WHERE purpose = 'api/slikinquiry/path'`)
	a := waitingAssignment(t, h, caseID)
	ao := relogin(t, h, "AO001")
	ao.OpenAssignment(t, caseID, a.ID)
	before := len(fs.Calls())
	if r := checkSLIK(t, ao, caseID, a.ID); r.Status != http.StatusInternalServerError || len(fs.Calls()) != before || len(inquiryLogs(t, h, caseID)) != 0 {
		t.Fatalf("check = %d, calls %d, logs %v", r.Status, len(fs.Calls())-before, inquiryLogs(t, h, caseID))
	}
}
