package app_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"fos/internal/apptest"
)

const (
	pbsPath     = "/api/NBS/v1/PBSResponse"
	rpaUser     = "rpa01"
	rpaPassword = "Rpa#Pass1"
	spouseNIK   = "3171019876543210"
)

// month is one of a facility's 24 months: collectability and days past due
// ("" for an empty month).
type month struct{ kol, ht string }

// facility builds a kreditPembiayan element with quality and 24 months; the
// first len(bad) months take bad's values, the rest are 1/0.
func facility(quality string, bad ...month) map[string]any {
	f := map[string]any{
		"ljk": "999", "ljkKet": "Bank Contoh", "cabangKet": "Jakarta", "noRekening": "0001", "kualitas": quality,
		"jumlahHariTunggakan": "0", "plafon": "10000000", "bakiDebet": "5000000", "tanggalAkadAwal": "20230509",
		"tahun": "2026", "bulan": "9",
		"agunan":   []any{map[string]any{"jenisAgunanKet": "Tanah", "nilaiAgunanMenurutLJK": "250000000", "tanggalUpdate": "20260101"}},
		"penjamin": []any{map[string]any{"namaPenjamin": "Ayah", "nomorIdentitas": "3171000000000001"}},
	}
	for m := 1; m <= 24; m++ {
		kol, ht := "1", "0"
		if m <= len(bad) {
			kol, ht = bad[m-1].kol, bad[m-1].ht
		}
		f[fmt.Sprintf("tahunBulan%02d", m)] = fmt.Sprintf("2024%02d", (m-1)%12+1)
		f[fmt.Sprintf("tahunBulan%02dKol", m)] = kol
		f[fmt.Sprintf("tahunBulan%02dHt", m)] = ht
	}
	return f
}

// repeat is n months of the same values.
func repeat(n int, m month) []month {
	out := make([]month, n)
	for i := range out {
		out[i] = m
	}
	return out
}

// slikZip is an RPA result ZIP: KTP_<nik>.txt with the given facilities, and
// a Recap file that must be ignored.
func slikZip(t *testing.T, kode, nik string, facilities ...map[string]any) []byte {
	t.Helper()
	items := make([]any, len(facilities))
	for i, f := range facilities {
		items[i] = f
	}
	doc := map[string]any{
		"header": map[string]any{"kodeReferensiPengguna": kode, "tanggalHasil": "20261014", "idPermintaan": "0000000001"},
		"individual": map[string]any{
			"nomorLaporan": "R000001", "posisiDataTerakhir": "202609",
			"parameterPencarian": map[string]any{"namaDebitur": "BUDI SANTOSO", "noIdentitas": nik, "tanggalLahir": "19960101"},
			"dataPokokDebitur":   []any{map[string]any{"namaDebitur": "BUDI SANTOSO", "noIdentitas": nik, "alamat": "Jl. Sudirman"}},
			"ringkasanFasilitas": map[string]any{"kualitasTerburuk": "2", "kualitasBulanDataTerburuk": "202608", "plafonEfektifTotal": "10000000"},
			"fasilitas":          map[string]any{"kreditPembiayan": items, "lc": []any{}, "garansiYgDiberikan": []any{}, "fasilitasLain": []any{}},
		},
	}
	body, _ := json.Marshal(doc)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range map[string][]byte{
		"Recap_BSIM_9_LOS_20261014_abc_rn_40PX.txt": []byte("rekap"),
		"KTP_" + nik + ".txt":                       append([]byte("\xef\xbb\xbf"), body...),
	} {
		w, _ := zw.Create(name)
		_, _ = w.Write(content)
	}
	_ = zw.Close()
	return buf.Bytes()
}

