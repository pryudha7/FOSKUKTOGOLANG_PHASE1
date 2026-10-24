package app_test

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"fos/internal/apptest"
)

// facilityRequest takes an Individual case through Data Usaha Nasabah and
// Data Pembiayaan Lain to its Data Permohonan Fasilitas (product 794, Qardh,
// limits 0 to 2.000.000.000).
func facilityRequest(t *testing.T) (*apptest.Harness, *apptest.Client, string, int64) {
	t.Helper()
	h, c, caseID, id := hotProspect(t, false)
	oid := toOtherFinancing(t, h, c, caseID, id)
	fid := movedTo(t, h, caseID, submitOtherFinancing(t, c, caseID, oid, "Data Permohonan Fasilitas"), "Data Permohonan Fasilitas")
	h.SeedBMV(t, "DPFHPDE", "MK", "Modal Kerja", 1, "Active")
	h.SeedBMV(t, "DPFHPDE", "INV", "Investasi", 2, "Active")
	return h, c, caseID, fid
}

type facilityRequestForm struct {
	PurposeCode             string `json:"purposeCode"`
	RequestedAmount         string `json:"requestedAmount"`
	TenureMonths            string `json:"tenureMonths"`
	ExistingFinancingAmount string `json:"existingFinancingAmount"`
	MarginTypeCode          string `json:"marginTypeCode"`
	MarginRate              string `json:"marginRate"`
	DeviationNote           string `json:"deviationNote"`
	MitigationNote          string `json:"mitigationNote"`
	Notes                   string `json:"notes"`
}

func validFacilityRequestForm() facilityRequestForm {
	return facilityRequestForm{"MK", "100000000", "12", "0", "EFFECTIVE", "12", "Deviasi", "Mitigasi", "Layak dibiayai"}
}

func submitFacilityRequest(t *testing.T, c *apptest.Client, caseID string, id int64, to string, f any) apptest.Response {
	t.Helper()
	return c.Do(t, http.MethodPost, prospectPath(caseID, id, "facility-request", to), f)
}

type simulationInput struct {
	RequestedAmount string `json:"requestedAmount"`
	TenureMonths    string `json:"tenureMonths"`
	MarginTypeCode  string `json:"marginTypeCode"`
	MarginRate      string `json:"marginRate"`
}

// simulation is the answer of the simulation endpoint, as
// "ready|installmentEffective|installmentFlat|marginEffective|marginFlat".
func simulation(t *testing.T, c *apptest.Client, caseID string, id int64, in simulationInput) string {
	t.Helper()
	r := c.Do(t, http.MethodPost, submitPath(caseID, id, "facility-simulation"), in)
	if r.Status != http.StatusOK {
		t.Fatalf("simulation = %d %s", r.Status, r.Raw)
	}
	b := r.Body
	ready := "no"
	if b["ready"] == true {
		ready = "yes"
	}
	return ready + "|" + b["installmentEffective"].(string) + "|" + b["installmentFlat"].(string) + "|" + b["marginEffective"].(string) + "|" +
		b["marginFlat"].(string)
}

// AC-FR-APP-018-01: Data Permohonan Fasilitas opens empty with Tipe Akad
// from the case's product, its radios and dropdown, and what the Credit
// Simulation names (product and debtor).
func TestDataPermohonanFasilitasPage(t *testing.T) {
	_, c, caseID, id := facilityRequest(t)
	p := pipelinePage(t, c, caseID, id)
	if !reflect.DeepEqual(p["values"], map[string]any{"purposeCode": "", "requestedAmount": "", "tenureMonths": "", "existingFinancingAmount": "",
		"marginTypeCode": "", "marginRate": "", "deviationNote": "", "mitigationNote": "", "notes": ""}) {
		t.Fatalf("values = %v", p["values"])
	}
	if p["editable"] != true || p["contractType"] != "Qardh" || p["productName"] != "testing_mikro_aira_qardh" || p["applicantName"] != "Budi Santoso" {
		t.Fatalf("page = %v", p)
	}
	opts := p["options"].(map[string]any)
	if got := optionTexts(opts["purpose"]); !reflect.DeepEqual(got, []string{"MK=Modal Kerja", "INV=Investasi"}) {
		t.Errorf("purpose = %v", got)
	}
	if got := optionTexts(opts["marginType"]); !reflect.DeepEqual(got, []string{"EFFECTIVE=Effective", "FLAT=Flat"}) {
		t.Errorf("marginType = %v", got)
	}
}

