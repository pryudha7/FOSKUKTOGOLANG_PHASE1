package app_test

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"fos/internal/apptest"
)

// seedBusinessMasters adds the test master data of FR-APP-005 beyond the
// Prospect ones.
func seedBusinessMasters(t *testing.T, h *apptest.Harness) {
	t.Helper()
	h.SeedBMV(t, "BUSINESS_ENTITY", "PT", "Perseroan Terbatas", 1, "Active")
	h.SeedBMV(t, "GO_PUBLIC_STATUS", "GP1", "Tidak Go Public", 1, "Active")
	h.SeedBMV(t, "GO_PUBLIC_STATUS", "GP2", "Go Public", 2, "Active")
	h.SeedBMV(t, "BUSINESSENTITY_CATEGORY", "0101", "Perusahaan Swasta Nasional", 1, "Active")
	h.SeedBMV(t, "BUSINESSENTITY_CATEGORY", "0202", "BUMN", 2, "Active")
	h.SeedBMV(t, "BUSINESS_ROLE", "R1", "Pemilik Perusahaan", 1, "Active")
	h.SeedBMV(t, "BUSINESS_ROLE", "R2", "Direktur", 2, "Active")
}

// businessEntity takes a Badan Usaha case through Pipeline Data Entry to
// its Badan Usaha assignment.
func businessEntity(t *testing.T) (*apptest.Harness, *apptest.Client, string, int64) {
	t.Helper()
	h, _, caseID := pefindoBusiness(t, pefindoNotFound())
	seedPipelineMasters(t, h)
	seedProspectMasters(t, h)
	seedBusinessMasters(t, h)
	c := relogin(t, h, "AO001")
	pipelineID := assignmentsNamed(t, h, caseID, "Pipeline Data Entry")[0].ID
	if r := submitPipeline(t, c, caseID, pipelineID, pipelineForm{"New", "794", ""}); r.Status != http.StatusOK {
		t.Fatalf("pipeline = %d %s", r.Status, r.Raw)
	}
	return h, c, caseID, assignmentsNamed(t, h, caseID, "Badan Usaha")[0].ID
}

func seedCompanyOfficer(t *testing.T, h *apptest.Harness, caseID string, order int, role, nik, name, born string) {
	t.Helper()
	exec(t, h, `INSERT INTO fos.customer (case_id, customer_type, sort_order, role_code, id_number, full_name, birth_date)
		VALUES ($1, 'Pengurus', $2, $3, $4, $5, $6)`, caseID, order, role, nik, name, born)
}

type companyOfficerForm struct {
	RoleCode  string `json:"roleCode"`
	NIK       string `json:"nik"`
	Name      string `json:"name"`
	BirthDate string `json:"birthDate"`
}

type businessForm struct {
	BusinessEntityCode       string               `json:"businessEntityCode"`
	PlaceOfEstablishment     string               `json:"placeOfEstablishment"`
	DeedNumber               string               `json:"deedNumber"`
	LastDeedNumber           string               `json:"lastDeedNumber"`
	LastDeedDate             string               `json:"lastDeedDate"`
	Phone                    string               `json:"phone"`
	MobilePhone              string               `json:"mobilePhone"`
	Email                    string               `json:"email"`
	EconomicSectorCode       string               `json:"economicSectorCode"`
	RelationshipWithBankCode string               `json:"relationshipWithBankCode"`
	BusinessTypeCode         string               `json:"businessTypeCode"`
	CustomerCategoryCode     string               `json:"customerCategoryCode"`
	Address                  string               `json:"address"`
	GoPublic                 string               `json:"goPublic"`
	ProvinceCode             string               `json:"provinceCode"`
	CityCode                 string               `json:"cityCode"`
	DistrictCode             string               `json:"districtCode"`
	SubdistrictCode          string               `json:"subdistrictCode"`
	BusinessGroupName        string               `json:"businessGroupName"`
	CompanyOfficers          []companyOfficerForm `json:"companyOfficers"`
}