// slikCase takes a case through Sesuai with Dukcapil, DHN and SLIK faked,
// with or without a Spouse, until the run waits for the SLIK results.
func slikCase(t *testing.T, withSpouse bool, slik ...apptest.FakeReply) (*apptest.Harness, *apptest.FakeAPI, string) {
	t.Helper()
	if len(slik) == 0 {
		slik = []apptest.FakeReply{apptest.SLIKAccepted}
	}
	fs := apptest.StartFakeAPI(t, slik...)
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) {
		h.UseSLIK(t, fs)
		h.SeedOperator(t, apptest.Operator{UserID: rpaUser, Password: rpaPassword, RoleCode: "RPA"})
		if withSpouse {
			exec(t, h, `INSERT INTO fos.customer (case_id, customer_type, id_number, full_name, birth_date) VALUES ($1, 'Pasangan', $2, 'Siti Aminah', '1990-01-01')`, caseID, spouseNIK)
		}
	})
	eventually(t, "the run to wait for SLIK", func() bool {
		r := run(t, h, caseID)
		return r.State == "WAITING_SLIK" || r.State == "STOPPED_FAILED" || r.State == "STOPPED_BROKEN"
	})
	if got := run(t, h, caseID); got.State != "WAITING_SLIK" || got.Step != "Pefindo" {
		t.Fatalf("run = %+v", got)
	}
	return h, fs, caseID
}

// referenceCode is the kode reff of a subject.
func referenceCode(t *testing.T, h *apptest.Harness, caseID, subject string) string {
	t.Helper()
	var code string
	if err := h.DB.QueryRow(context.Background(), `SELECT reference_code FROM fos.slik_report WHERE case_id = $1 AND subject_type = $2`, caseID, subject).Scan(&code); err != nil {
		t.Fatalf("reference code %s: %v", subject, err)
	}
	return code
}

func callPBS(t *testing.T, h *apptest.Harness, kode string, data []byte) apptest.Response {
	t.Helper()
	return h.CallService(t, pbsPath, rpaUser, rpaPassword, map[string]string{"Kode": kode}, "File", "result.zip", data)
}

// formFields reads a multipart request the SLIK fake received.
func formFields(t *testing.T, c apptest.FakeRequest) (map[string]string, map[string]int) {
	t.Helper()
	_, params, err := mime.ParseMediaType(c.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("content type %q", c.Header.Get("Content-Type"))
	}
	mr := multipart.NewReader(strings.NewReader(c.Body), params["boundary"])
	fields, files := map[string]string{}, map[string]int{}
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		var b bytes.Buffer
		_, _ = b.ReadFrom(p)
		if p.FileName() != "" {
			files[p.FormName()+":"+p.FileName()] = b.Len()
		} else {
			fields[p.FormName()] = b.String()
		}
	}
	return fields, files
}