// FR-APP-018 Perhitungan, AC-FR-APP-018-04, -05, -18, -19: the installments
// and the other margin, rounded half up (rupiah, five decimals); zero until
// the four fields are filled and valid. Expected values beyond the AC come
// from an independent decimal computation of the same formulas.
func TestDataPermohonanFasilitasSimulation(t *testing.T) {
	_, c, caseID, id := facilityRequest(t)
	for _, tc := range []struct {
		name string
		in   simulationInput
		want string
	}{
		{"AC-04 effective", simulationInput{"100000000", "12", "EFFECTIVE", "12"}, "yes|8884879|8884879|12.00000|6.61855"},
		{"AC-05 flat", simulationInput{"100000000", "12", "FLAT", "6.61855"}, "yes|8884879|8884879|12.00000|6.61855"},
		{"effective 0", simulationInput{"100000000", "12", "EFFECTIVE", "0"}, "yes|8333333|8333333|0.00000|0.00000"},
		{"flat 0", simulationInput{"100000000", "12", "FLAT", "0"}, "yes|8333333|8333333|0.00000|0.00000"},
		{"flat installment from the rounded flat margin", simulationInput{"9000000000", "12", "EFFECTIVE", "12"}, "yes|799639098|799639125|12.00000|6.61855"},
		{"largest effective", simulationInput{"999999999999999", "9999", "EFFECTIVE", "100"}, "yes|83333333333333|83333335001000|100.00000|99.87999"},
		{"largest flat", simulationInput{"999999999999999", "360", "FLAT", "100"}, "yes|86111108333344|86111111111111|103.33333|100.00000"},
		{"odd flat", simulationInput{"123456789", "37", "FLAT", "7.12345"}, "yes|4069535|4069535|13.02990|7.12345"},
		{"odd effective", simulationInput{"123456789", "37", "EFFECTIVE", "7.12345"}, "yes|3726360|3726360|7.12345|3.78779"},
		// BR-UI-014: a computed margin is never negative (ticket 19 review).
		{"flat installment rounded to 0", simulationInput{"1", "9999", "FLAT", "0.00001"}, "yes|0|0|0.00000|0.00001"},
		{"effective installment rounded down", simulationInput{"100", "3", "EFFECTIVE", "0.00001"}, "yes|33|33|0.00001|0.00000"},
		{"AC-18 no margin", simulationInput{"100000000", "12", "EFFECTIVE", ""}, "no|0|0|0.00000|0.00000"},
		{"AC-18 no type", simulationInput{"100000000", "12", "", "12"}, "no|0|0|0.00000|0.00000"},
		{"AC-18 no tenure", simulationInput{"100000000", "", "FLAT", "12"}, "no|0|0|0.00000|0.00000"},
		{"AC-18 no amount", simulationInput{"", "12", "FLAT", "12"}, "no|0|0|0.00000|0.00000"},
		{"margin over 100", simulationInput{"100000000", "12", "FLAT", "100.00001"}, "no|0|0|0.00000|0.00000"},
		{"six decimals", simulationInput{"100000000", "12", "FLAT", "1.000001"}, "no|0|0|0.00000|0.00000"},
		{"unknown type", simulationInput{"100000000", "12", "X", "12"}, "no|0|0|0.00000|0.00000"},
		{"amount 0", simulationInput{"0", "12", "FLAT", "12"}, "no|0|0|0.00000|0.00000"},
		{"tenure 10000", simulationInput{"100000000", "10000", "FLAT", "12"}, "no|0|0|0.00000|0.00000"},
	} {
		if got := simulation(t, c, caseID, id, tc.in); got != tc.want {
			t.Errorf("%s = %s, want %s", tc.name, got, tc.want)
		}
	}
}