func validBusinessForm(h *apptest.Harness) businessForm {
	return businessForm{
		BusinessEntityCode: "PT", PlaceOfEstablishment: "Jakarta", DeedNumber: "AKTA-1", LastDeedNumber: "AKTA-2",
		LastDeedDate: yearsAgo(h, 0, -1), Phone: "021-5551234", MobilePhone: "0812", Email: "info@contoh.co.id",
		EconomicSectorCode: "1100", RelationshipWithBankCode: "RB1", BusinessTypeCode: "BT1", CustomerCategoryCode: "0101",
		Address: "Jl. Usaha 5", GoPublic: "Tidak Go Public", ProvinceCode: "31", CityCode: "3171", DistrictCode: "317101",
		SubdistrictCode: "3171011", BusinessGroupName: "Grup Contoh",
		CompanyOfficers: []companyOfficerForm{{"R1", "3171011234567801", "Ahmad Direktur", yearsAgo(h, 40, 0)}},
	}
}

func submitBusiness(t *testing.T, c *apptest.Client, caseID string, id int64, f any) apptest.Response {
	t.Helper()
	return c.Do(t, http.MethodPost, submitPath(caseID, id, "business-entity"), f)
}

// AC-FR-APP-005-01, -02, -03, FR-APP-005 Process 2: the page shows the
// Pre-Screening NPWP, name and deed date, the Pengurus of Pre-Screening in
// order (one empty card without any), and the customer status by NPWP and
// Badan Usaha.
func TestBadanUsahaPageShowsPreScreeningAndCustomerStatus(t *testing.T) {
	h, c, caseID, id := businessEntity(t)
	p := pipelinePage(t, c, caseID, id)
	v := p["values"].(map[string]any)
	if p["npwp"] != companyNPWP || p["name"] != "PT Contoh" || p["establishmentDate"] != "2010-05-01" || p["existingCustomer"] != false ||
		p["editable"] != true || p["latestDeedDate"] != yearsAgo(h, 0, -1) {
		t.Fatalf("page = %v", p)
	}
	if officers := v["companyOfficers"].([]any); len(officers) != 1 || officers[0].(map[string]any)["nik"] != "" {
		t.Fatalf("officers = %v", officers)
	}
	opts := p["options"].(map[string]any)
	if got := optionTexts(opts["goPublic"]); !reflect.DeepEqual(got, []string{"Tidak Go Public=Tidak Go Public", "Go Public=Go Public"}) {
		t.Fatalf("goPublic = %v", got)
	}
	if got := optionTexts(opts["businessRole"]); !reflect.DeepEqual(got, []string{"R1=Pemilik Perusahaan", "R2=Direktur"}) {
		t.Fatalf("businessRole = %v", got)
	}

	seedCompanyOfficer(t, h, caseID, 2, "R2", "3171011234567802", "Budi Kedua", "1980-02-02")
	seedCompanyOfficer(t, h, caseID, 1, "R1", "3171011234567801", "Ahmad Pertama", "1975-01-01")
	exec(t, h, `INSERT INTO fos.existing_customers (id_number, customer_type, full_name, created_at, created_by, updated_at, updated_by)
		VALUES ($1, 'Perorangan', 'x', now(), 'x', now(), 'x')`, companyNPWP)
	p = pipelinePage(t, c, caseID, id)
	officers := p["values"].(map[string]any)["companyOfficers"].([]any)
	if len(officers) != 2 || officers[0].(map[string]any)["name"] != "Ahmad Pertama" || officers[1].(map[string]any)["birthDate"] != "1980-02-02" {
		t.Fatalf("officers = %v", officers)
	}
	if p["existingCustomer"] != false {
		t.Fatal("a Perorangan row counted for a Badan Usaha")
	}
	exec(t, h, `INSERT INTO fos.registered_customers (id_number, customer_type, full_name, created_at, created_by, updated_at, updated_by)
		VALUES ($1, 'Badan Usaha', 'x', now(), 'x', now(), 'x')`, companyNPWP)
	if p := pipelinePage(t, c, caseID, id); p["existingCustomer"] != true {
		t.Fatalf("existing = %v", p["existingCustomer"])
	}
	if n := count(t, h, `SELECT count(*) FROM fos.customer WHERE case_id = $1 AND customer_type = 'Badan Usaha' AND is_existing_customer`, caseID); n != 1 {
		t.Fatal("is_existing_customer not stored")
	}
}