// AC-FR-PSAPI-008-01, -04: the ZIP request is made once; SLIK then SLIK
// Pasangan are sent with the nine fields, different kode reff and the same
// ZIP; the run then waits.
func TestSLIKRequestsCustomerThenSpouse(t *testing.T) {
	h, fs, caseID := slikCase(t, true)
	calls := fs.Calls()
	if len(calls) != 2 {
		t.Fatalf("SLIK calls = %d", len(calls))
	}
	zipName := fmt.Sprintf("Pre-Screening-%s-3171011234567890-%s.zip", caseID, h.Clock.Now().UTC().Add(7*3600e9).Format("20060102"))
	var refs []string
	for i, c := range calls {
		fields, files := formFields(t, c)
		if c.Method != http.MethodPost || c.Path != "/v1/slik/check" || c.Header.Get("x-api-key") != apptest.SLIKAPIKey || len(fields) != 8 ||
			fields["tujuan_permintaan"] != "TP-1" || fields["pilihan_fasilitas"] != "PF-1" || fields["alamat_sesuai_identitas"] != "Jl. Sudirman No. 1, Jakarta" ||
			fields["tipe_pencarian"] != "TC-1" || fields["segmentasi"] != "SG-1" || !strings.HasPrefix(fields["nomor_memo"], "032/AO001/") ||
			files["underlying:"+zipName] == 0 {
			t.Fatalf("call %d: fields %v files %v", i, fields, files)
		}
		refs = append(refs, fields["kode_reff_pengguna"])
		want := map[int]string{0: "3171011234567890", 1: spouseNIK}[i]
		if fields["nomor_identitas"] != want {
			t.Fatalf("call %d identity = %s", i, fields["nomor_identitas"])
		}
	}
	month := h.Clock.Now().UTC().Add(7 * 3600e9)
	prefix := fmt.Sprintf("SKL253/LOS032/%02d/%04d/", int(month.Month()), month.Year())
	if refs[0] == refs[1] || !strings.HasPrefix(refs[0], prefix) || !strings.HasPrefix(refs[1], prefix) {
		t.Fatalf("kode reff = %v", refs)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.case_document WHERE case_id = $1 AND document_type = 'SLIK_ZIP_REQUEST' AND document_category = 'PreScreeningZIPRequest' AND file_name = $2`, caseID, zipName); n != 1 {
		t.Fatalf("ZIP request docs = %d", n)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.slik_report WHERE case_id = $1 AND status = 'REQUESTED' AND requested_at IS NOT NULL`, caseID); n != 2 {
		t.Fatalf("requested reports = %d", n)
	}
	logs := apiLogs(t, h, caseID)
	names := apiNames(logs)
	if names[len(names)-2] != "SLIK Success" || names[len(names)-1] != "SLIKPasangan Success" {
		t.Fatalf("logs = %v", names)
	}
	last := logs[len(logs)-1]
	if !strings.Contains(last.Request, refs[1]) || !strings.Contains(last.Request, "[file "+zipName+", application/zip,") || strings.Contains(last.Request, "PK\x03\x04") {
		t.Fatalf("SLIKPasangan log request = %s", last.Request)
	}
}

// AC-FR-PSAPI-008-02, -03: a 4000 is a Fail retried with the same kode reff
// and no new slik_report; without a Spouse only SLIK runs.
func TestSLIKFailRetriesWithTheSameKodeReff(t *testing.T) {
	h, fs, caseID := slikCase(t, false, apptest.SLIKInvalid, apptest.SLIKAccepted)
	calls := fs.Calls()
	if len(calls) != 2 {
		t.Fatalf("SLIK calls = %d", len(calls))
	}
	a, _ := formFields(t, calls[0])
	b, _ := formFields(t, calls[1])
	if a["kode_reff_pengguna"] != b["kode_reff_pengguna"] || count(t, h, `SELECT count(*) FROM fos.slik_report WHERE case_id = $1`, caseID) != 1 {
		t.Fatalf("kode reff %s / %s", a["kode_reff_pengguna"], b["kode_reff_pengguna"])
	}
	names := apiNames(apiLogs(t, h, caseID))
	if names[len(names)-2] != "SLIK Fail" || names[len(names)-1] != "SLIK Success" {
		t.Fatalf("logs = %v", names)
	}
}

// pbsLogs are the case's SLIKPBSResponse rows: status and response.
func pbsLogs(t *testing.T, h *apptest.Harness, caseID string) []string {
	t.Helper()
	rows, err := h.DB.Query(context.Background(), `
		SELECT api_status || ' ' || COALESCE(response, '') FROM fos.connect_api_log
		WHERE case_id = $1 AND api_name = 'SLIKPBSResponse' AND api_type = 'Service Rest' ORDER BY submitted_at, ctid`, caseID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		out = append(out, s)
	}
	return out
}

