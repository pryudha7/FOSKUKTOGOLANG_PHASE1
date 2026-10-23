package app_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"fos/internal/apptest"
)

const companyNPWP = "012345678901000"

// pefindoBusiness takes a Badan Usaha case through Sesuai with Dukcapil and
// DHN faked (SLIK does not apply) and Pefindo scripted, until its run ends.
func pefindoBusiness(t *testing.T, scripts map[string][]apptest.FakeReply) (*apptest.Harness, *apptest.FakePefindo, string) {
	t.Helper()
	fp := apptest.StartFakePefindo(t, scripts)
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) {
		h.UsePefindo(t, fp)
		exec(t, h, `UPDATE fos.work SET customer_type = 'Badan Usaha' WHERE case_id = $1`, caseID)
		exec(t, h, `INSERT INTO fos.customer (case_id, customer_type, npwp, full_name) VALUES ($1, 'Badan Usaha', $2, 'PT Contoh')`, caseID, companyNPWP)
	})
	waitRunEnd(t, h, caseID)
	return h, fp, caseID
}

// waitRunEnd waits until the case's run is no longer RUNNING.
func waitRunEnd(t *testing.T, h *apptest.Harness, caseID string) {
	t.Helper()
	eventually(t, "the run to end", func() bool { return run(t, h, caseID).State != "RUNNING" })
}

func customerCode(t *testing.T, h *apptest.Harness, caseID string) string {
	t.Helper()
	var code string
	if err := h.DB.QueryRow(context.Background(), `SELECT customer_code FROM fos.work WHERE case_id = $1`, caseID).Scan(&code); err != nil {
		t.Fatal(err)
	}
	return code
}

// pefindoCalls are the fake's calls to one route.
func pefindoCalls(fp *apptest.FakePefindo, route string) []apptest.FakeRequest {
	var out []apptest.FakeRequest
	for _, c := range fp.Calls() {
		if strings.HasPrefix(c.Path, route) {
			out = append(out, c)
		}
	}
	return out
}

// pefindoNames are the case's Pefindo log rows as "api_name status".
func pefindoNames(t *testing.T, h *apptest.Harness, caseID string) []string {
	t.Helper()
	var out []string
	for _, n := range apiNames(apiLogs(t, h, caseID)) {
		if strings.HasPrefix(n, "Pefindo") {
			out = append(out, n)
		}
	}
	return out
}

