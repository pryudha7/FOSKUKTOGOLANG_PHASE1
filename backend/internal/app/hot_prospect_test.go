package app_test

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"fos/internal/apptest"
)

// hotProspect takes a case through Prospect Data Entry to its Data Usaha
// Nasabah: an Individual (with a received customer SLIK) or a Badan Usaha.
func hotProspect(t *testing.T, badanUsaha bool) (*apptest.Harness, *apptest.Client, string, int64) {
	t.Helper()
	var (
		h      *apptest.Harness
		c      *apptest.Client
		caseID string
	)
	if badanUsaha {
		var id int64
		h, c, caseID, id = businessEntity(t)
		if r := submitBusiness(t, c, caseID, id, validBusinessForm(h)); r.Status != http.StatusOK {
			t.Fatalf("badan usaha = %d %s", r.Status, r.Raw)
		}
	} else {
		var id int64
		h, c, caseID, id = prospect(t, false)
		fid := movedTo(t, h, caseID, submitIndividualData(t, c, caseID, id, "Data Keluarga Dekat", validIndividualDataForm("MS-1")), "Data Keluarga Dekat")
		r := c.Do(t, http.MethodPost, prospectPath(caseID, fid, "close-relative", "create"),
			closeRelativeForm{"Andi", "FR1", "Jl. Saudara 4", "", "", "31", "3171", "317101", "3171011", "0814", "ED1"})
		if r.Status != http.StatusOK {
			t.Fatalf("create = %d %s", r.Status, r.Raw)
		}
	}
	h.SeedBMV(t, "BUSINESSCATEGORY", "BC1", "Perdagangan", 1, "Active")
	h.SeedBMV(t, "BUSINESSLOCATION", "BL1", "Ruko", 1, "Active")
	return h, c, caseID, assignmentsNamed(t, h, caseID, "Data Usaha Nasabah")[0].ID
}

type businessDataForm struct {
	BusinessField        string `json:"businessField"`
	BusinessCategoryCode string `json:"businessCategoryCode"`
	BusinessAgeMonths    string `json:"businessAgeMonths"`
	EmployeeCount        string `json:"employeeCount"`
	BusinessLocationCode string `json:"businessLocationCode"`
	Address              string `json:"address"`
	HouseOwnershipCode   string `json:"houseOwnershipCode"`
	YearsOfStay          string `json:"yearsOfStay"`
	MonthsOfStay         string `json:"monthsOfStay"`
	Phone                string `json:"phone"`
	RT                   string `json:"rt"`
	RW                   string `json:"rw"`
	ProvinceCode         string `json:"provinceCode"`
	CityCode             string `json:"cityCode"`
	DistrictCode         string `json:"districtCode"`
	SubdistrictCode      string `json:"subdistrictCode"`
}

func validBusinessDataForm() businessDataForm {
	return businessDataForm{"Toko Kelontong", "BC1", "99999", "9999999", "BL1", "Jl. Pasar 3", "HO1", "999", "11", "021-777", "007", "008",
		"31", "3171", "317101", "3171011"}
}

func submitBusinessData(t *testing.T, c *apptest.Client, caseID string, id int64, to string, f any) apptest.Response {
	t.Helper()
	return c.Do(t, http.MethodPost, prospectPath(caseID, id, "business-data", to), f)
}