// AC-FR-PSAPI-009-01, -04, -06, AC-FR-PSAPI-010-08: the result is stored
// and parsed, the Recap file ignored; Code 0 is answered; the log row
// records the request without the ZIP; the run resumes (Pefindo).
func TestPBSResponseStoresTheResultAndResumes(t *testing.T) {
	h, _, caseID := slikCase(t, false)
	kode := referenceCode(t, h, caseID, "CUSTOMER")

	r := callPBS(t, h, kode, slikZip(t, kode, "3171011234567890", facility("1"), facility("3"), facility("2")))
	if r.Status != http.StatusOK || r.Body["Code"] != "0" || r.Body["Error"] != "false" ||
		r.Body["message"] != "Case "+caseID+" successfully resume to Pipeline Data Entry" {
		t.Fatalf("answer = %d %s", r.Status, r.Raw)
	}
	eventually(t, "the run to resume", func() bool { return run(t, h, caseID).State == "RUNNING" })
	if got := run(t, h, caseID); got.Step != "Pefindo" {
		t.Fatalf("run = %+v", got)
	}
	for sql, want := range map[string]int{
		`SELECT count(*) FROM fos.slik_report WHERE case_id = $1 AND status = 'RECEIVED' AND worst_collect = 3 AND source_file_name = 'KTP_3171011234567890.txt' AND zip_document_id IS NOT NULL AND report_number = 'R000001'`:     1,
		`SELECT count(*) FROM fos.slik_facility f JOIN fos.slik_report r USING (slik_report_id) WHERE r.case_id = $1 AND f.financial_institution = '999' AND f.start_contract_date = '2023-05-09' AND f.data_period = '202609'`:     3,
		`SELECT count(*) FROM fos.slik_facility_history h JOIN fos.slik_facility f USING (facility_id) JOIN fos.slik_report r USING (slik_report_id) WHERE r.case_id = $1`:                                                          72,
		`SELECT count(*) FROM fos.slik_debtor d JOIN fos.slik_report r USING (slik_report_id) WHERE r.case_id = $1 AND d.address = 'Jl. Sudirman'`:                                                                                  1,
		`SELECT count(*) FROM fos.slik_collateral c JOIN fos.slik_facility f USING (facility_id) JOIN fos.slik_report r USING (slik_report_id) WHERE r.case_id = $1`:                                                                3,
		`SELECT count(*) FROM fos.slik_guarantor g JOIN fos.slik_facility f USING (facility_id) JOIN fos.slik_report r USING (slik_report_id) WHERE r.case_id = $1`:                                                                 3,
		`SELECT count(*) FROM fos.work WHERE case_id = $1 AND slik_worst_collect = 3`:                                                                                                                                               1,
		`SELECT count(*) FROM fos.case_document WHERE case_id = $1 AND document_category = 'PreScreeningResponse' AND document_type = 'SLIK_ZIP_CUSTOMER' AND created_by = 'rpa01' AND file_name = replace($2, '/', '-') || '.zip'`: 1,
	} {
		args := []any{caseID}
		if strings.Contains(sql, "$2") {
			args = append(args, kode)
		}
		if n := count(t, h, sql, args...); n != want {
			t.Fatalf("%d for %s, want %d", n, sql, want)
		}
	}
	logs := pbsLogs(t, h, caseID)
	if len(logs) != 1 || !strings.HasPrefix(logs[0], "Success ") || !strings.Contains(logs[0], `"Code":"0"`) {
		t.Fatalf("PBS logs = %v", logs)
	}
	var req, user string
	if err := h.DB.QueryRow(context.Background(), `SELECT request, auth_username FROM fos.connect_api_log WHERE case_id = $1 AND api_name = 'SLIKPBSResponse'`, caseID).Scan(&req, &user); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(req, `"Kode":"`+kode+`"`) || !strings.Contains(req, "[file result.zip, application/zip,") || user != rpaUser {
		t.Fatalf("log request = %s, user %s", req, user)
	}
}