// AC-FR-PSAPI-014-02, -04, AC-FR-PSAPI-015-01: with no stored token, getToken
// is called once and logged without a case and masked; a Badan Usaha Search
// is CORPORATE with the NPWP and no birth date; code 31 ends Pefindo and,
// with no Spouse, the run completes.
func TestPefindoSearchForABusinessEntity(t *testing.T) {
	h, fp, caseID := pefindoBusiness(t, map[string][]apptest.FakeReply{
		apptest.PefindoToken:  {apptest.PefindoTokenOK},
		apptest.PefindoSearch: {apptest.PefindoNotFound},
	})
	if got := run(t, h, caseID); got.State != "COMPLETED" {
		t.Fatalf("run = %+v", got)
	}
	tokens := pefindoCalls(fp, apptest.PefindoToken)
	if len(tokens) != 1 || tokens[0].Method != http.MethodGet || tokens[0].Header.Get("Authorization") != apptest.PefindoAuthorization {
		t.Fatalf("token calls = %+v", tokens)
	}
	searches := pefindoCalls(fp, apptest.PefindoSearch)
	if len(searches) != 1 {
		t.Fatalf("searches = %d", len(searches))
	}
	s := searches[0]
	if s.Method != http.MethodPost || s.Header.Get("x-gateway-apikey") != apptest.PefindoAPIKey ||
		s.Header.Get("Authorization") != "Bearer "+apptest.PefindoTokenValue || s.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("search headers = %v", s.Header)
	}
	want := `{"inquiry_reason":"12","product_id":"1","reference_code":"` + customerCode(t, h, caseID) +
		`","type":"CORPORATE","params":[{"id_no":"` + companyNPWP + `","id_type":"NPWP","name":"PT Contoh"}]}`
	if s.Body != want {
		t.Fatalf("search body = %s\nwant %s", s.Body, want)
	}
	if got := pefindoNames(t, h, caseID); strings.Join(got, ",") != "Pefindo Success" {
		t.Fatalf("logs = %v", got)
	}
	var req, resp string
	var headers []byte
	if err := h.DB.QueryRow(context.Background(), `
		SELECT COALESCE(request, ''), response, request_headers FROM fos.connect_api_log
		WHERE api_name = 'PefindoToken' AND case_id IS NULL AND api_type = 'ConnectRest' AND api_status = 'Success'`).Scan(&req, &resp, &headers); err != nil {
		t.Fatalf("token log: %v", err)
	}
	var hs map[string][]string
	_ = json.Unmarshal(headers, &hs)
	if strings.Contains(resp, apptest.PefindoTokenValue) || !strings.Contains(resp, `"token":"***"`) || hs["Authorization"][0] != "***" {
		t.Fatalf("token log response %s headers %v", resp, hs)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.integration_token WHERE provider = 'PEFINDO' AND token_ciphertext IS NOT NULL AND position($1::bytea IN token_ciphertext) = 0`, []byte(apptest.PefindoTokenValue)); n != 1 {
		t.Fatal("token not stored encrypted")
	}
}

// pefindoIndividual takes an Individual case, with or without a Spouse,
// through Sesuai and its SLIK results (no rejection) into Pefindo, scripted,
// until its run ends.
func pefindoIndividual(t *testing.T, withSpouse bool, scripts map[string][]apptest.FakeReply) (*apptest.Harness, *apptest.FakePefindo, string) {
	t.Helper()
	h, _, caseID := slikCase(t, withSpouse)
	fp := apptest.StartFakePefindo(t, scripts)
	h.UsePefindo(t, fp)
	kc := referenceCode(t, h, caseID, "CUSTOMER")
	callPBS(t, h, kc, slikZip(t, kc, "3171011234567890", facility("1")))
	if withSpouse {
		ks := referenceCode(t, h, caseID, "SPOUSE")
		callPBS(t, h, ks, slikZip(t, ks, spouseNIK, facility("1")))
	}
	waitRunEnd(t, h, caseID)
	return h, fp, caseID
}

// searchHit is a Search data element.
func searchHit(idPefindo string, similarity float64) string {
	b, _ := json.Marshal(map[string]any{"similarity_score": similarity, "id_pefindo": json.Number(idPefindo), "id_type": "KTP", "nama_debitur": "BUDI"})
	return string(b)
}

// scoring is one Get Report scoring element.
func scoring(period, grade string) map[string]any {
	return map[string]any{"id_pefindo": "101010267000004", "period": period, "score": 600, "risk_grade": grade,
		"risk_grade_desc": "Risiko", "risk_desc": "Keterangan", "pod": 0.05, "reason_code": []string{"R1", "R2"}, "reason_desc": []string{"Alasan 1"}}
}

// reportReady is a Get Report answer with a report and scoring.
func reportReady(code string, scores ...map[string]any) apptest.FakeReply {
	b, _ := json.Marshal(map[string]any{"code": code, "status": "Success", "message": "ok", "event_id": "e",
		"report": map[string]any{"header": map[string]any{"username": "BSIM", "id_report": 12345, "inquiry_date": "2026-10-14 09:30:00"},
			"debitur": map[string]any{"nomor_identitas": "3171011234567890"}, "fasilitas": []any{}, "pengurus": []any{}, "CurrentContract": nil},
		"scoring": scores})
	return apptest.FakeReply{Status: 200, Body: string(b)}
}

// pefindoPassScripts answer every Pefindo API with a pass, for the Customer
// and then the Spouse.
func pefindoPassScripts() map[string][]apptest.FakeReply {
	return map[string][]apptest.FakeReply{
		apptest.PefindoToken:    {apptest.PefindoTokenOK},
		apptest.PefindoSearch:   {apptest.PefindoFound(searchHit("101100684000001", 80), searchHit("101100684000004", 100)), apptest.PefindoFound(searchHit("202200000000002", 90))},
		apptest.PefindoGenerate: {apptest.PefindoGenerated},
		apptest.PefindoDownload: {apptest.PefindoPDF},
		apptest.PefindoReport:   {reportReady("01", scoring("2025-07-15", "C2"), scoring("2025-06-15", "B1"))},
	}
}

// pefindoSearches are the case's Search rows by api_name: EventID,
// id_pefindo, inquiry_id.
func pefindoSearches(t *testing.T, h *apptest.Harness, caseID string) map[string][3]string {
	t.Helper()
	out := map[string][3]string{}
	rows, err := h.DB.Query(context.Background(), `
		SELECT api_name, COALESCE(event_id, ''), COALESCE(pefindo_id, ''), COALESCE(inquiry_id, '') FROM fos.connect_api_log
		WHERE case_id = $1 AND api_name IN ('Pefindo', 'PefindoPasangan') ORDER BY submitted_at, ctid`, caseID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var s [3]string
		_ = rows.Scan(&name, &s[0], &s[1], &s[2])
		out[name] = s
	}
	return out
}

// AC-FR-PSAPI-014-01, -03, -06, AC-FR-PSAPI-015-02, AC-FR-PSAPI-016-01, -04,
// AC-FR-PSAPI-017-01, -06, -07, AC-FR-PSAPI-018-01, -02, -05: every Customer
// API (Search, Generate Report, Download PDF, Get Report) runs before any
// Spouse API, each subject with its own Search's EventID, inquiry and
// id_pefindo (highest similarity); one token serves them all; the PDFs and
// reports are stored; the run completes.
func TestPefindoRunsTheCustomerThenTheSpouse(t *testing.T) {
	h, fp, caseID := pefindoIndividual(t, true, pefindoPassScripts())
	if got := run(t, h, caseID); got.State != "COMPLETED" {
		t.Fatalf("run = %+v", got)
	}
	want := []string{
		"Pefindo Success", "PefindoGenerateReport Success", "PefindoDownloadPDF Success", "PefindoGetReport Success",
		"PefindoPasangan Success", "PefindoPasanganGenerateReport Success", "PefindoPasanganDownloadPDF Success", "PefindoPasanganGetReport Success",
	}
	if got := pefindoNames(t, h, caseID); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("logs = %v", got)
	}
	if n := len(pefindoCalls(fp, apptest.PefindoToken)); n != 1 {
		t.Fatalf("token requests = %d", n)
	}
	searches := pefindoSearches(t, h, caseID)
	c, sp := searches["Pefindo"], searches["PefindoPasangan"]
	if len(c[0]) != 36 || c[0] == sp[0] || c[1] != "101100684000004" || sp[1] != "202200000000002" || c[2] != "1436899" {
		t.Fatalf("searches = %+v", searches)
	}

	calls := fp.Calls()
	var paths []string
	for _, call := range calls {
		paths = append(paths, call.Method+" "+call.Path)
	}
	wantPaths := []string{
		"GET " + apptest.PefindoToken, "POST " + apptest.PefindoSearch, "POST " + apptest.PefindoGenerate,
		"GET " + apptest.PefindoDownload + c[0], "GET " + apptest.PefindoReport + c[0],
		"POST " + apptest.PefindoSearch, "POST " + apptest.PefindoGenerate,
		"GET " + apptest.PefindoDownload + sp[0], "GET " + apptest.PefindoReport + sp[0],
	}
	if strings.Join(paths, "\n") != strings.Join(wantPaths, "\n") {
		t.Fatalf("calls:\n%s\nwant:\n%s", strings.Join(paths, "\n"), strings.Join(wantPaths, "\n"))
	}
	ref := customerCode(t, h, caseID)
	if want := `{"inquiry_reason":"12","product_id":"1","reference_code":"` + ref + `","type":"PERSONAL","params":[{"date_of_birth":"1990-01-01","id_no":"` + spouseNIK + `","id_type":"KTP","name":"Siti Aminah"}]}`; calls[5].Body != want {
		t.Fatalf("spouse search = %s", calls[5].Body)
	}
	if want := `{"event_id":"` + c[0] + `","inquiry_id":"1436899","language":"01","ids":[{"id_no":"3171011234567890","id_pefindo":"101100684000004","id_type":"KTP"}]}`; calls[2].Body != want {
		t.Fatalf("generate = %s", calls[2].Body)
	}
	for _, i := range []int{2, 3, 4} {
		hd := calls[i].Header
		if hd.Get("x-gateway-apikey") != "" || hd.Get("Authorization") != "Bearer "+apptest.PefindoTokenValue || hd.Get("Content-Type") != "application/json" {
			t.Fatalf("call %d headers = %v", i, hd)
		}
	}

	recorder := lastTL(t, h, caseID)
	for sql, want := range map[string]int{
		`SELECT count(*) FROM fos.case_document WHERE case_id = $1 AND document_category = 'PreScreeningResponse' AND document_type = 'PEFINDO_PDF_CUSTOMER' AND file_name = 'Pefindo Report - ' || $2 || '.pdf' AND content_type = 'application/pdf' AND created_by = $3`:                                                                                                                                                                                               1,
		`SELECT count(*) FROM fos.case_document WHERE case_id = $1 AND document_category = 'PreScreeningResponse' AND document_type = 'PEFINDO_PDF_SPOUSE' AND file_name = 'Pefindo Report - ' || $2 || ' - Spouse.pdf' AND created_by = $3`:                                                                                                                                                                                                                             1,
		`SELECT count(*) FROM fos.connect_api_log WHERE case_id = $1 AND api_name = 'PefindoDownloadPDF' AND response = '[application/pdf, 23 bytes]' AND http_status_code = 200 AND $2 <> $3`:                                                                                                                                                                                                                                                                           1,
		`SELECT count(*) FROM fos.pefindo_report WHERE case_id = $1 AND subject_type = 'CUSTOMER' AND report_code = '01' AND latest_period = '2025-07-15' AND latest_risk_grade = 'C2' AND latest_score = 600 AND header_username = 'BSIM' AND header_id_report = '12345' AND header_inquiry_date = '2026-10-14 02:30:00' AND report_json->'debitur'->>'nomor_identitas' = '3171011234567890' AND management_json = '[]' AND current_contract_json IS NULL AND $2 <> $3`: 1,
		`SELECT count(*) FROM fos.pefindo_scoring s JOIN fos.pefindo_report r USING (pefindo_report_id) WHERE r.case_id = $1 AND s.probability_of_default = 0.05 AND s.risk_grade_description = 'Risiko' AND $2 <> $3`:                                                                                                                                                                                                                                                   4,
		`SELECT count(*) FROM fos.pefindo_scoring_reason x JOIN fos.pefindo_scoring s USING (pefindo_scoring_id) JOIN fos.pefindo_report r USING (pefindo_report_id) WHERE r.case_id = $1 AND (x.sequence_number = 1 AND x.reason_description = 'Alasan 1' OR x.sequence_number = 2 AND x.reason_code = 'R2' AND x.reason_description IS NULL) AND $2 <> $3`:                                                                                                             8,
	} {
		if n := count(t, h, sql, caseID, ref, recorder); n != want {
			t.Fatalf("%d for %s, want %d", n, sql, want)
		}
	}
}

// lastTL is the Team Lead who submitted the case's last Pre-Screening Review.
func lastTL(t *testing.T, h *apptest.Harness, caseID string) string {
	t.Helper()
	var by string
	if err := h.DB.QueryRow(context.Background(), `
		SELECT worked_by_user_id FROM fos.assignments WHERE case_id = $1 AND assignment_name = 'Pre Screening' AND assignment_status = 'sudah selesai dikerjakan'
		ORDER BY completed_at DESC LIMIT 1`, caseID).Scan(&by); err != nil {
		t.Fatal(err)
	}
	return by
}

// AC-FR-PSAPI-017-03, -04, -06, -08 (Rejected List part): the Customer's
// latest period (not the last element) decides: an old E2 with a later C1
// passes; the Spouse's latest " e3 " rejects after its report is stored:
// log Rejected, the case Rejected, both on the Rejected List with the
// PefindoPasangan reason.
func TestPefindoRejectsOnTheLatestRiskGrade(t *testing.T) {
	scripts := pefindoPassScripts()
	scripts[apptest.PefindoReport] = []apptest.FakeReply{
		reportReady("01", scoring("2025-08-01", "C1"), scoring("2025-03-01", "E2")),
		reportReady("01", scoring("2025-01-01", "A1"), scoring("2025-09-01", " e3 ")),
	}
	h, _, caseID := pefindoIndividual(t, true, scripts)
	if got := run(t, h, caseID); got.State != "STOPPED_REJECTED" || got.Step != "Pefindo Pasangan" || caseStatus(t, h, caseID) != "ResolvedRejected-Prescreening" || currentStage(t, h, caseID) != "Rejected" {
		t.Fatalf("run %+v, status %s", got, caseStatus(t, h, caseID))
	}
	names := pefindoNames(t, h, caseID)
	if names[3] != "PefindoGetReport Success" || names[len(names)-1] != "PefindoPasanganGetReport Rejected" {
		t.Fatalf("logs = %v", names)
	}
	want := `• Reject Pre-screening "PefindoPasangan" terhadap kriteria "Risk Grade di bawah D3"`
	list := rejectedList(t, h)
	if len(list) != 2 || list[0].Reason != want || list[1].Reason != want {
		t.Fatalf("rejected list = %+v", list)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.pefindo_report WHERE case_id = $1 AND subject_type = 'SPOUSE' AND latest_risk_grade = ' e3 ' AND latest_period = '2025-09-01'`, caseID); n != 1 {
		t.Fatal("spouse report not stored before the reject")
	}
}

