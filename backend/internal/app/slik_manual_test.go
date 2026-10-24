package app_test

import (
	"net/http"
	"strings"
	"testing"

	"fos/internal/apptest"
	"fos/internal/config"
)

const manualSLIKNotes = "Hasil SLIK belum diterima sistem lebih dari waktu tunggu. Harap lanjutkan untuk unggah hasil SLIK secara manual."

// sweepNow runs the 60-second check at once: another backend on the same
// database sweeps when it starts.
func sweepNow(t *testing.T, h *apptest.Harness) {
	t.Helper()
	apptest.New(t, func(c *config.Config) { c.DatabaseURL = h.Config.DatabaseURL })
}

// timedOut lets the case's SLIK wait pass its deadline and runs the check.
func timedOut(t *testing.T, h *apptest.Harness, caseID string) {
	t.Helper()
	exec(t, h, `UPDATE fos.prescreening_runs SET wait_deadline_at = '2000-01-01' WHERE case_id = $1`, caseID)
	sweepNow(t, h)
	eventually(t, "the SLIK timeout", func() bool { return run(t, h, caseID).State == "STOPPED_SLIK_TIMEOUT" })
}

// timeoutRows are the case's SLIK timeout log rows.
func timeoutRows(t *testing.T, h *apptest.Harness, caseID string) int {
	t.Helper()
	return count(t, h, `
		SELECT count(*) FROM fos.connect_api_log WHERE case_id = $1 AND api_type = 'Service Rest' AND api_name = 'SLIKPBSResponse'
		AND api_status = 'Fail' AND response = 'Timeout SLIK API No Response' AND request IS NULL`, caseID)
}

// systemInputManual counts the System's Input Manual Recommendations.
func systemInputManual(t *testing.T, h *apptest.Harness, caseID string) int {
	t.Helper()
	return count(t, h, `
		SELECT count(*) FROM fos.case_pre_screening_decision WHERE case_id = $1 AND recommendation_type = 'Pre Screening'
		AND decision_code = 'INPUTMANUAL' AND decided_by = 'System' AND decided_by_role_code = 'System' AND notes = $2`, caseID, manualSLIKNotes)
}

// AC-FR-PSAPI-011-01, -02, AC-FR-PSAPI-009-03: with the Spouse's result
// missing at the deadline, the check logs one timeout row and stops the run
// STOPPED_SLIK_TIMEOUT; the System's Input Manual Recommendation is added
// and the Branch Officer gets a new Pre-Screening Submission showing it; a
// later result is refused with Code 1; a second check changes nothing.
func TestSLIKTimeoutReturnsTheCaseForManualEntry(t *testing.T) {
	h, _, caseID := slikCase(t, true)
	if n := count(t, h, `SELECT count(*) FROM fos.prescreening_runs WHERE case_id = $1 AND wait_deadline_at = updated_at + interval '3600 seconds'`, caseID); n != 1 {
		t.Fatal("the wait is not the 1-hour SLA")
	}
	kc, ks := referenceCode(t, h, caseID, "CUSTOMER"), referenceCode(t, h, caseID, "SPOUSE")
	callPBS(t, h, kc, slikZip(t, kc, "3171011234567890", facility("1")))
	timedOut(t, h, caseID)

	if timeoutRows(t, h, caseID) != 1 || systemInputManual(t, h, caseID) != 1 || caseStatus(t, h, caseID) != "Pending-Prescreening" {
		t.Fatalf("timeout rows %d, decisions %d, status %s", timeoutRows(t, h, caseID), systemInputManual(t, h, caseID), caseStatus(t, h, caseID))
	}
	as := assignments(t, h, caseID)
	back := as[len(as)-1]
	if back.Name != "Pre-Screening Submission" || back.Status != "baru" || back.Holder != "AO001" {
		t.Fatalf("assignments = %+v", as)
	}
	p := relogin(t, h, "AO001").OpenAssignment(t, caseID, back.ID)
	if banner, _ := field(t, p, "page", "notesBanner").(map[string]any); banner["decision"] != "Input Manual" || banner["notes"] != manualSLIKNotes {
		t.Fatalf("banner = %s", p.Raw)
	}

	r := callPBS(t, h, ks, slikZip(t, ks, spouseNIK, facility("1")))
	if r.Body["Code"] != "1" || r.Body["message"] != "an unexpected error occurred. Reason: the SLIK waiting time for case "+caseID+" has passed" {
		t.Fatalf("late result = %s", r.Raw)
	}
	sweepNow(t, h)
	if timeoutRows(t, h, caseID) != 1 || systemInputManual(t, h, caseID) != 1 {
		t.Fatal("a second check timed the case out again")
	}
}