// AC-FR-PSAPI-009-08, -02, -03: a resend after RECEIVED is Code 1 and not
// processed again; an unknown kode is Code 2; a case past its SLIK wait is
// Code 1; every call adds a log row.
func TestPBSResponseRefusals(t *testing.T) {
	h, _, caseID := slikCase(t, false)
	kode := referenceCode(t, h, caseID, "CUSTOMER")
	data := slikZip(t, kode, "3171011234567890", facility("1"))
	callPBS(t, h, kode, data)

	r := callPBS(t, h, kode, data)
	if r.Body["Code"] != "1" || r.Body["Error"] != "true" || r.Body["message"] != "an unexpected error occurred. Reason: SLIK result for "+kode+" has already been received" {
		t.Fatalf("resend = %s", r.Raw)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.slik_facility f JOIN fos.slik_report r USING (slik_report_id) WHERE r.case_id = $1`, caseID); n != 1 {
		t.Fatalf("facilities after resend = %d", n)
	}
	if r := callPBS(t, h, "SKL253/UNKNOWN", data); r.Status != http.StatusOK || r.Body["Code"] != "2" || r.Body["message"] != "Case not found" {
		t.Fatalf("unknown kode = %s", r.Raw)
	}
	if got := pbsLogs(t, h, caseID); len(got) != 2 {
		t.Fatalf("PBS logs = %v", got)
	}

	h2, _, other := slikCase(t, false)
	kode2 := referenceCode(t, h2, other, "CUSTOMER")
	exec(t, h2, `INSERT INTO fos.connect_api_log (log_id, case_id, api_type, api_name, submitted_at, api_status, response) VALUES (gen_random_uuid(), $1, 'Service Rest', 'SLIKPBSResponse', now(), 'Fail', 'Timeout SLIK API No Response')`, other)
	if r := callPBS(t, h2, kode2, slikZip(t, kode2, "3171011234567890", facility("1"))); r.Body["Code"] != "1" {
		t.Fatalf("after timeout = %s", r.Raw)
	}
	if n := count(t, h2, `SELECT count(*) FROM fos.slik_report WHERE case_id = $1 AND status = 'RECEIVED'`, other); n != 0 {
		t.Fatal("processed after timeout")
	}
}

// AC-FR-PSAPI-009-05, DB-RULE-067: wrong credentials, a non-RPA account or
// an unknown path are refused and logged without case or API name.
func TestPBSResponseAuthenticationAndUnknownPaths(t *testing.T) {
	h, _, caseID := slikCase(t, false)
	kode := referenceCode(t, h, caseID, "CUSTOMER")
	data := slikZip(t, kode, "3171011234567890", facility("1"))
	for _, c := range []struct{ user, pass string }{{rpaUser, "wrong"}, {"AO001", "Rahasia#1"}, {"", ""}} {
		r := h.CallService(t, pbsPath, c.user, c.pass, map[string]string{"Kode": kode}, "File", "r.zip", data)
		if r.Status != http.StatusUnauthorized {
			t.Fatalf("%s = %d", c.user, r.Status)
		}
	}
	if r := h.CallService(t, "/api/NBS/v1/Other", rpaUser, rpaPassword, nil, "", "", nil); r.Status != http.StatusNotFound {
		t.Fatalf("unknown path = %d", r.Status)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.slik_report WHERE case_id = $1 AND status = 'RECEIVED'`, caseID); n != 0 {
		t.Fatal("processed without credentials")
	}
	if n := count(t, h, `SELECT count(*) FROM fos.connect_api_log WHERE api_type = 'Service Rest' AND case_id IS NULL AND api_name IS NULL AND http_status_code = 401 AND auth_username = $1`, rpaUser); n != 1 {
		t.Fatalf("401 rows for %s = %d", rpaUser, n)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.connect_api_log WHERE request_headers::text LIKE '%Basic %' OR request_headers::text LIKE '%Rpa#Pass1%'`); n != 0 {
		t.Fatal("credentials stored")
	}
	if n := count(t, h, `SELECT count(*) FROM fos.connect_api_log WHERE http_status_code = 404 AND request_url = '/api/NBS/v1/Other' AND api_status = 'Fail'`); n != 1 {
		t.Fatal("unknown path not logged")
	}
}

// AC-FR-PSAPI-010-01, -04: seven months of collectability 2 with 1-15 days
// rejects at once, before the Spouse's result: Rejected, the run stopped,
// the Rejected List with the SLIK reason, log Rejected.
func TestSLIKRejectsOnCriterionA(t *testing.T) {
	h, _, caseID := slikCase(t, true)
	kode := referenceCode(t, h, caseID, "CUSTOMER")
	r := callPBS(t, h, kode, slikZip(t, kode, "3171011234567890", facility("2", repeat(7, month{"2", "10"})...)))
	if r.Body["Code"] != "0" {
		t.Fatalf("answer = %s", r.Raw)
	}
	if got := run(t, h, caseID); got.State != "STOPPED_REJECTED" || caseStatus(t, h, caseID) != "ResolvedRejected-Prescreening" || currentStage(t, h, caseID) != "Rejected" {
		t.Fatalf("run %+v, status %s", got, caseStatus(t, h, caseID))
	}
	want := `• Reject Pre-screening "SLIK" terhadap kriteria "Kolektibilitas 2 dengan maksimum tunggakan hari 15 (sebanyak 6x) ATAU Kolektabilitas 2 dengan maksimum tunggakan hari di atas 15 (sebanyak 1x)"`
	list := rejectedList(t, h)
	if len(list) != 2 || list[0].Reason != want || list[1].Reason != want {
		t.Fatalf("rejected list = %+v", list)
	}
	if got := pbsLogs(t, h, caseID); len(got) != 1 || !strings.HasPrefix(got[0], "Rejected ") {
		t.Fatalf("PBS logs = %v", got)
	}
}

// AC-FR-PSAPI-010-02, -03: two months over 15 days rejects (the Spouse's
// result names SLIKPasangan); six months 1-15 plus one over 15 does not.
func TestSLIKRejectsOnCriterionBButNotBelowTheCounts(t *testing.T) {
	h, _, caseID := slikCase(t, true)
	kc, ks := referenceCode(t, h, caseID, "CUSTOMER"), referenceCode(t, h, caseID, "SPOUSE")
	notQuite := append(repeat(6, month{"2", "15"}), month{"3", "40"}, month{"", ""})
	if r := callPBS(t, h, kc, slikZip(t, kc, "3171011234567890", facility("2", notQuite...))); r.Body["message"] != "SLIK result for "+kc+" received; waiting for the other result" {
		t.Fatalf("customer answer = %s", r.Raw)
	}
	if got := run(t, h, caseID); got.State != "WAITING_SLIK" {
		t.Fatalf("run after customer = %+v", got)
	}
	callPBS(t, h, ks, slikZip(t, ks, spouseNIK, facility("2", month{"2", "16"}, month{"2", "30"})))
	if caseStatus(t, h, caseID) != "ResolvedRejected-Prescreening" {
		t.Fatalf("status = %s", caseStatus(t, h, caseID))
	}
	for _, r := range rejectedList(t, h) {
		if !strings.HasPrefix(r.Reason, `• Reject Pre-screening "SLIKPasangan" terhadap kriteria`) {
			t.Fatalf("reason = %s", r.Reason)
		}
	}
	if n := count(t, h, `SELECT count(*) FROM fos.slik_report WHERE case_id = $1 AND subject_type = 'SPOUSE' AND worst_collect = 2`, caseID); n != 1 {
		t.Fatal("spouse worst collect")
	}
}

// FR-PSAPI-010 Pemilihan berkas 3: a ZIP without a SLIK file is Code 1 and
// the report keeps waiting, so the RPA may send again.
func TestPBSResponseWithoutASLIKFileKeepsWaiting(t *testing.T) {
	h, _, caseID := slikCase(t, false)
	kode := referenceCode(t, h, caseID, "CUSTOMER")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("Recap_only.txt")
	_, _ = w.Write([]byte("x"))
	_ = zw.Close()
	if r := callPBS(t, h, kode, buf.Bytes()); r.Body["Code"] != "1" {
		t.Fatalf("answer = %s", r.Raw)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.slik_report WHERE case_id = $1 AND status = 'REQUESTED'`, caseID); n != 1 {
		t.Fatal("report left waiting")
	}
	if n := count(t, h, `SELECT count(*) FROM fos.case_document WHERE case_id = $1 AND document_type = 'SLIK_ZIP_CUSTOMER'`, caseID); n != 0 {
		t.Fatal("ZIP stored for a failed result")
	}
	if r := callPBS(t, h, kode, slikZip(t, kode, "3171011234567890")); r.Body["Code"] != "0" {
		t.Fatalf("resend = %s", r.Raw)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.work WHERE case_id = $1 AND slik_worst_collect = 0`, caseID); n != 1 {
		t.Fatal("worst collect without facilities should be 0")
	}
}

// AC-FR-PSAPI-009-07: when the log row cannot be written, the answer stands,
// the data stays, and API_LOG_WRITE_FAILED is in the application log.
func TestPBSLogWriteFailureIsReported(t *testing.T) {
	h, _, caseID := slikCase(t, false)
	kode := referenceCode(t, h, caseID, "CUSTOMER")
	exec(t, h, `ALTER TABLE fos.connect_api_log ADD CONSTRAINT refuse_pbs CHECK (api_name IS DISTINCT FROM 'SLIKPBSResponse') NOT VALID`)
	logs := apptest.CaptureLogs(t)
	if r := callPBS(t, h, kode, slikZip(t, kode, "3171011234567890", facility("1"))); r.Body["Code"] != "0" {
		t.Fatalf("answer = %s", r.Raw)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.slik_report WHERE case_id = $1 AND status = 'RECEIVED'`, caseID); n != 1 {
		t.Fatal("data rolled back")
	}
	eventually(t, "the failure to be logged", func() bool { return strings.Contains(logs.String(), "API_LOG_WRITE_FAILED") })
	if !strings.Contains(logs.String(), kode) || !strings.Contains(logs.String(), caseID) {
		t.Fatalf("log entry lacks kode or case: %s", logs)
	}
}