// FR-APP-005 Validation, AC-FR-APP-005-06, -08, FR-APP-002 Pengurus rules:
// each wrong field is one error; Pengurus errors name their card; a
// refused Next stores nothing.
func TestBadanUsahaValidation(t *testing.T) {
	h, c, caseID, id := businessEntity(t)
	second := companyOfficerForm{"R2", "3171011234567802", "Budi Kedua", yearsAgo(h, 30, 0)}
	for _, tc := range []struct {
		name   string
		change func(f *businessForm)
		want   []fieldError
	}{
		{"phone with a space", func(f *businessForm) { f.Phone = "+62-21-555 1234" }, []fieldError{{"form", "Nomor Telepon", "ERR-037", "Harap masukkan Nomor Telepon yang valid."}}},
		{"email", func(f *businessForm) { f.Email = "a@b" }, []fieldError{{"form", "Alamat Email", "ERR-037", "Harap masukkan Alamat Email yang valid."}}},
		{"last deed today", func(f *businessForm) { f.LastDeedDate = yearsAgo(h, 0, 0) }, []fieldError{{"form", "Tanggal Akta Perubahan Terakhir", "ERR-037", "Harap masukkan Tanggal Akta Perubahan Terakhir yang valid."}}},
		{"last deed not a date", func(f *businessForm) { f.LastDeedDate = "2020-02-30" }, []fieldError{{"form", "Tanggal Akta Perubahan Terakhir", "ERR-037", "Harap masukkan Tanggal Akta Perubahan Terakhir yang valid."}}},
		{"sector", func(f *businessForm) { f.EconomicSectorCode = "pertanian" }, []fieldError{{"form", "Kode Bidang Usaha", "ERR-037", "Harap masukkan Kode Bidang Usaha yang valid."}}},
		{"category", func(f *businessForm) { f.CustomerCategoryCode = "CC1" }, []fieldError{{"form", "Golongan Nasabah", "ERR-037", "Harap masukkan Golongan Nasabah yang valid."}}},
		{"go public code", func(f *businessForm) { f.GoPublic = "GP1" }, []fieldError{{"form", "Go Public", "ERR-037", "Harap masukkan Go Public yang valid."}}},
		{"entity", func(f *businessForm) { f.BusinessEntityCode = "CV" }, []fieldError{{"form", "Kode Badan Usaha", "ERR-037", "Harap masukkan Kode Badan Usaha yang valid."}}},
		{"place", func(f *businessForm) { f.PlaceOfEstablishment = strings.Repeat("a", 51) }, []fieldError{{"form", "Tempat Pendirian", "ERR-040", "Tempat Pendirian maksimal 50 karakter"}}},
		{"group", func(f *businessForm) { f.BusinessGroupName = strings.Repeat("a", 101) }, []fieldError{{"form", "Nama Group Usaha Nasabah", "ERR-040", "Nama Group Usaha Nasabah maksimal 100 karakter"}}},
		{"kelurahan", func(f *businessForm) { f.SubdistrictCode = "" }, []fieldError{{"form", "Kelurahan", "ERR-010", "Kelurahan wajib diisi"}}},
		{"no role", func(f *businessForm) { f.CompanyOfficers[0].RoleCode = "" }, []fieldError{{"company-officer-1", "Jabatan", "ERR-014", "Harap isi jabatan"}}},
		{"officer NIK", func(f *businessForm) {
			f.CompanyOfficers = append(f.CompanyOfficers, second)
			f.CompanyOfficers[1].NIK = "123"
		}, []fieldError{{"company-officer-2", "NIK", "ERR-007", "Harap masukkan NIK 16 digit yang valid"}}},
		{"officer age", func(f *businessForm) { f.CompanyOfficers[0].BirthDate = yearsAgo(h, 16, 0) }, []fieldError{{"company-officer-1", "Tanggal Lahir", "ERR-009", "Umur nasabah minimal 17 tahun"}}},
		{"same NIK", func(f *businessForm) {
			s := second
			s.NIK = f.CompanyOfficers[0].NIK
			f.CompanyOfficers = append(f.CompanyOfficers, s)
		}, []fieldError{
			{"company-officer-1", "NIK", "ERR-018", "NIK tidak boleh sama"}, {"company-officer-2", "NIK", "ERR-018", "NIK tidak boleh sama"}}},
		{"no officers", func(f *businessForm) { f.CompanyOfficers = nil }, []fieldError{{"form", "Data Pengurus", "ERR-015", "Minimal 1 data pengurus dibutuhkan"}}},
	} {
		f := validBusinessForm(h)
		tc.change(&f)
		if got := fieldErrors(t, submitBusiness(t, c, caseID, id, f)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: errors %v, want %v", tc.name, got, tc.want)
		}
	}
	// An empty form names every required field, top to bottom.
	got := fieldErrors(t, submitBusiness(t, c, caseID, id, businessForm{CompanyOfficers: []companyOfficerForm{{}}}))
	var fields []string
	for _, e := range got {
		fields = append(fields, e.Section+"/"+e.Field)
	}
	want := []string{"form/Tempat Pendirian", "form/Nomor Akta Pendirian", "form/Nomor Telepon", "form/Kode Bidang Usaha", "form/Tipe Bisnis",
		"form/Golongan Nasabah", "form/Alamat", "form/Go Public", "form/Provinsi", "form/Kota", "form/Kecamatan", "form/Kelurahan",
		"company-officer-1/Jabatan", "company-officer-1/NIK", "company-officer-1/Nama", "company-officer-1/Tanggal Lahir"}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("empty form errors\n got %v\nwant %v", fields, want)
	}
	// Eleven cards are not a form the page can send.
	f := validBusinessForm(h)
	for i := 0; i < 10; i++ {
		f.CompanyOfficers = append(f.CompanyOfficers, second)
	}
	if r := submitBusiness(t, c, caseID, id, f); r.Status != http.StatusBadRequest {
		t.Fatalf("11 officers = %d", r.Status)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.customer WHERE case_id = $1 AND place_of_establishment IS NOT NULL`, caseID); n != 0 ||
		assignmentsNamed(t, h, caseID, "Badan Usaha")[0].Status == "sudah selesai dikerjakan" {
		t.Fatal("a refused Next stored or moved")
	}
}

// FR-APP-005 Process 4, rules 12-13, agreed comments 1, 5, 6: Next stores
// the entity (Go Public as its description), its Badan Usaha address and
// the Pengurus in card order (rows kept, extra inserted, removed deleted),
// then the case enters Financing Application with a new Data Usaha Nasabah.
func TestBadanUsahaNextStoresAndEntersFinancingApplication(t *testing.T) {
	h, c, caseID, id := businessEntity(t)
	seedCompanyOfficer(t, h, caseID, 1, "R1", "3171011234567801", "Ahmad Lama", "1975-01-01")
	seedCompanyOfficer(t, h, caseID, 2, "R2", "3171011234567802", "Budi Lama", "1980-02-02")
	seedCompanyOfficer(t, h, caseID, 3, "R2", "3171011234567803", "Citra Lama", "1985-03-03")
	firstID := companyOfficerIDs(t, h, caseID)[0]
	f := validBusinessForm(h)
	f.CompanyOfficers = []companyOfficerForm{
		{"R2", "3171011234567801", "Ahmad Baru", "1975-01-01"},
		{"R1", "3171011234567809", "Dewi Baru", "1990-09-09"},
	}
	r := submitBusiness(t, c, caseID, id, f)
	if r.Status != http.StatusOK {
		t.Fatalf("next = %d %s", r.Status, r.Raw)
	}
	var row string
	if err := h.DB.QueryRow(context.Background(), `
		SELECT concat_ws('|', business_entity_code, place_of_establishment, deed_number, last_deed_number, last_deed_date::text, email,
		       economic_sector_code, relationship_with_bank_code, business_type_code, customer_category_code, go_public, business_group_name)
		FROM fos.customer WHERE case_id = $1 AND customer_type = 'Badan Usaha'`, caseID).Scan(&row); err != nil {
		t.Fatal(err)
	}
	if want := "PT|Jakarta|AKTA-1|AKTA-2|" + yearsAgo(h, 0, -1) + "|info@contoh.co.id|1100|RB1|BT1|0101|Tidak Go Public|Grup Contoh"; row != want {
		t.Fatalf("customer = %s\nwant %s", row, want)
	}
	var customerID int64
	if err := h.DB.QueryRow(context.Background(), `SELECT customer_id FROM fos.customer WHERE case_id = $1 AND customer_type = 'Badan Usaha'`, caseID).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if got := addressOf(t, h, customerID, "Badan Usaha"); got != "Jl. Usaha 5|021-5551234||0812||||||31|3171|JAKSEL|317101|3171011|12850" {
		t.Fatalf("address = %s", got)
	}
	ids := companyOfficerIDs(t, h, caseID)
	if len(ids) != 2 || ids[0] != firstID {
		t.Fatalf("officer rows = %v (first was %d)", ids, firstID)
	}
	if got := officers(t, h, caseID); !reflect.DeepEqual(got, []string{"1|R2|3171011234567801|Ahmad Baru|1975-01-01", "2|R1|3171011234567809|Dewi Baru|1990-09-09"}) {
		t.Fatalf("officers = %v", got)
	}
	if got := assignmentsNamed(t, h, caseID, "Badan Usaha"); got[0].Status != "sudah selesai dikerjakan" {
		t.Fatalf("Badan Usaha = %+v", got)
	}
	next := assignmentsNamed(t, h, caseID, "Data Usaha Nasabah")
	if len(next) != 1 || next[0].Status != "baru" || currentStage(t, h, caseID) != "Financing Application" || caseStatus(t, h, caseID) != "Pending-Submission" {
		t.Fatalf("next %+v, stage %s, status %s", next, currentStage(t, h, caseID), caseStatus(t, h, caseID))
	}
}

func companyOfficerIDs(t *testing.T, h *apptest.Harness, caseID string) []int64 {
	t.Helper()
	rows, err := h.DB.Query(context.Background(), `SELECT customer_id FROM fos.customer WHERE case_id = $1 AND customer_type = 'Pengurus' ORDER BY sort_order`, caseID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	return out
}

func officers(t *testing.T, h *apptest.Harness, caseID string) []string {
	t.Helper()
	rows, err := h.DB.Query(context.Background(), `
		SELECT concat_ws('|', sort_order, role_code, id_number, full_name, birth_date::text) FROM fos.customer
		WHERE case_id = $1 AND customer_type = 'Pengurus' ORDER BY sort_order`, caseID)
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

// AC-FR-APP-005-05, BR-UI-005: Kode Bidang Usaha and Golongan Nasabah
// suggest by code or description; other types are refused; only the owner
// moves the assignment and only with its own fields (BR-UI-010).
func TestBadanUsahaSuggestionsAndAccess(t *testing.T) {
	h, c, caseID, id := businessEntity(t)
	if got := regionOptions(t, c, caseID, id, "suggestions?type=BUSINESSENTITY_CATEGORY&q=bum"); !reflect.DeepEqual(got, []string{"0202=BUMN="}) {
		t.Fatalf("category bum = %v", got)
	}
	if got := regionOptions(t, c, caseID, id, "suggestions?type=BUSINESSENTITY_CATEGORY&q=01"); !reflect.DeepEqual(got, []string{"0101=Perusahaan Swasta Nasional="}) {
		t.Fatalf("category 01 = %v", got)
	}
	if r := c.Do(t, http.MethodGet, submitPath(caseID, id, "suggestions?type=BANK&q=a"), nil); r.Status != http.StatusBadRequest {
		t.Fatalf("other type = %d", r.Status)
	}
	if r := submitBusiness(t, c, caseID, id, map[string]any{"placeOfEstablishment": "x", "nationality": "x"}); r.Status != http.StatusBadRequest {
		t.Fatalf("other form's field = %d", r.Status)
	}
	tl := relogin(t, h, "TL01")
	if r := submitBusiness(t, tl, caseID, id, validBusinessForm(h)); r.Status != http.StatusForbidden {
		t.Fatalf("Team Lead = %d", r.Status)
	}
}