// AC-FR-PSAPI-011-04: results within the SLA leave no timeout row.
func TestSLIKResultsWithinTheSLAHaveNoTimeout(t *testing.T) {
	h, _, caseID := slikCase(t, false)
	kc := referenceCode(t, h, caseID, "CUSTOMER")
	callPBS(t, h, kc, slikZip(t, kc, "3171011234567890", facility("1")))
	sweepNow(t, h)
	waitRunEnd(t, h, caseID)
	if timeoutRows(t, h, caseID) != 0 || systemInputManual(t, h, caseID) != 0 || run(t, h, caseID).Step != "Pefindo" {
		t.Fatalf("run %+v", run(t, h, caseID))
	}
}

// AC-FR-PSAPI-011-05, -06: after the timeout Create needs the SLIK document
// (ERR-036) even though it is not mandatory; the Team Lead then sees the
// SLIK log rows above the history, and Sesuai goes on at Pefindo without
// repeating Dukcapil, DHN or SLIK.
func TestManualSLIKEntryNeedsTheSLIKDocumentThenGoesOnAtPefindo(t *testing.T) {
	h, fs, caseID := slikCase(t, false)
	timedOut(t, h, caseID)
	ao := relogin(t, h, "AO001")
	as := assignments(t, h, caseID)
	pss := as[len(as)-1].ID
	ao.OpenAssignment(t, caseID, pss)
	duu := toUnderlying(t, h, ao, caseID, pss)
	ao.OpenAssignment(t, caseID, duu)

	r := createUnderlying(t, ao, caseID, duu, validSLIK())
	if r.Status != http.StatusUnprocessableEntity || !strings.Contains(r.Raw, `"code":"ERR-036"`) || !strings.Contains(r.Raw, `"message":"Upload Dokumen SLIK"`) {
		t.Fatalf("create without SLIK = %d %s", r.Status, r.Raw)
	}
	if r := upload(t, ao, caseID, duu, "SLIK", "SLIK", "slik.pdf", []byte("pdf")); r.Status != http.StatusCreated {
		t.Fatalf("upload SLIK = %d %s", r.Status, r.Raw)
	}
	r = createUnderlying(t, ao, caseID, duu, validSLIK())
	if r.Status != http.StatusOK {
		t.Fatalf("create = %d %s", r.Status, r.Raw)
	}

	tl := relogin(t, h, "TL01")
	p := tl.OpenAssignment(t, caseID, r.Body["assignmentId"])
	rows, _ := field(t, p, "page", "slikTimeout", "logs").([]any)
	var names []string
	for _, row := range rows {
		m := row.(map[string]any)
		names = append(names, m["name"].(string)+" "+m["status"].(string))
	}
	if strings.Join(names, ",") != "SLIK Success,SLIKPBSResponse Fail" || field(t, p, "page", "retry") != nil {
		t.Fatalf("timeout panel = %v; %s", names, p.Raw)
	}
	history, _ := field(t, p, "page", "history").([]any)
	if last := history[len(history)-1].(map[string]any); last["decidedBy"] != "System" || last["decision"] != "Input Manual" {
		t.Fatalf("history = %v", history)
	}

	dukcapilCalls := count(t, h, `SELECT count(*) FROM fos.connect_api_log WHERE case_id = $1 AND api_name IN ('Dukcapil', 'DHN')`, caseID)
	if r := review(t, tl, caseID, r.Body["assignmentId"], "SESUAI", "SLIK manual sesuai."); r.Status != http.StatusNoContent {
		t.Fatalf("sesuai = %d %s", r.Status, r.Raw)
	}
	waitRunEnd(t, h, caseID)
	if got := run(t, h, caseID); got.Step != "Pefindo" || got.State == "STOPPED_SLIK_TIMEOUT" {
		t.Fatalf("run = %+v", got)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.prescreening_runs WHERE case_id = $1`, caseID); n != 1 || len(fs.Calls()) != 1 ||
		count(t, h, `SELECT count(*) FROM fos.connect_api_log WHERE case_id = $1 AND api_name IN ('Dukcapil', 'DHN')`, caseID) != dukcapilCalls {
		t.Fatalf("runs %d, SLIK calls %d", n, len(fs.Calls()))
	}
}

// AC-FR-PSAPI-019-01: with the bypass on, the retry of a failed SLIK
// Pasangan request is a Bypass row without a call; the case returns for
// Manual SLIK Entry, not to the Team Lead's Retry; the Customer's requested
// result arriving later is refused with Code 1, as after the timeout.
func TestSLIKBypassReturnsTheCaseForManualEntry(t *testing.T) {
	fs := apptest.StartFakeAPI(t, apptest.SLIKAccepted, apptest.SLIKInvalid)
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) {
		h.UseSLIK(t, fs)
		h.SeedOperator(t, apptest.Operator{UserID: rpaUser, Password: rpaPassword, RoleCode: "RPA"})
		exec(t, h, `INSERT INTO fos.customer (case_id, customer_type, id_number, full_name, birth_date) VALUES ($1, 'Pasangan', $2, 'Siti Aminah', '1990-01-01')`, caseID, spouseNIK)
		exec(t, h, `UPDATE fos.dynamic_system_settings SET value = 'true' WHERE purpose = 'process/slik/bypass_enabled'`)
	})
	eventually(t, "the bypass", func() bool { return run(t, h, caseID).State == "STOPPED_SLIK_BYPASS" })
	if len(fs.Calls()) != 2 {
		t.Fatalf("SLIK calls = %d", len(fs.Calls()))
	}
	logs := apiLogs(t, h, caseID)
	last, failed := logs[len(logs)-1], logs[len(logs)-2]
	if failed.Name != "SLIKPasangan" || failed.Status != "Fail" || last.Name != "SLIKPasangan" || last.Status != "Bypass" || last.Request != "" || last.Response != "" || last.Method != "" {
		t.Fatalf("logs = %+v / %+v", failed, last)
	}
	if systemInputManual(t, h, caseID) != 1 || caseStatus(t, h, caseID) != "Pending-Prescreening" {
		t.Fatalf("decisions %d, status %s", systemInputManual(t, h, caseID), caseStatus(t, h, caseID))
	}
	as := assignments(t, h, caseID)
	if back := as[len(as)-1]; back.Name != "Pre-Screening Submission" || back.Holder != "AO001" {
		t.Fatalf("assignments = %+v", as)
	}
	kc := referenceCode(t, h, caseID, "CUSTOMER")
	if r := callPBS(t, h, kc, slikZip(t, kc, "3171011234567890", facility("1"))); r.Body["Code"] != "1" || !strings.Contains(r.Raw, "waiting time") {
		t.Fatalf("result after bypass = %s", r.Raw)
	}
}

// FR-PSAPI-011 rule 4 (agreed): the System operator cannot log in.
func TestTheSystemOperatorCannotLogIn(t *testing.T) {
	h := apptest.New(t)
	for _, pw := range []string{"!", "", "System"} {
		if r := h.NewClient().LoginDatabase(t, "System", pw); r.Status == http.StatusOK {
			t.Fatalf("System logged in with %q", pw)
		}
	}
	if n := count(t, h, `SELECT count(*) FROM fos.business_mapping_value WHERE type = 'FAC_DECISION' AND code = 'INPUTMANUAL' AND status = 'Inactive'`); n != 1 {
		t.Fatal("Input Manual not seeded inactive")
	}
}

// AC-FR-PSAPI-019-01: after a bypass, Create needs the SLIK document too.
func TestSLIKBypassManualEntryAlsoNeedsTheSLIKDocument(t *testing.T) {
	fs := apptest.StartFakeAPI(t, apptest.SLIKInvalid)
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) {
		h.UseSLIK(t, fs)
		exec(t, h, `UPDATE fos.dynamic_system_settings SET value = 'true' WHERE purpose = 'process/slik/bypass_enabled'`)
	})
	eventually(t, "the bypass", func() bool { return run(t, h, caseID).State == "STOPPED_SLIK_BYPASS" })
	ao := relogin(t, h, "AO001")
	as := assignments(t, h, caseID)
	pss := as[len(as)-1].ID
	ao.OpenAssignment(t, caseID, pss)
	duu := toUnderlying(t, h, ao, caseID, pss)
	ao.OpenAssignment(t, caseID, duu)
	if r := createUnderlying(t, ao, caseID, duu, validSLIK()); r.Status != http.StatusUnprocessableEntity || !strings.Contains(r.Raw, `"code":"ERR-036"`) {
		t.Fatalf("create without SLIK = %d %s", r.Status, r.Raw)
	}
}

// Review fix (agreed): after a bypass, the Customer's requested result is
// still refused with Code 1 once the Team Lead's Sesuai has the run go on
// at Pefindo; it is neither stored nor judged.
func TestResultAfterABypassStaysRefusedOnceTheRunGoesOn(t *testing.T) {
	fs := apptest.StartFakeAPI(t, apptest.SLIKAccepted, apptest.SLIKInvalid)
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) {
		h.UseSLIK(t, fs)
		h.SeedOperator(t, apptest.Operator{UserID: rpaUser, Password: rpaPassword, RoleCode: "RPA"})
		exec(t, h, `INSERT INTO fos.customer (case_id, customer_type, id_number, full_name, birth_date) VALUES ($1, 'Pasangan', $2, 'Siti Aminah', '1990-01-01')`, caseID, spouseNIK)
		exec(t, h, `UPDATE fos.dynamic_system_settings SET value = 'true' WHERE purpose = 'process/slik/bypass_enabled'`)
	})
	eventually(t, "the bypass", func() bool { return run(t, h, caseID).State == "STOPPED_SLIK_BYPASS" })
	ao := relogin(t, h, "AO001")
	as := assignments(t, h, caseID)
	pss := as[len(as)-1].ID
	ao.OpenAssignment(t, caseID, pss)
	duu := toUnderlying(t, h, ao, caseID, pss)
	ao.OpenAssignment(t, caseID, duu)
	if r := upload(t, ao, caseID, duu, "SLIK", "SLIK", "slik.pdf", []byte("pdf")); r.Status != http.StatusCreated {
		t.Fatalf("upload SLIK = %d %s", r.Status, r.Raw)
	}
	r := createUnderlying(t, ao, caseID, duu, validSLIK())
	if r.Status != http.StatusOK {
		t.Fatalf("create = %d %s", r.Status, r.Raw)
	}
	tl := relogin(t, h, "TL01")
	tl.OpenAssignment(t, caseID, r.Body["assignmentId"])
	if r := review(t, tl, caseID, r.Body["assignmentId"], "SESUAI", "SLIK manual sesuai."); r.Status != http.StatusNoContent {
		t.Fatalf("sesuai = %d %s", r.Status, r.Raw)
	}
	waitRunEnd(t, h, caseID)
	if got := run(t, h, caseID); got.Step != "Pefindo" {
		t.Fatalf("run = %+v", got)
	}

	kc := referenceCode(t, h, caseID, "CUSTOMER")
	res := callPBS(t, h, kc, slikZip(t, kc, "3171011234567890", facility("2", repeat(7, month{"2", "10"})...)))
	if res.Body["Code"] != "1" || !strings.Contains(res.Raw, "waiting time") {
		t.Fatalf("result after the run went on = %s", res.Raw)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.slik_report WHERE case_id = $1 AND status = 'RECEIVED'`, caseID); n != 0 || caseStatus(t, h, caseID) == "ResolvedRejected-Prescreening" {
		t.Fatalf("stored %d, status %s", n, caseStatus(t, h, caseID))
	}
}