// AC-FR-PSAPI-017-03: a Customer reject stops the run before any Spouse API.
func TestPefindoCustomerRejectSkipsTheSpouse(t *testing.T) {
	scripts := pefindoPassScripts()
	scripts[apptest.PefindoReport] = []apptest.FakeReply{reportReady("01", scoring("2025-07-15", "E1"))}
	h, fp, caseID := pefindoIndividual(t, true, scripts)
	if got := run(t, h, caseID); got.State != "STOPPED_REJECTED" || got.Step != "Pefindo" {
		t.Fatalf("run = %+v", got)
	}
	if n := len(pefindoCalls(fp, apptest.PefindoSearch)); n != 1 {
		t.Fatalf("searches = %d", n)
	}
	for _, r := range rejectedList(t, h) {
		if r.Reason != `• Reject Pre-screening "Pefindo" terhadap kriteria "Risk Grade di bawah D3"` {
			t.Fatalf("reason = %s", r.Reason)
		}
	}
}

// AC-FR-PSAPI-014-07: a Customer Search code 31 makes no EventID and no
// other Customer API; the Spouse's Pefindo still runs.
func TestPefindoNotFoundStillRunsTheSpouse(t *testing.T) {
	scripts := pefindoPassScripts()
	scripts[apptest.PefindoSearch] = []apptest.FakeReply{apptest.PefindoNotFound, apptest.PefindoFound(searchHit("202200000000002", 90))}
	h, fp, caseID := pefindoIndividual(t, true, scripts)
	if got := run(t, h, caseID); got.State != "COMPLETED" {
		t.Fatalf("run = %+v", got)
	}
	names := pefindoNames(t, h, caseID)
	if names[0] != "Pefindo Success" || names[1] != "PefindoPasangan Success" || len(names) != 5 {
		t.Fatalf("logs = %v", names)
	}
	if s := pefindoSearches(t, h, caseID)["Pefindo"]; s != [3]string{} {
		t.Fatalf("customer search kept %v", s)
	}
	if n := len(pefindoCalls(fp, apptest.PefindoGenerate)); n != 1 {
		t.Fatalf("generate calls = %d", n)
	}
}