// FR-PSAPI-010 rule 1 edges: collectability 1 with days past due, or
// collectability 2 with 0 days, in every month does not reject.
func TestSLIKCriteriaNeedCollectabilityTwoAndDaysPastDue(t *testing.T) {
	h, _, caseID := slikCase(t, false)
	kode := referenceCode(t, h, caseID, "CUSTOMER")
	r := callPBS(t, h, kode, slikZip(t, kode, "3171011234567890",
		facility("1", repeat(24, month{"1", "10"})...), facility("1", repeat(24, month{"1", "40"})...), facility("2", repeat(24, month{"2", "0"})...)))
	if r.Body["Code"] != "0" || caseStatus(t, h, caseID) != "Pending-Prescreening" {
		t.Fatalf("answer %s, status %s", r.Raw, caseStatus(t, h, caseID))
	}
}

// AC-FR-PSAPI-009-04: only KTP_<nomor KTP>.txt is a SLIK file; a ZIP with a
// valid JSON Recap file and nothing else has no result.
func TestOnlyTheKTPFileIsRead(t *testing.T) {
	h, _, caseID := slikCase(t, false)
	kode := referenceCode(t, h, caseID, "CUSTOMER")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("Recap_BSIM_9_LOS_1.txt")
	_, _ = w.Write([]byte(`{"header":{},"individual":{}}`))
	_ = zw.Close()
	if r := callPBS(t, h, kode, buf.Bytes()); r.Body["Code"] != "1" {
		t.Fatalf("answer = %s", r.Raw)
	}
}
