package app_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"fos/internal/apptest"
)

// reportWith is a Get Report answer with the given report object and scoring.
func reportWith(code string, report map[string]any, scores ...map[string]any) apptest.FakeReply {
	b, _ := json.Marshal(map[string]any{"code": code, "status": "Success", "message": "ok", "event_id": "e", "report": report, "scoring": scores})
	return apptest.FakeReply{Status: 200, Body: string(b)}
}

// detailedReport has every kind of detail value: text from a number, whole
// numbers as strings, amounts, dates, timestamps, values out of range and
// unreadable ones, nested arrays, elements that are not objects.
func detailedReport() map[string]any {
	return map[string]any{
		"header": map[string]any{"username": "BSIM", "id_report": 1},
		"debitur": map[string]any{
			"npwp": "012345678901000", "location_id": 3171, "kecamatan": "Menteng", "nomor_identitas": "3171011234567890",
			"nama_gadis_ibu_kandung": "SITI", "jml_agunan": "2", "kolektabilitas_terburuk": 3.0, "arrears_days": 99999999999,
			"jml_plafon": "1234567.891", "jml_tunggakan": "123456789012345678", "tanggal_lahir": "1990-01-31", "tgl_tunggakan_terakhir": "31/01/2020",
			"id_debitur_golden_record": json.Number("9007199254740993"),
			"pawn_jml_plafon":          "9999999999999999.996", "jml_saldo_terutang": "99.999", "tgl_buka_fasilitas_terakhir": "2020-01-31T00:00:00",
			"email": true, "debtor_address": "Jl. Rahasia Pribadi 9",
		},
		"fasilitas": []any{
			map[string]any{"plafon": 485000000, "baki_debet": "5000000.5", "id_valuta": "IDR", "id_pelapor": json.Number("4294967297"),
				"kolektabilitas_terburuk": "1", "koletabilitas_terburuk": "2", "tanggal_akad_awal": "2023-05-09",
				"agunan": []any{map[string]any{"jenis": "Tanah"}}, "penjamin": []any{}, "riwayat_fasilitas": nil},
			map[string]any{"plafon": "abc", "keterangan": "kedua"},
		},
		"permintaan_data":           []any{map[string]any{"nama_ljk": "Bank Contoh", "inquiry_date": "2026-10-14 09:30:00", "id_tujuan_permintaan": "1"}},
		"summary_permintaan_data":   []any{map[string]any{"x": 1}, 5},
		"riwayat_identitas_debitur": []any{map[string]any{"id_elemen": 7, "id_pelapor": "8", "id_element": nil}},
	}
}