// AC-FR-PSAPI-014-05, AC-FR-PSAPI-015-04, -05: a code 06 requests a new
// token (a PefindoToken row without a case) and repeats the API at once
// without the retry gap, using one retry; a failed token request is not
// repeated and the API is repeated anyway; when the retries run out the
// case returns to its Team Lead.
func TestPefindoExpiredTokenIsRenewedAndRetriedAtOnce(t *testing.T) {
	scripts := map[string][]apptest.FakeReply{
		apptest.PefindoToken:  {apptest.PefindoTokenOK, {Status: 200, Body: `{"code":"01","data":{}}`}, {Hang: true}, apptest.PefindoTokenOK},
		apptest.PefindoSearch: {apptest.PefindoExpired},
	}
	fp := apptest.StartFakePefindo(t, scripts)
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) {
		h.UsePefindo(t, fp)
		exec(t, h, `UPDATE fos.dynamic_system_settings SET value = '600000' WHERE purpose = 'api/pefindo/retry_gap_ms'`)
		exec(t, h, `UPDATE fos.work SET customer_type = 'Badan Usaha' WHERE case_id = $1`, caseID)
		exec(t, h, `INSERT INTO fos.customer (case_id, customer_type, npwp, full_name) VALUES ($1, 'Badan Usaha', $2, 'PT Contoh')`, caseID, companyNPWP)
	})
	waitRunEnd(t, h, caseID)
	if got := run(t, h, caseID); got.State != "STOPPED_FAILED" || caseStatus(t, h, caseID) != "Failed-PrescreeningConnection" {
		t.Fatalf("run %+v, status %s", got, caseStatus(t, h, caseID))
	}
	if got := pefindoNames(t, h, caseID); strings.Join(got, ",") != "Pefindo Fail,Pefindo Fail,Pefindo Fail,Pefindo Fail" {
		t.Fatalf("logs = %v", got)
	}
	var tokens []string
	rows, err := h.DB.Query(context.Background(), `SELECT api_status FROM fos.connect_api_log WHERE api_name = 'PefindoToken' AND case_id IS NULL ORDER BY submitted_at, ctid`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		tokens = append(tokens, s)
	}
	rows.Close()
	// The first token, then one per 06: no data.token, a timeout, then tokens.
	if strings.Join(tokens, ",") != "Success,Fail,Fail,Success,Success" {
		t.Fatalf("token rows = %v", tokens)
	}
	if n := len(pefindoCalls(fp, apptest.PefindoSearch)); n != 4 {
		t.Fatalf("searches = %d", n)
	}
}