// facilityRows are the case's §9.41, §9.42 and §9.43 rows, "" when absent.
func facilityRows(t *testing.T, h *apptest.Harness, caseID string) (request, financing, simulation string) {
	t.Helper()
	row := func(sql string) string {
		var s *string
		if err := h.DB.QueryRow(context.Background(), sql, caseID).Scan(&s); err != nil || s == nil {
			return ""
		}
		return *s
	}
	request = row(`SELECT string_agg(concat_ws('|', sort_order, purpose_code, requested_amount, tenure_months), ' / ' ORDER BY sort_order)
		FROM fos.case_financing_request WHERE case_id = $1`)
	financing = row(`SELECT concat_ws('|', total_requested_amount, existing_financing_amount, total_obligor, COALESCE(deviation_note, '-'),
		COALESCE(mitigation_note, '-'), COALESCE(officer_notes, '-')) FROM fos.case_financing WHERE case_id = $1`)
	simulation = row(`SELECT concat_ws('|', COALESCE(product_code, '-'), total_obligor, amount, tenure_months, margin_type_code, margin_flat,
		margin_effective, installment_flat, installment_effective) FROM fos.case_credit_simulation WHERE case_id = $1`)
	return request, financing, simulation
}

func facilityRecommendations(t *testing.T, h *apptest.Harness, caseID string) int {
	t.Helper()
	return count(t, h, `SELECT count(*) FROM fos.case_pre_screening_decision WHERE case_id = $1 AND recommendation_type = 'Data Permohonan Fasilitas'`, caseID)
}

func setProductLimits(t *testing.T, h *apptest.Harness, start, end any) {
	t.Helper()
	exec(t, h, `UPDATE fos.product_management SET financing_limit_start = $1, financing_limit_end = $2 WHERE code = '794'`, start, end)
}