// applicantID is the customer_id of the case's applicant (Nasabah or Badan Usaha).
func applicantID(t *testing.T, h *apptest.Harness, caseID string) int64 {
	t.Helper()
	var id int64
	if err := h.DB.QueryRow(context.Background(), `SELECT customer_id FROM fos.customer WHERE case_id = $1 AND customer_type IN ('Nasabah', 'Badan Usaha')`, caseID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func businessRow(t *testing.T, h *apptest.Harness, customerID int64) string {
	t.Helper()
	var s string
	err := h.DB.QueryRow(context.Background(), `
		SELECT concat_ws('|', business_field, business_category_code, business_age_months, employee_count, business_location_code)
		FROM fos.customer_business WHERE customer_id = $1`, customerID).Scan(&s)
	if err != nil {
		return ""
	}
	return s
}

// AC-FR-APP-016-01: Data Usaha Nasabah opens empty with its dropdowns.
func TestDataUsahaNasabahPage(t *testing.T) {
	_, c, caseID, id := hotProspect(t, false)
	p := pipelinePage(t, c, caseID, id)
	v := p["values"].(map[string]any)
	if p["editable"] != true || v["businessField"] != "" || v["provinceCode"] != "" {
		t.Fatalf("page = %v", p)
	}
	opts := p["options"].(map[string]any)
	for key, want := range map[string]string{"businessCategory": "BC1=Perdagangan", "businessLocation": "BL1=Ruko", "houseOwnership": "HO1=Milik Sendiri", "province": "31=DKI Jakarta"} {
		if got := optionTexts(opts[key]); len(got) == 0 || got[0] != want {
			t.Errorf("%s = %v", key, got)
		}
	}
}

// AC-FR-APP-016-02, -03, FR-APP-016 Validation: required fields (all but
// Nomor Telepon) are ERR-010 top to bottom; out of range is ERR-037 and the
// limits themselves pass.
func TestDataUsahaNasabahValidation(t *testing.T) {
	h, c, caseID, id := hotProspect(t, false)
	got := fieldErrors(t, submitBusinessData(t, c, caseID, id, "Data Pembiayaan Lain", businessDataForm{}))
	var fields []string
	for _, e := range got {
		fields = append(fields, e.Field+" "+e.Code)
	}
	want := []string{"Jenis Bidang Usaha ERR-010", "Kategori Bidang Usaha ERR-010", "Lama Usaha (Bulan) ERR-010", "Jumlah Karyawan ERR-010",
		"Jenis Tempat Usaha ERR-010", "Alamat Usaha ERR-010", "Status Kepemilikan Tempat Usaha ERR-010", "Lama Tempat Usaha (Tahun) ERR-010",
		"Lama Tempat Usaha (Bulan) ERR-010", "RT ERR-010", "RW ERR-010", "Provinsi ERR-010", "Kota ERR-010", "Kecamatan ERR-010", "Kelurahan ERR-010"}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("empty form\n got %v\nwant %v", fields, want)
	}
	for _, tc := range []struct {
		name   string
		change func(f *businessDataForm)
		want   fieldError
	}{
		{"age", func(f *businessDataForm) { f.BusinessAgeMonths = "100000" }, fieldError{"form", "Lama Usaha (Bulan)", "ERR-037", "Harap masukkan Lama Usaha (Bulan) yang valid."}},
		{"employees", func(f *businessDataForm) { f.EmployeeCount = "10000000" }, fieldError{"form", "Jumlah Karyawan", "ERR-037", "Harap masukkan Jumlah Karyawan yang valid."}},
		{"years", func(f *businessDataForm) { f.YearsOfStay = "1000" }, fieldError{"form", "Lama Tempat Usaha (Tahun)", "ERR-037", "Harap masukkan Lama Tempat Usaha (Tahun) yang valid."}},
		{"months", func(f *businessDataForm) { f.MonthsOfStay = "12" }, fieldError{"form", "Lama Tempat Usaha (Bulan)", "ERR-037", "Harap masukkan Lama Tempat Usaha (Bulan) yang valid."}},
		{"not whole", func(f *businessDataForm) { f.EmployeeCount = "1.5" }, fieldError{"form", "Jumlah Karyawan", "ERR-037", "Harap masukkan Jumlah Karyawan yang valid."}},
		{"field length", func(f *businessDataForm) { f.BusinessField = strings.Repeat("a", 101) }, fieldError{"form", "Jenis Bidang Usaha", "ERR-040", "Jenis Bidang Usaha maksimal 100 karakter"}},
		{"category", func(f *businessDataForm) { f.BusinessCategoryCode = "X" }, fieldError{"form", "Kategori Bidang Usaha", "ERR-037", "Harap masukkan Kategori Bidang Usaha yang valid."}},
		{"phone", func(f *businessDataForm) { f.Phone = "021 777" }, fieldError{"form", "Nomor Telepon", "ERR-037", "Harap masukkan Nomor Telepon yang valid."}},
		{"rw", func(f *businessDataForm) { f.RW = "0012" }, fieldError{"form", "RW", "ERR-040", "RW maksimal 3 karakter"}},
	} {
		f := validBusinessDataForm()
		tc.change(&f)
		if got := fieldErrors(t, submitBusinessData(t, c, caseID, id, "Data Pembiayaan Lain", f)); !reflect.DeepEqual(got, []fieldError{tc.want}) {
			t.Errorf("%s: errors %v, want %v", tc.name, got, tc.want)
		}
	}
	if businessRow(t, h, applicantID(t, h, caseID)) != "" {
		t.Fatal("a refused Next stored")
	}
}

// AC-FR-APP-016-04, FR-APP-016 rule 1, DB-RULE-105/106: Next stores the
// business (codes only) and the Badan Usaha's address type "Usaha", then
// Data Pembiayaan Lain opens at once.
func TestDataUsahaNasabahNextForABusinessEntity(t *testing.T) {
	h, c, caseID, id := hotProspect(t, true)
	movedTo(t, h, caseID, submitBusinessData(t, c, caseID, id, "Data Pembiayaan Lain", validBusinessDataForm()), "Data Pembiayaan Lain")
	cid := applicantID(t, h, caseID)
	if got := businessRow(t, h, cid); got != "Toko Kelontong|BC1|99999|9999999|BL1" {
		t.Fatalf("customer_business = %s", got)
	}
	if got := addressOf(t, h, cid, "Usaha"); got != "Jl. Pasar 3|021-777|||007|008|999|11|HO1|31|3171|JAKSEL|317101|3171011|12850" {
		t.Fatalf("Usaha address = %s", got)
	}
	if got := assignmentsNamed(t, h, caseID, "Data Usaha Nasabah"); got[0].Status != "sudah selesai dikerjakan" {
		t.Fatalf("Data Usaha Nasabah = %+v", got)
	}
}

// FR-APP-016 rule 1: for a Perorangan the address type "Usaha" belongs to
// the Nasabah row.
func TestDataUsahaNasabahNextForAnIndividual(t *testing.T) {
	h, c, caseID, id := hotProspect(t, false)
	movedTo(t, h, caseID, submitBusinessData(t, c, caseID, id, "Data Pembiayaan Lain", validBusinessDataForm()), "Data Pembiayaan Lain")
	nasabah, _ := customerOf(t, h, caseID, "Nasabah")
	if got := addressOf(t, h, nasabah.ID, "Usaha"); !strings.HasPrefix(got, "Jl. Pasar 3|") {
		t.Fatalf("Usaha address of Nasabah = %q", got)
	}
	if got := businessRow(t, h, nasabah.ID); got != "Toko Kelontong|BC1|99999|9999999|BL1" {
		t.Fatalf("customer_business = %s", got)
	}
}

// FR-APP-016 Process 4, agreed comments 2 and 16: Save keeps what is there
// without the required check and stays; customer_business only when its
// five fields are filled; formats are still checked.
func TestDataUsahaNasabahSave(t *testing.T) {
	h, c, caseID, id := hotProspect(t, false)
	partial := businessDataForm{BusinessField: "Toko", Address: "Jl. Sebagian", BusinessAgeMonths: "12"}
	r := submitBusinessData(t, c, caseID, id, "save", partial)
	if r.Status != http.StatusOK || r.Body["assignmentId"] != float64(0) {
		t.Fatalf("save = %d %s", r.Status, r.Raw)
	}
	cid := applicantID(t, h, caseID)
	if businessRow(t, h, cid) != "" || !strings.HasPrefix(addressOf(t, h, cid, "Usaha"), "Jl. Sebagian|") {
		t.Fatalf("partial save: business %q, address %q", businessRow(t, h, cid), addressOf(t, h, cid, "Usaha"))
	}
	if got := assignmentsNamed(t, h, caseID, "Data Usaha Nasabah"); got[0].Status == "sudah selesai dikerjakan" || len(assignmentsNamed(t, h, caseID, "Data Pembiayaan Lain")) != 0 {
		t.Fatal("save moved the case")
	}
	if got := fieldErrors(t, submitBusinessData(t, c, caseID, id, "save", businessDataForm{RT: "1a"})); !reflect.DeepEqual(got, []fieldError{{"form", "RT", "ERR-037", "Harap masukkan RT yang valid."}}) {
		t.Fatalf("save formats = %v", got)
	}
	almost := partial
	almost.BusinessCategoryCode, almost.EmployeeCount = "BC1", "3"
	if r := submitBusinessData(t, c, caseID, id, "save", almost); r.Status != http.StatusOK || businessRow(t, h, cid) != "" {
		t.Fatalf("save without Jenis Tempat Usaha = %d, business %q", r.Status, businessRow(t, h, cid))
	}
	full := partial
	full.BusinessCategoryCode, full.EmployeeCount, full.BusinessLocationCode = "BC1", "3", "BL1"
	if r := submitBusinessData(t, c, caseID, id, "save", full); r.Status != http.StatusOK {
		t.Fatalf("save = %d %s", r.Status, r.Raw)
	}
	if got := businessRow(t, h, cid); got != "Toko|BC1|12|3|BL1" {
		t.Fatalf("customer_business = %s", got)
	}
	v := pipelinePage(t, c, caseID, id)["values"].(map[string]any)
	if v["businessField"] != "Toko" || v["employeeCount"] != "3" || v["address"] != "Jl. Sebagian" {
		t.Fatalf("reopened = %v", v)
	}
}

type otherFinancingCard struct {
	BankName          string `json:"bankName"`
	FinancingType     string `json:"financingType"`
	FinancingAmount   string `json:"financingAmount"`
	OutstandingAmount string `json:"outstandingAmount"`
	TenorMonths       string `json:"tenorMonths"`
	MarginRate        string `json:"marginRate"`
	SLIKCollect       string `json:"slikCollect"`
	Purpose           string `json:"purpose"`
}

type otherFinancingForm struct {
	OtherFinancings []otherFinancingCard `json:"otherFinancings"`
}

func toOtherFinancing(t *testing.T, h *apptest.Harness, c *apptest.Client, caseID string, id int64) int64 {
	t.Helper()
	return movedTo(t, h, caseID, submitBusinessData(t, c, caseID, id, "Data Pembiayaan Lain", validBusinessDataForm()), "Data Pembiayaan Lain")
}

func submitOtherFinancing(t *testing.T, c *apptest.Client, caseID string, id int64, to string, cards ...otherFinancingCard) apptest.Response {
	t.Helper()
	return c.Do(t, http.MethodPost, prospectPath(caseID, id, "other-financing", to), otherFinancingForm{cards})
}

func otherFinancingRows(t *testing.T, h *apptest.Harness, caseID string) []string {
	t.Helper()
	rows, err := h.DB.Query(context.Background(), `
		SELECT concat_ws('|', sort_order, COALESCE(bank_name, ''), COALESCE(financing_type, ''), COALESCE(financing_amount::text, ''),
		       COALESCE(outstanding_amount::text, ''), COALESCE(tenor_months::text, ''), COALESCE(margin_rate::text, ''),
		       COALESCE(slik_collect::text, ''), COALESCE(purpose, ''))
		FROM fos.other_financing WHERE case_id = $1 ORDER BY sort_order`, caseID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

// AC-FR-APP-017-01, -02, -04, -05: one empty card at first; an all-empty
// card is dropped; Outstanding over the amount passes; Margin 0 to 100 with
// two decimals and SLIK Collect 0 to 5; Next opens Data Permohonan Fasilitas.
func TestDataPembiayaanLainNext(t *testing.T) {
	h, c, caseID, id := hotProspect(t, false)
	oid := toOtherFinancing(t, h, c, caseID, id)
	p := pipelinePage(t, c, caseID, oid)
	cards := p["values"].(map[string]any)["otherFinancings"].([]any)
	if len(cards) != 1 || cards[0].(map[string]any)["bankName"] != "" || p["slikAvailable"] != true {
		t.Fatalf("page = %v", p)
	}
	full := otherFinancingCard{"Bank Lain", "KPR", "100000000", "120000000", "120", "21.5", "5", "Rumah"}
	for _, tc := range []struct {
		name   string
		change func(c *otherFinancingCard)
		want   fieldError
	}{
		{"margin over 100", func(c *otherFinancingCard) { c.MarginRate = "100.01" }, fieldError{"other-financing-1", "Margin / Ujrah (%)", "ERR-037", "Harap masukkan Margin / Ujrah (%) yang valid."}},
		{"margin three decimals", func(c *otherFinancingCard) { c.MarginRate = "21.123" }, fieldError{"other-financing-1", "Margin / Ujrah (%)", "ERR-037", "Harap masukkan Margin / Ujrah (%) yang valid."}},
		{"margin letters", func(c *otherFinancingCard) { c.MarginRate = "abc" }, fieldError{"other-financing-1", "Margin / Ujrah (%)", "ERR-037", "Harap masukkan Margin / Ujrah (%) yang valid."}},
		{"collect 6", func(c *otherFinancingCard) { c.SLIKCollect = "6" }, fieldError{"other-financing-1", "SLIK Collect", "ERR-037", "Harap masukkan SLIK Collect yang valid."}},
		{"tenor", func(c *otherFinancingCard) { c.TenorMonths = "10000" }, fieldError{"other-financing-1", "Jangka Waktu (Bulan)", "ERR-037", "Harap masukkan Jangka Waktu (Bulan) yang valid."}},
		{"amount 16 digits", func(c *otherFinancingCard) { c.FinancingAmount = "1000000000000000" }, fieldError{"other-financing-1", "Jumlah Pembiayaan", "ERR-037", "Harap masukkan Jumlah Pembiayaan yang valid."}},
		{"bank length", func(c *otherFinancingCard) { c.BankName = strings.Repeat("a", 101) }, fieldError{"other-financing-1", "Nama Bank", "ERR-040", "Nama Bank maksimal 100 karakter"}},
		{"purpose length", func(c *otherFinancingCard) { c.Purpose = strings.Repeat("a", 501) }, fieldError{"other-financing-1", "Tujuan", "ERR-040", "Tujuan maksimal 500 karakter"}},
	} {
		card := full
		tc.change(&card)
		if got := fieldErrors(t, submitOtherFinancing(t, c, caseID, oid, "Data Permohonan Fasilitas", card)); !reflect.DeepEqual(got, []fieldError{tc.want}) {
			t.Errorf("%s: errors %v, want %v", tc.name, got, tc.want)
		}
	}
	zero := otherFinancingCard{MarginRate: "0", SLIKCollect: "0"}
	hundred := otherFinancingCard{MarginRate: "100"}
	movedTo(t, h, caseID, submitOtherFinancing(t, c, caseID, oid, "Data Permohonan Fasilitas", full, otherFinancingCard{}, zero, hundred), "Data Permohonan Fasilitas")
	want := []string{"1|Bank Lain|KPR|100000000|120000000|120|21.50|5|Rumah", "2||||||0.00|0|", "3||||||100.00||"}
	if got := otherFinancingRows(t, h, caseID); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows\n got %v\nwant %v", got, want)
	}
}

// AC-FR-APP-017-03, DB-RULE-107: Next with no cards removes every row;
// Save replaces the rows and stays.
func TestDataPembiayaanLainNoCardsAndSave(t *testing.T) {
	h, c, caseID, id := hotProspect(t, false)
	oid := toOtherFinancing(t, h, c, caseID, id)
	if r := submitOtherFinancing(t, c, caseID, oid, "save", otherFinancingCard{BankName: "Bank A"}, otherFinancingCard{BankName: "Bank B"}); r.Status != http.StatusOK {
		t.Fatalf("save = %d %s", r.Status, r.Raw)
	}
	if got := otherFinancingRows(t, h, caseID); len(got) != 2 || len(assignmentsNamed(t, h, caseID, "Data Permohonan Fasilitas")) != 0 {
		t.Fatalf("after save rows %v", got)
	}
	cards := pipelinePage(t, c, caseID, oid)["values"].(map[string]any)["otherFinancings"].([]any)
	if len(cards) != 2 || cards[1].(map[string]any)["bankName"] != "Bank B" {
		t.Fatalf("reopened cards = %v", cards)
	}
	movedTo(t, h, caseID, submitOtherFinancing(t, c, caseID, oid, "Data Permohonan Fasilitas"), "Data Permohonan Fasilitas")
	if got := otherFinancingRows(t, h, caseID); len(got) != 0 {
		t.Fatalf("rows = %v", got)
	}
}

// Agreed comments 3 and 5: Back to Previous Step saves and reopens Data Usaha
// Nasabah filled; the breadcrumb reaches any step; a step is no target of
// itself; only these forms' fields are accepted.
func TestHotProspectMoves(t *testing.T) {
	h, c, caseID, id := hotProspect(t, false)
	if r := submitBusinessData(t, c, caseID, id, "Data Usaha Nasabah", validBusinessDataForm()); r.Status != http.StatusBadRequest {
		t.Fatalf("to itself = %d", r.Status)
	}
	if r := submitBusinessData(t, c, caseID, id, "Data Pembiayaan Lain", map[string]any{"businessField": "x", "bankName": "y"}); r.Status != http.StatusBadRequest {
		t.Fatalf("other form's field = %d", r.Status)
	}
	oid := toOtherFinancing(t, h, c, caseID, id)
	bid := movedTo(t, h, caseID, submitOtherFinancing(t, c, caseID, oid, "Data Usaha Nasabah", otherFinancingCard{BankName: "Bank A"}), "Data Usaha Nasabah")
	if v := pipelinePage(t, c, caseID, bid)["values"].(map[string]any); v["businessField"] != "Toko Kelontong" || v["subdistrictCode"] != "3171011" {
		t.Fatalf("reopened = %v", v)
	}
	if got := otherFinancingRows(t, h, caseID); len(got) != 1 {
		t.Fatalf("back did not save: %v", got)
	}
	movedTo(t, h, caseID, submitBusinessData(t, c, caseID, bid, "QCA Data", validBusinessDataForm()), "QCA Data")
	if !processStarted(t, h, caseID, "QCA Data") {
		t.Fatal("process QCA Data not started")
	}
}

// FR-APP-017 Isi Data dari SLIK: the customer's received SLIK facilities
// grouped by condition (ordered by its code), each mapped to a card (money
// rounded, Jangka Waktu in full months); none for a Badan Usaha.
func TestDataPembiayaanLainSLIKFacilities(t *testing.T) {
	h, c, caseID, id := hotProspect(t, false)
	oid := toOtherFinancing(t, h, c, caseID, id)
	exec(t, h, `DELETE FROM fos.slik_facility WHERE slik_report_id IN (SELECT slik_report_id FROM fos.slik_report WHERE case_id = $1)`, caseID)
	exec(t, h, `INSERT INTO fos.slik_facility (slik_report_id, sequence_number, financial_institution_description, credit_financing_type_description,
		initial_credit_limit, outstanding_principal, credit_start_date, due_date, interest_rate, quality, usage_type_description, condition, condition_description, detail_json)
		SELECT slik_report_id, f.seq, f.bank, f.kind, f.amount, f.outstanding, f.started::date, f.due::date, f.rate, f.quality, f.usage, f.cond, f.cond_ket, '{}'::jsonb
		FROM fos.slik_report, (VALUES
			(1, 'Bank Lunas', 'KPR', 1000::numeric, 0::numeric, '2010-01-01', '2020-01-01', 10.5::numeric, 1::smallint, 'Investasi', '02', 'Lunas'),
			(2, 'PT Bank Mega Tbk', 'Kartu Kredit', 7000000.40, 9822753.50, '2014-01-15', '2027-01-14', 21::numeric, 1::smallint, 'Konsumsi', '00', 'Aktif'),
			(3, 'Bank Kedua', 'KMK', 5000000, 2500000.49, '2025-03-31', '2025-04-30', NULL, 2::smallint, NULL, '00', 'Aktif'),
			(4, 'Bank Tanpa Ket', 'KMK', 1, 1, NULL, NULL, NULL, 1::smallint, NULL, '09', NULL)
		) AS f (seq, bank, kind, amount, outstanding, started, due, rate, quality, usage, cond, cond_ket)
		WHERE case_id = $1 AND subject_type = 'CUSTOMER'`, caseID)
	r := c.Do(t, http.MethodGet, submitPath(caseID, oid, "slik-facilities"), nil)
	if r.Status != http.StatusOK {
		t.Fatalf("facilities = %d %s", r.Status, r.Raw)
	}
	groups := r.Body["groups"].([]any)
	var titles []string
	for _, g := range groups {
		titles = append(titles, g.(map[string]any)["title"].(string))
	}
	if !reflect.DeepEqual(titles, []string{"Fasilitas Aktif (2)", "Fasilitas Lunas (1)", "Fasilitas 09 (1)"}) {
		t.Fatalf("titles = %v", titles)
	}
	mega := groups[0].(map[string]any)["facilities"].([]any)[0].(map[string]any)
	card := mega["card"].(map[string]any)
	if mega["financingAmount"] != "7000000.40" || mega["outstandingAmount"] != "9822753.50" || mega["marginRate"] != "21.00" {
		t.Fatalf("shown = %v", mega)
	}
	want := map[string]any{"bankName": "PT Bank Mega Tbk", "financingType": "Kartu Kredit", "financingAmount": "7000000", "outstandingAmount": "9822754",
		"tenorMonths": "155", "marginRate": "21.00", "slikCollect": "1", "purpose": "Konsumsi"}
	if !reflect.DeepEqual(card, want) {
		t.Fatalf("card = %v", card)
	}
	second := groups[0].(map[string]any)["facilities"].([]any)[1].(map[string]any)["card"].(map[string]any)
	if second["tenorMonths"] != "0" || second["marginRate"] != "" || second["outstandingAmount"] != "2500000" {
		t.Fatalf("second card = %v", second)
	}

	hb, cb, caseB, idB := hotProspect(t, true)
	ob := toOtherFinancing(t, hb, cb, caseB, idB)
	// Even with a received customer SLIK, a Badan Usaha gets no button (agreed comment 10).
	exec(t, hb, `INSERT INTO fos.slik_report (case_id, subject_type, reference_code, identity_number, status, created_at, updated_at) VALUES ($1, 'CUSTOMER', 'REF-BU-1', $2, 'RECEIVED', now(), now())`, caseB, companyNPWP)
	if p := pipelinePage(t, cb, caseB, ob); p["slikAvailable"] != false {
		t.Fatalf("badan usaha slikAvailable = %v", p["slikAvailable"])
	}
	if r := cb.Do(t, http.MethodGet, submitPath(caseB, ob, "slik-facilities"), nil); r.Status != http.StatusBadRequest {
		t.Fatalf("badan usaha facilities = %d", r.Status)
	}
}