// AC-FR-PSAPI-017-02, AC-FR-PSAPI-018-04, FR-PSAPI-001 rule 5: a code 32
// fails and is retried until the retries run out; the TL's Retry resumes at
// the failed API (agreed: current_api), not at Search.
func TestPefindoStillScoringFailsAndRetryResumesAtTheFailedAPI(t *testing.T) {
	scripts := pefindoPassScripts()
	scripts[apptest.PefindoDownload] = []apptest.FakeReply{apptest.PefindoScoring, apptest.PefindoNoPDFYet}
	scripts[apptest.PefindoReport] = []apptest.FakeReply{apptest.PefindoScoring}
	fp := apptest.StartFakePefindo(t, scripts)
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) {
		h.UsePefindo(t, fp)
		exec(t, h, `UPDATE fos.work SET customer_type = 'Badan Usaha' WHERE case_id = $1`, caseID)
		exec(t, h, `INSERT INTO fos.customer (case_id, customer_type, npwp, full_name) VALUES ($1, 'Badan Usaha', $2, 'PT Contoh')`, caseID, companyNPWP)
	})
	waitRunEnd(t, h, caseID)
	want := "Pefindo Success,PefindoGenerateReport Success,PefindoDownloadPDF Fail,PefindoDownloadPDF Success," +
		"PefindoGetReport Fail,PefindoGetReport Fail,PefindoGetReport Fail,PefindoGetReport Fail"
	if got := pefindoNames(t, h, caseID); strings.Join(got, ",") != want {
		t.Fatalf("logs = %v", got)
	}
	if got := run(t, h, caseID); got.State != "STOPPED_FAILED" || caseStatus(t, h, caseID) != "Failed-PrescreeningConnection" {
		t.Fatalf("run %+v, status %s", got, caseStatus(t, h, caseID))
	}
	if n := count(t, h, `SELECT count(*) FROM fos.case_document WHERE case_id = $1 AND document_type LIKE 'PEFINDO_PDF_%'`, caseID); n != 0 {
		t.Fatal("a PDF was stored for a JSON answer")
	}

	as := assignments(t, h, caseID)
	retry := as[len(as)-1]
	tl := relogin(t, h, "TL01")
	tl.OpenAssignment(t, caseID, retry.ID)
	fp.Script(apptest.PefindoReport, reportReady("36"))
	if r := tl.Do(t, http.MethodPost, submitPath(caseID, retry.ID, "retry"), nil); r.Status != http.StatusNoContent {
		t.Fatalf("retry = %d %s", r.Status, r.Raw)
	}
	eventually(t, "the run to complete", func() bool { return run(t, h, caseID).State == "COMPLETED" })
	if got := pefindoNames(t, h, caseID); strings.Join(got, ",") != want+",PefindoGetReport Success" {
		t.Fatalf("logs after retry = %v", got)
	}
	if n := len(pefindoCalls(fp, apptest.PefindoSearch)); n != 1 {
		t.Fatalf("searches = %d", n)
	}
}