// AC-FR-PSAPI-017-06, DB-RULE-112, DB-RULE-113, DB-RULE-077 (agreed for
// ticket 35): a report's debtor, facilities, inquiries, inquiry summary and
// identity history are stored with it, values converted per column and NULL
// when unreadable, out of range (after rounding), a boolean or a date that
// is not exactly yyyy-MM-dd; none of it is logged; a new Get Report
// replaces them.
func TestPefindoReportDetailsAreStored(t *testing.T) {
	scripts := pefindoPassScripts()
	scripts[apptest.PefindoReport] = []apptest.FakeReply{reportWith("01", detailedReport(), scoring("2025-07-15", "C2"))}
	logs := apptest.CaptureLogs(t)
	h, _, caseID := pefindoBusiness(t, scripts)
	if got := run(t, h, caseID); got.State != "COMPLETED" {
		t.Fatalf("run = %+v", got)
	}
	// DB-RULE-077: the report's personal data never reaches the application log.
	for _, private := range []string{"Jl. Rahasia Pribadi 9", "SITI", "Menteng"} {
		if strings.Contains(logs.String(), private) {
			t.Fatalf("application log holds %q", private)
		}
	}
	for sql, want := range map[string]int{
		`SELECT count(*) FROM fos.pefindo_debtor d JOIN fos.pefindo_report r USING (pefindo_report_id) WHERE r.case_id = $1
			AND d.npwp = '012345678901000' AND d.location_id = '3171' AND d.subdistrict = 'Menteng' AND d.identity_number = '3171011234567890'
			AND d.mother_maiden_name = 'SITI' AND d.collateral_count = 2 AND d.worst_collectability = 3 AND d.arrears_days IS NULL
			AND d.total_credit_limit = 1234567.89 AND d.total_arrears IS NULL AND d.birth_date = '1990-01-31' AND d.last_arrears_date IS NULL
			AND d.golden_record_debtor_id = 9007199254740993 AND d.email IS NULL AND d.pawn_total_credit_limit IS NULL
			AND d.total_outstanding_balance = 100.00 AND d.last_facility_open_date IS NULL AND d.debtor_address = 'Jl. Rahasia Pribadi 9'`: 1,
		`SELECT count(*) FROM fos.pefindo_facility f JOIN fos.pefindo_report r USING (pefindo_report_id) WHERE r.case_id = $1 AND f.sequence_number = 1
			AND f.plafon = 485000000 AND f.outstanding_principal = 5000000.50 AND f.currency_id = 'IDR' AND f.reporter_id = 4294967297
			AND f.worst_collectability = 1 AND f.worst_collectability_source_variant = 2 AND f.start_contract_date = '2023-05-09'
			AND f.collateral_json = '[{"jenis":"Tanah"}]' AND f.guarantor_json = '[]' AND f.facility_history_json IS NULL`: 1,
		`SELECT count(*) FROM fos.pefindo_facility f JOIN fos.pefindo_report r USING (pefindo_report_id) WHERE r.case_id = $1 AND f.sequence_number = 2
			AND f.plafon IS NULL AND f.keterangan = 'kedua'`: 1,
		`SELECT count(*) FROM fos.pefindo_inquiry i JOIN fos.pefindo_report r USING (pefindo_report_id) WHERE r.case_id = $1 AND i.sequence_number = 1
			AND i.financial_institution_name = 'Bank Contoh' AND i.inquiry_date = '2026-10-14 02:30:00' AND i.inquiry_purpose_id = 1 AND i.reporter_id IS NULL`: 1,
		`SELECT count(*) FROM fos.pefindo_inquiry_summary s JOIN fos.pefindo_report r USING (pefindo_report_id) WHERE r.case_id = $1
			AND s.reporter_count_12_months IS NULL AND s.inquiry_count_12_months IS NULL`: 2,
		`SELECT count(*) FROM fos.pefindo_identity_history x JOIN fos.pefindo_report r USING (pefindo_report_id) WHERE r.case_id = $1
			AND x.sequence_number = 1 AND x.element_id = 7 AND x.reporter_id = 8 AND x.id_element IS NULL AND x.value IS NULL AND x.data_year_month IS NULL`: 1,
	} {
		if n := count(t, h, sql, caseID); n != want {
			t.Fatalf("%d for %s, want %d", n, sql, want)
		}
	}

	// A TL Retry on Get Report replaces the report and its details.
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
	for table, want := range map[string]int{"pefindo_debtor": 1, "pefindo_facility": 2, "pefindo_inquiry": 1, "pefindo_inquiry_summary": 2, "pefindo_identity_history": 1} {
		if n := count(t, h, `SELECT count(*) FROM fos.`+table); n != want {
			t.Fatalf("%s rows after the new Get Report = %d", table, n)
		}
	}
}

// DB-RULE-112, FR-PSAPI-017 rule 2: a code 36 report without debitur or
// fasilitas makes no debtor or facility rows.
func TestPefindoBigReportHasNoFacilities(t *testing.T) {
	scripts := pefindoPassScripts()
	scripts[apptest.PefindoReport] = []apptest.FakeReply{reportWith("36", map[string]any{"header": map[string]any{}, "debitur": nil, "fasilitas": []any{}}, scoring("2025-07-15", "B1"))}
	h, _, caseID := pefindoBusiness(t, scripts)
	if count(t, h, `SELECT count(*) FROM fos.pefindo_report WHERE case_id = $1`, caseID) != 1 ||
		count(t, h, `SELECT count(*) FROM fos.pefindo_debtor`)+count(t, h, `SELECT count(*) FROM fos.pefindo_facility`) != 0 {
		t.Fatal("rows for an empty debitur or fasilitas")
	}
}

// DB-RULE-075: detail rows that cannot be saved fail Get Report like the
// report itself; nothing is stored and the case is not rejected.
func TestPefindoReportDetailsNotSavedFailTheAttempt(t *testing.T) {
	scripts := pefindoPassScripts()
	scripts[apptest.PefindoReport] = []apptest.FakeReply{reportWith("01", detailedReport(), scoring("2025-07-15", "E1"))}
	h, _, caseID := pefindoBusinessWith(t, scripts, func(h *apptest.Harness, caseID string) {
		exec(t, h, `ALTER TABLE fos.pefindo_facility ADD CONSTRAINT refuse CHECK (keterangan IS DISTINCT FROM 'kedua')`)
	})
	names := pefindoNames(t, h, caseID)
	if got := run(t, h, caseID); got.State != "STOPPED_FAILED" || !strings.HasPrefix(strings.Join(names[3:], ","), "PefindoGetReport Fail") ||
		count(t, h, `SELECT count(*) FROM fos.pefindo_report`) != 0 || len(rejectedList(t, h)) != 0 {
		t.Fatalf("run %+v, logs %v", got, names)
	}
}