// FR-APP-018 Validation, AC-FR-APP-018-09, -12: required fields are ERR-010
// top to bottom; ranges, codes and margins over five decimals or 100 are
// ERR-037; notes over 1000 characters (CRLF counting one) are ERR-040; the
// limits themselves pass; a refused Next stores nothing.
func TestDataPermohonanFasilitasValidation(t *testing.T) {
	h, c, caseID, id := facilityRequest(t)
	var fields []string
	for _, e := range fieldErrors(t, submitFacilityRequest(t, c, caseID, id, "next", facilityRequestForm{})) {
		fields = append(fields, e.Field+" "+e.Code)
	}
	want := []string{"Tujuan Pembiayaan ERR-010", "Pembiayaan yang di ajukan ERR-010", "Jangka Waktu (Bulan) ERR-010",
		"Jumlah Pembiayaan Eksisting di Bank Nano ERR-010", "Tipe Margin ERR-010", "Margin / Ujrah (%) ERR-010", "Notes ERR-010"}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("errors\n got %v\nwant %v", fields, want)
	}
	notesOnly := validFacilityRequestForm()
	notesOnly.Notes = "  "
	if got := fieldErrors(t, submitFacilityRequest(t, c, caseID, id, "next", notesOnly)); !reflect.DeepEqual(got, []fieldError{{"form", "Notes", "ERR-010", "Notes wajib diisi"}}) {
		t.Fatalf("AC-12 = %v", got)
	}
	for _, tc := range []struct {
		name   string
		change func(f *facilityRequestForm)
		field  string
		code   string
	}{
		{"purpose unknown", func(f *facilityRequestForm) { f.PurposeCode = "X" }, "Tujuan Pembiayaan", "ERR-037"},
		{"amount 0", func(f *facilityRequestForm) { f.RequestedAmount = "0" }, "Pembiayaan yang di ajukan", "ERR-037"},
		{"amount 16 digits", func(f *facilityRequestForm) { f.RequestedAmount = "1000000000000000" }, "Pembiayaan yang di ajukan", "ERR-037"},
		{"amount letters", func(f *facilityRequestForm) { f.RequestedAmount = "1e6" }, "Pembiayaan yang di ajukan", "ERR-037"},
		{"tenure 0", func(f *facilityRequestForm) { f.TenureMonths = "0" }, "Jangka Waktu (Bulan)", "ERR-037"},
		{"tenure 10000", func(f *facilityRequestForm) { f.TenureMonths = "10000" }, "Jangka Waktu (Bulan)", "ERR-037"},
		{"existing 16 digits", func(f *facilityRequestForm) { f.ExistingFinancingAmount = "1000000000000000" }, "Jumlah Pembiayaan Eksisting di Bank Nano", "ERR-037"},
		{"margin type unknown", func(f *facilityRequestForm) { f.MarginTypeCode = "X" }, "Tipe Margin", "ERR-037"},
		{"margin over 100", func(f *facilityRequestForm) { f.MarginRate = "100.00001" }, "Margin / Ujrah (%)", "ERR-037"},
		{"margin six decimals", func(f *facilityRequestForm) { f.MarginRate = "12.000001" }, "Margin / Ujrah (%)", "ERR-037"},
		{"margin negative", func(f *facilityRequestForm) { f.MarginRate = "-1" }, "Margin / Ujrah (%)", "ERR-037"},
		{"deviation 1001", func(f *facilityRequestForm) { f.DeviationNote = strings.Repeat("a", 1001) }, "Catatan Deviasi", "ERR-040"},
		{"mitigation 1001", func(f *facilityRequestForm) { f.MitigationNote = strings.Repeat("a", 1001) }, "Catatan Mitigasi", "ERR-040"},
		{"notes 1001", func(f *facilityRequestForm) { f.Notes = strings.Repeat("a", 1001) }, "Notes", "ERR-040"},
	} {
		f := validFacilityRequestForm()
		tc.change(&f)
		got := fieldErrors(t, submitFacilityRequest(t, c, caseID, id, "next", f))
		if len(got) != 1 || got[0].Field != tc.field || got[0].Code != tc.code {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
	if request, financing, sim := facilityRows(t, h, caseID); request != "" || financing != "" || sim != "" || facilityRecommendations(t, h, caseID) != 0 {
		t.Fatalf("refused Next stored %q %q %q", request, financing, sim)
	}
	// The limits: margin 100, 999.999.999.999.999, 9.999 months, 1000
	// characters with CRLF counting one; then margin 0 on Next.
	limits := validFacilityRequestForm()
	limits.MarginRate, limits.TenureMonths = "100", "9999"
	limits.RequestedAmount, limits.ExistingFinancingAmount = "999999999999999", "999999999999999"
	limits.Notes = strings.Repeat("a\r\n", 333) + "a"
	limits.DeviationNote = strings.Repeat("d", 1000)
	if r := submitFacilityRequest(t, c, caseID, id, "save", limits); r.Status != http.StatusOK {
		t.Fatalf("limits = %d %s", r.Status, r.Raw)
	}
	zero := validFacilityRequestForm()
	zero.MarginRate = "0"
	movedTo(t, h, caseID, submitFacilityRequest(t, c, caseID, id, "next", zero), "QCA Data")
}

// AC-FR-APP-018-07, -08, -13, -17, FR-APP-018 Process 8, Rekomendasi: Next
// stores §9.41-9.43 with the server's totals and simulation, adds one
// "Data Permohonan Fasilitas" recommendation (diajukan from the screen,
// direkomendasi and disetujui 0) with its §9.44 simulation, completes the
// assignment and opens QCA Data. 9.000.000.000 equal to the upper limit
// passes and is kept whole.
func TestDataPermohonanFasilitasNext(t *testing.T) {
	h, c, caseID, id := facilityRequest(t)
	setProductLimits(t, h, 100000000, 9000000000)
	f := facilityRequestForm{"INV", "9000000000", "12", "50000000", "EFFECTIVE", "12", "Deviasi\r\nbaris dua", "Mitigasi", "Layak dibiayai"}
	movedTo(t, h, caseID, submitFacilityRequest(t, c, caseID, id, "next", f), "QCA Data")
	if !processStarted(t, h, caseID, "QCA Data") {
		t.Fatal("process QCA Data not started")
	}
	if a := assignmentsNamed(t, h, caseID, "Data Permohonan Fasilitas"); a[0].Completed == nil {
		t.Fatalf("Data Permohonan Fasilitas = %+v", a)
	}
	request, financing, sim := facilityRows(t, h, caseID)
	if request != "1|INV|9000000000|12" || financing != "9000000000|50000000|9050000000|Deviasi\nbaris dua|Mitigasi|Layak dibiayai" ||
		sim != "794|9050000000|9000000000|12|EFFECTIVE|6.61855|12.00000|799639125|799639098" {
		t.Fatalf("rows\n%s\n%s\n%s", request, financing, sim)
	}
	var rec, recSim string
	if err := h.DB.QueryRow(context.Background(), `
		SELECT concat_ws('|', d.recommendation_type, COALESCE(d.decision_code, '-'), d.notes, d.decided_by, d.decided_by_role_code,
		       d.recommendation_status, d.limit_action_code, d.total_obligor, d.requested_amount, d.recommended_amount, d.approved_amount,
		       d.requested_margin_flat, d.requested_margin_effective, d.recommended_margin_flat, d.recommended_margin_effective,
		       d.approved_margin_flat, d.approved_margin_effective, d.requested_tenure, d.recommended_tenure, d.approved_tenure),
		       concat_ws('|', s.purpose_code, s.debtor_name, s.product_code, s.total_obligor, s.amount, s.tenure_months, s.margin_type_code,
		       s.margin_flat, s.margin_effective, s.installment_flat, s.installment_effective)
		FROM fos.case_pre_screening_decision d JOIN fos.recommendation_credit_simulation s ON s.decision_id = d.decision_id
		WHERE d.case_id = $1`, caseID).Scan(&rec, &recSim); err != nil {
		t.Fatal(err)
	}
	if rec != "Data Permohonan Fasilitas|-|Layak dibiayai|AO001|AO|RECOMMEND|SESUAILMT|9050000000|9000000000|0|0|6.61855|12.00000|0.00000|0.00000|0.00000|0.00000|12|0|0" {
		t.Errorf("recommendation = %s", rec)
	}
	if recSim != "INV|Budi Santoso|794|9050000000|9000000000|12|EFFECTIVE|6.61855|12.00000|799639125|799639098" {
		t.Errorf("recommendation simulation = %s", recSim)
	}
	// FR-APP-018 Process 1, agreed comment 9: reopened as saved.
	exec(t, h, `UPDATE fos.assignments SET assignment_status = 'sudah dibuka', completed_at = NULL WHERE assignment_id = $1`, id)
	want := map[string]any{"purposeCode": "INV", "requestedAmount": "9000000000", "tenureMonths": "12", "existingFinancingAmount": "50000000",
		"marginTypeCode": "EFFECTIVE", "marginRate": "12.00000", "deviationNote": "Deviasi\nbaris dua", "mitigationNote": "Mitigasi", "notes": "Layak dibiayai"}
	if got := pipelinePage(t, c, caseID, id)["values"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("reopened = %v", got)
	}
}

// AC-FR-APP-018-15, -16, -17, -20, ERR-042, ERR-043: the Total Requested
// Amount within the product's limits, the limits themselves accepted; a
// limit that cannot be read is ERR-043; a refused Next stores nothing.
func TestDataPermohonanFasilitasProductLimits(t *testing.T) {
	h, c, caseID, id := facilityRequest(t)
	below := fieldError{"form", "Pembiayaan yang di ajukan", "ERR-042", "Total nilai pengajuan tidak memenuhi batas bawah limit pengajuan."}
	above := fieldError{"form", "Pembiayaan yang di ajukan", "ERR-042", "Total nilai pengajuan tidak memenuhi batas atas limit pengajuan."}
	unknown := fieldError{"form", "", "ERR-043", "Gagal mendapatkan limit pembiayaan produk, harap coba lagi atau hubungi helpdesk jika berulang."}
	f := validFacilityRequestForm()
	for _, tc := range []struct {
		name       string
		start, end any
		amount     string
		want       fieldError
	}{
		{"AC-15 below", 5000000000, 7000000000, "100000000", below},
		{"AC-16 above", 0, 2000000000, "3000000000", above},
		{"no start", nil, 2000000000, "100000000", unknown},
		{"no end", 0, nil, "100000000", unknown},
	} {
		setProductLimits(t, h, tc.start, tc.end)
		f.RequestedAmount = tc.amount
		if got := fieldErrors(t, submitFacilityRequest(t, c, caseID, id, "next", f)); !reflect.DeepEqual(got, []fieldError{tc.want}) {
			t.Errorf("%s = %v", tc.name, got)
		}
	}
	setProductLimits(t, h, 0, 2000000000)
	exec(t, h, `UPDATE fos.work SET product_code = 'none' WHERE case_id = $1`, caseID)
	if got := fieldErrors(t, submitFacilityRequest(t, c, caseID, id, "next", f)); !reflect.DeepEqual(got, []fieldError{unknown}) {
		t.Errorf("no product = %v", got)
	}
	if request, financing, sim := facilityRows(t, h, caseID); request != "" || financing != "" || sim != "" || facilityRecommendations(t, h, caseID) != 0 {
		t.Fatalf("refused Next stored %q %q %q", request, financing, sim)
	}
	exec(t, h, `UPDATE fos.work SET product_code = '794' WHERE case_id = $1`, caseID)
	setProductLimits(t, h, 100000000, 200000000)
	f.RequestedAmount = "100000000"
	movedTo(t, h, caseID, submitFacilityRequest(t, c, caseID, id, "next", f), "QCA Data")
}

// Agreed comments 6 and 9, DB-RULE-119: Save checks formats but no required
// field nor the product limit, stays, records no recommendation, and writes
// each table only when its inputs are complete; the totals follow a changed
// amount; no total is accepted from the client.
func TestDataPermohonanFasilitasSave(t *testing.T) {
	h, c, caseID, id := facilityRequest(t)
	if r := submitFacilityRequest(t, c, caseID, id, "save", map[string]any{"requestedAmount": "1", "totalObligor": "5"}); r.Status != http.StatusBadRequest {
		t.Fatalf("client total = %d", r.Status)
	}
	bad := facilityRequestForm{TenureMonths: "abc"}
	if got := fieldErrors(t, submitFacilityRequest(t, c, caseID, id, "save", bad)); len(got) != 1 || got[0].Field != "Jangka Waktu (Bulan)" {
		t.Fatalf("save formats = %v", got)
	}
	save := func(f facilityRequestForm) {
		t.Helper()
		if r := submitFacilityRequest(t, c, caseID, id, "save", f); r.Status != http.StatusOK {
			t.Fatalf("save = %d %s", r.Status, r.Raw)
		}
	}
	save(facilityRequestForm{RequestedAmount: "3000000000", Notes: "catatan"})
	if request, financing, sim := facilityRows(t, h, caseID); request != "" || financing != "" || sim != "" {
		t.Fatalf("incomplete save stored %q %q %q", request, financing, sim)
	}
	// §9.43 follows §9.41: none without Tujuan Pembiayaan (ticket 19 review).
	save(facilityRequestForm{RequestedAmount: "100000000", TenureMonths: "12", ExistingFinancingAmount: "0", MarginTypeCode: "FLAT", MarginRate: "6.61855"})
	if request, _, sim := facilityRows(t, h, caseID); request != "" || sim != "" {
		t.Fatalf("simulation without a request line %q %q", request, sim)
	}
	save(facilityRequestForm{PurposeCode: "MK", RequestedAmount: "3000000000", TenureMonths: "24", ExistingFinancingAmount: "1000", Notes: "catatan"})
	request, financing, sim := facilityRows(t, h, caseID)
	if request != "1|MK|3000000000|24" || financing != "3000000000|1000|3000001000|-|-|catatan" || sim != "" {
		t.Fatalf("after save %q %q %q", request, financing, sim)
	}
	save(facilityRequestForm{PurposeCode: "MK", RequestedAmount: "100000000", TenureMonths: "12", MarginTypeCode: "FLAT", MarginRate: "6.61855"})
	request, financing, sim = facilityRows(t, h, caseID)
	if request != "1|MK|100000000|12" || financing != "100000000|1000|100001000|-|-|catatan" || sim != "" {
		t.Fatalf("totals not following %q %q %q", request, financing, sim)
	}
	save(facilityRequestForm{PurposeCode: "MK", RequestedAmount: "100000000", TenureMonths: "12", ExistingFinancingAmount: "0", MarginTypeCode: "FLAT",
		MarginRate: "6.61855"})
	if _, _, sim := facilityRows(t, h, caseID); sim != "794|100000000|100000000|12|FLAT|6.61855|12.00000|8884879|8884879" {
		t.Fatalf("simulation = %q", sim)
	}
	if facilityRecommendations(t, h, caseID) != 0 || len(assignmentsNamed(t, h, caseID, "QCA Data")) != 0 {
		t.Fatal("save moved or recommended")
	}
	if v := pipelinePage(t, c, caseID, id)["values"].(map[string]any); v["marginTypeCode"] != "FLAT" || v["marginRate"] != "6.61855" || v["notes"] != "" {
		t.Fatalf("reopened = %v", v)
	}
}

// Agreed comment 7: Back to Previous Step and the breadcrumb validate every
// field and save, without the product limit or a recommendation; a step is
// no target of itself. The steps after this one open by breadcrumb only
// once Next has recorded the recommendation (ticket 19 review).
func TestDataPermohonanFasilitasMoves(t *testing.T) {
	h, c, caseID, id := facilityRequest(t)
	if r := submitFacilityRequest(t, c, caseID, id, "Data Permohonan Fasilitas", validFacilityRequestForm()); r.Status != http.StatusBadRequest {
		t.Fatalf("to itself = %d", r.Status)
	}
	if got := fieldErrors(t, submitFacilityRequest(t, c, caseID, id, "Data Pembiayaan Lain", facilityRequestForm{})); len(got) != 7 {
		t.Fatalf("back without fields = %v", got)
	}
	f := validFacilityRequestForm()
	f.RequestedAmount = "5000000000"
	oid := movedTo(t, h, caseID, submitFacilityRequest(t, c, caseID, id, "Data Pembiayaan Lain", f), "Data Pembiayaan Lain")
	request, financing, sim := facilityRows(t, h, caseID)
	if request != "1|MK|5000000000|12" || financing == "" || sim == "" || facilityRecommendations(t, h, caseID) != 0 {
		t.Fatalf("back stored %q %q %q", request, financing, sim)
	}
	fid := movedTo(t, h, caseID, submitOtherFinancing(t, c, caseID, oid, "Data Permohonan Fasilitas"), "Data Permohonan Fasilitas")
	if r := submitFacilityRequest(t, c, caseID, fid, "Collateral Data", validFacilityRequestForm()); r.Status != http.StatusBadRequest {
		t.Fatalf("past this step before Next = %d %s", r.Status, r.Raw)
	}
	if p := pipelinePage(t, c, caseID, fid); p["facilityRequested"] != false {
		t.Fatalf("facilityRequested = %v", p["facilityRequested"])
	}
	movedTo(t, h, caseID, submitFacilityRequest(t, c, caseID, fid, "next", validFacilityRequestForm()), "QCA Data")
	exec(t, h, `UPDATE fos.assignments SET assignment_status = 'sudah dibuka', completed_at = NULL WHERE assignment_id = $1`, fid)
	if p := pipelinePage(t, c, caseID, fid); p["facilityRequested"] != true {
		t.Fatalf("facilityRequested after Next = %v", p["facilityRequested"])
	}
	movedTo(t, h, caseID, submitFacilityRequest(t, c, caseID, fid, "Collateral Data", validFacilityRequestForm()), "Collateral Data")
	if facilityRecommendations(t, h, caseID) != 1 {
		t.Fatal("breadcrumb recommended")
	}
}