// AC-FR-PSAPI-017-05, AC-FR-PSAPI-018-03: Download PDF code 36 or 9s and
// Get Report code 9s without a report pass without storing anything.
func TestPefindoCodesWithoutData(t *testing.T) {
	for _, c := range []struct{ download, report apptest.FakeReply }{
		{apptest.FakeReply{Status: 200, Body: `{"code":"36","status":"Success"}`}, apptest.FakeReply{Status: 200, Body: `{"code":"9999","status":"Success","message":"gagal hitung score"}`}},
		{apptest.FakeReply{Status: 200, Body: `{"code":"99","status":"Success"}`}, reportReady("36")},
	} {
		scripts := pefindoPassScripts()
		scripts[apptest.PefindoDownload] = []apptest.FakeReply{c.download}
		scripts[apptest.PefindoReport] = []apptest.FakeReply{c.report}
		h, _, caseID := pefindoBusiness(t, scripts)
		if got := run(t, h, caseID); got.State != "COMPLETED" {
			t.Fatalf("run = %+v", got)
		}
		reports := count(t, h, `SELECT count(*) FROM fos.pefindo_report WHERE case_id = $1`, caseID)
		if count(t, h, `SELECT count(*) FROM fos.case_document WHERE case_id = $1 AND document_type LIKE 'PEFINDO_PDF_%'`, caseID) != 0 ||
			(c.report.Body[9] == '9') != (reports == 0) {
			t.Fatalf("stored for %s / %s: %d reports", c.download.Body, c.report.Body, reports)
		}
	}
}

// AC-FR-PSAPI-017-06, DB-RULE-074: a new Get Report on the subject replaces
// its old report and scoring.
func TestPefindoReportReplacesTheOldOne(t *testing.T) {
	scripts := pefindoPassScripts()
	h, _, caseID := pefindoBusiness(t, scripts)
	if n := count(t, h, `SELECT count(*) FROM fos.pefindo_scoring s JOIN fos.pefindo_report r USING (pefindo_report_id) WHERE r.case_id = $1`, caseID); n != 2 {
		t.Fatalf("scoring = %d", n)
	}
	exec(t, h, `UPDATE fos.pefindo_report SET report_code = 'OLD' WHERE case_id = $1`, caseID)
	// A TL Retry after a later failure, or a reopen, runs Get Report again;
	// here the run is put back on it.
	exec(t, h, `UPDATE fos.prescreening_runs SET state = 'STOPPED_FAILED', current_step = 'Pefindo', current_api = 'PefindoGetReport' WHERE case_id = $1`, caseID)
	exec(t, h, `UPDATE fos.work SET case_status = 'Failed-PrescreeningConnection' WHERE case_id = $1`, caseID)
	exec(t, h, `INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at) VALUES ($1, 'Pre Screening', 'worklist', 'TL01', 'baru', now())`, caseID)
	as := assignments(t, h, caseID)
	retry := as[len(as)-1]
	tl := relogin(t, h, "TL01")
	tl.OpenAssignment(t, caseID, retry.ID)
	if r := tl.Do(t, http.MethodPost, submitPath(caseID, retry.ID, "retry"), nil); r.Status != http.StatusNoContent {
		t.Fatalf("retry = %d %s", r.Status, r.Raw)
	}
	eventually(t, "the run to complete", func() bool { return run(t, h, caseID).State == "COMPLETED" })
	if n := count(t, h, `SELECT count(*) FROM fos.pefindo_report WHERE case_id = $1 AND report_code = '01'`, caseID); n != 1 || count(t, h, `SELECT count(*) FROM fos.pefindo_report`) != 1 {
		t.Fatal("old report not replaced")
	}
	if n := count(t, h, `SELECT count(*) FROM fos.pefindo_scoring_reason`); n != 4 {
		t.Fatalf("reasons = %d", n)
	}
}

// DB-RULE-075: a report that cannot be saved fails Get Report, which is
// retried; nothing is stored and the case is not rejected.
func TestPefindoReportNotSavedFailsTheAttempt(t *testing.T) {
	scripts := pefindoPassScripts()
	scripts[apptest.PefindoReport] = []apptest.FakeReply{reportReady("01", scoring("2025-07-15", "E1"))}
	fp := apptest.StartFakePefindo(t, scripts)
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) {
		h.UsePefindo(t, fp)
		exec(t, h, `ALTER TABLE fos.pefindo_report ADD CONSTRAINT refuse CHECK (report_code <> '01')`)
		exec(t, h, `UPDATE fos.work SET customer_type = 'Badan Usaha' WHERE case_id = $1`, caseID)
		exec(t, h, `INSERT INTO fos.customer (case_id, customer_type, npwp, full_name) VALUES ($1, 'Badan Usaha', $2, 'PT Contoh')`, caseID, companyNPWP)
	})
	waitRunEnd(t, h, caseID)
	if got := run(t, h, caseID); got.State != "STOPPED_FAILED" || caseStatus(t, h, caseID) != "Failed-PrescreeningConnection" {
		t.Fatalf("run %+v, status %s", got, caseStatus(t, h, caseID))
	}
	names := pefindoNames(t, h, caseID)
	if strings.Join(names[3:], ",") != "PefindoGetReport Fail,PefindoGetReport Fail,PefindoGetReport Fail,PefindoGetReport Fail" || len(rejectedList(t, h)) != 0 {
		t.Fatalf("logs = %v", names)
	}
}

// DEC-099, DB-RULE-084: without the token key the run stops broken before
// any Pefindo call.
func TestPefindoWithoutTheTokenKeyStopsBroken(t *testing.T) {
	fp := apptest.StartFakePefindo(t, pefindoPassScripts())
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) {
		h.UsePefindo(t, fp)
		t.Setenv("INTEGRATION_TOKEN_KEY", "")
		exec(t, h, `UPDATE fos.work SET customer_type = 'Badan Usaha' WHERE case_id = $1`, caseID)
		exec(t, h, `INSERT INTO fos.customer (case_id, customer_type, npwp, full_name) VALUES ($1, 'Badan Usaha', $2, 'PT Contoh')`, caseID, companyNPWP)
	})
	waitRunEnd(t, h, caseID)
	if got := run(t, h, caseID); got.State != "STOPPED_BROKEN" || got.Step != "Pefindo" || len(fp.Calls()) != 0 {
		t.Fatalf("run %+v, calls %d", got, len(fp.Calls()))
	}
}

// AC-FR-PSAPI-015-03, DB-RULE-080: an executor that needs a token while
// another holds the token row waits, then uses the token written meanwhile
// instead of requesting its own.
func TestPefindoTokenRequestsMerge(t *testing.T) {
	fp := apptest.StartFakePefindo(t, map[string][]apptest.FakeReply{
		apptest.PefindoToken:  {apptest.PefindoTokenOK},
		apptest.PefindoSearch: {apptest.PefindoNotFound},
	})
	ctx := context.Background()
	var tx pgx.Tx
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) {
		h.UsePefindo(t, fp)
		exec(t, h, `UPDATE fos.work SET customer_type = 'Badan Usaha' WHERE case_id = $1`, caseID)
		exec(t, h, `INSERT INTO fos.customer (case_id, customer_type, npwp, full_name) VALUES ($1, 'Badan Usaha', $2, 'PT Contoh')`, caseID, companyNPWP)
		var err error
		if tx, err = h.DB.Begin(ctx); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(ctx) })
		if _, err := tx.Exec(ctx, `SELECT 1 FROM fos.integration_token WHERE provider = 'PEFINDO' FOR UPDATE`); err != nil {
			t.Fatal(err)
		}
	})
	eventually(t, "the run to reach Pefindo", func() bool { return run(t, h, caseID).Step == "Pefindo" })
	time.Sleep(300 * time.Millisecond) // the executor now waits for the token row

	// What another executor's getToken would have written.
	key, _ := base64.StdEncoding.DecodeString(os.Getenv("INTEGRATION_TOKEN_KEY"))
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	nonce := make([]byte, aead.NonceSize())
	_, _ = rand.Read(nonce)
	if _, err := tx.Exec(ctx, `UPDATE fos.integration_token SET token_ciphertext = $1, token_nonce = $2, key_version = 'INTEGRATION_TOKEN_KEY', obtained_at = now(), updated_at = now() WHERE provider = 'PEFINDO'`,
		aead.Seal(nil, nonce, []byte("tok-2"), []byte("PEFINDO")), nonce); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	waitRunEnd(t, h, caseID)
	searches := pefindoCalls(fp, apptest.PefindoSearch)
	if n := len(pefindoCalls(fp, apptest.PefindoToken)); n != 0 || len(searches) != 1 || searches[0].Header.Get("Authorization") != "Bearer tok-2" {
		t.Fatalf("token calls %d, searches %+v", n, searches)
	}
}

// Agreed for ticket 13: a simulated Pefindo API logs its configured answer
// and needs no token; the simulated Search (code 31) ends Pefindo.
func TestPefindoSimulation(t *testing.T) {
	fp := apptest.StartFakePefindo(t, pefindoPassScripts())
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) {
		h.UsePefindo(t, fp)
		exec(t, h, `UPDATE fos.dynamic_system_settings SET value = 'true' WHERE purpose = 'api/pefindo/simulate'`)
		exec(t, h, `UPDATE fos.work SET customer_type = 'Badan Usaha' WHERE case_id = $1`, caseID)
		exec(t, h, `INSERT INTO fos.customer (case_id, customer_type, npwp, full_name) VALUES ($1, 'Badan Usaha', $2, 'PT Contoh')`, caseID, companyNPWP)
	})
	waitRunEnd(t, h, caseID)
	if got := run(t, h, caseID); got.State != "COMPLETED" || len(fp.Calls()) != 0 {
		t.Fatalf("run %+v, calls %d", got, len(fp.Calls()))
	}
	logs := apiLogs(t, h, caseID)
	last := logs[len(logs)-1]
	if last.Name != "Pefindo" || last.Status != "Success" || !strings.Contains(last.Response, `"code":"31"`) {
		t.Fatalf("log = %+v", last)
	}
}
