package app_test

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"fos/internal/apptest"
)

// seedBMVUnder inserts a BMV row with a sub_type and Value1, as the
// cascading regions and MARITAL_STATUS need.
func seedBMVUnder(t *testing.T, h *apptest.Harness, bmvType, subType, code, description string, sortOrder int, value1 string) {
	t.Helper()
	exec(t, h, `INSERT INTO fos.business_mapping_value (code, type, sub_type, description, sort_order, value1, status, created_at, created_by, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), 'Active', now(), 'test', now(), 'test')`, code, bmvType, subType, description, sortOrder, value1)
}

// seedProspectMasters adds the test master data of FR-APP-012 to FR-APP-014.
func seedProspectMasters(t *testing.T, h *apptest.Harness) {
	t.Helper()
	for _, r := range []struct{ typ, code, description string }{
		{"JOB", "J1", "Karyawan Swasta"},
		{"GENDER", "G1", "Pria"}, {"GENDER", "G2", "Wanita"},
		{"SOURCE_INCOME", "SI1", "Gaji"},
		{"RELATIONSHIP_WITH_BANK", "RB1", "Tidak Ada"},
		{"CUSTOMER_CATEGORY", "CC1", "Perorangan"},
		{"ECONOMIC_SECTOR", "1100", "Konstruksi Gedung"}, {"ECONOMIC_SECTOR", "2200", "Perdagangan"},
		{"BUSINESS_TYPE", "BT1", "Swasta"},
		{"HOUSE_OWNERSHIP_STATUS", "HO1", "Milik Sendiri"},
		{"LAST_EDUCATION", "ED1", "S1"},
		{"DEPENDENTSLIST", "D0", "0"},
		{"PRENUP", "PN1", "Ya"}, {"PRENUP", "PN2", "Tidak"},
		{"FAMILY_RELATIONSHIP", "FR1", "Saudara Kandung"},
		{"PROVINCE", "31", "DKI Jakarta"}, {"PROVINCE", "32", "Jawa Barat"},
	} {
		h.SeedBMV(t, r.typ, r.code, r.description, 1, "Active")
	}
	h.SeedBMV(t, "ECONOMIC_SECTOR", "9900", "Konsultan Lama", 1, "Inactive")
	h.SeedBMV(t, "MAILING_ADDRESS", "MA-2", "Domisili", 2, "Active")
	h.SeedBMV(t, "MAILING_ADDRESS", "MA-1", "e-KTP", 1, "Active")
	seedBMVUnder(t, h, "MARITAL_STATUS", "", "MS-1", "Belum Kawin", 1, "B")
	seedBMVUnder(t, h, "MARITAL_STATUS", "", "MS-2", "Kawin", 2, "K")
	seedBMVUnder(t, h, "CITY", "31", "3171", "Jakarta Selatan", 1, "JAKSEL")
	seedBMVUnder(t, h, "CITY", "32", "3273", "Bandung", 1, "BDG")
	seedBMVUnder(t, h, "DISTRICT", "3171", "317101", "Tebet", 1, "")
	seedBMVUnder(t, h, "SUBDISTRICT", "317101", "3171011", "Manggarai", 1, "")
	seedBMVUnder(t, h, "POSTALCODE", "3171011", "12850", "12850", 1, "")
}

// prospect takes an Individual case (with or without a Spouse at
// Pre-Screening) through Pipeline Data Entry to its Data Pribadi.
func prospect(t *testing.T, withSpouse bool) (*apptest.Harness, *apptest.Client, string, int64) {
	t.Helper()
	h, _, caseID := pefindoIndividual(t, withSpouse, pefindoNotFound())
	seedPipelineMasters(t, h)
	seedProspectMasters(t, h)
	c := relogin(t, h, "AO001")
	pipelineID := assignmentsNamed(t, h, caseID, "Pipeline Data Entry")[0].ID
	if r := submitPipeline(t, c, caseID, pipelineID, pipelineForm{"New", "794", ""}); r.Status != http.StatusOK {
		t.Fatalf("pipeline = %d %s", r.Status, r.Raw)
	}
	return h, c, caseID, assignmentsNamed(t, h, caseID, "Data Pribadi")[0].ID
}

type addressForm struct {
	Address            string `json:"address"`
	MobilePhone        string `json:"mobilePhone"`
	Phone              string `json:"phone"`
	RT                 string `json:"rt"`
	RW                 string `json:"rw"`
	YearsOfStay        string `json:"yearsOfStay"`
	MonthsOfStay       string `json:"monthsOfStay"`
	HouseOwnershipCode string `json:"houseOwnershipCode"`
	ProvinceCode       string `json:"provinceCode"`
	CityCode           string `json:"cityCode"`
	DistrictCode       string `json:"districtCode"`
	SubdistrictCode    string `json:"subdistrictCode"`
}

type individualDataForm struct {
	IDLifetime                bool        `json:"idLifetime"`
	Nationality               string      `json:"nationality"`
	Email                     string      `json:"email"`
	NPWP                      string      `json:"npwp"`
	JobCode                   string      `json:"jobCode"`
	Workplace                 string      `json:"workplace"`
	GenderCode                string      `json:"genderCode"`
	WorkplaceAddress          string      `json:"workplaceAddress"`
	GrossIncome               string      `json:"grossIncome"`
	IncomeSourceCode          string      `json:"incomeSourceCode"`
	RelationshipWithBankCode  string      `json:"relationshipWithBankCode"`
	CustomerCategoryCode      string      `json:"customerCategoryCode"`
	WorkplaceSectorCode       string      `json:"workplaceSectorCode"`
	BirthPlace                string      `json:"birthPlace"`
	WorkplaceBusinessTypeCode string      `json:"workplaceBusinessTypeCode"`
	EKTP                      addressForm `json:"ektp"`
	SameAsEKTP                bool        `json:"sameAsEktp"`
	Domicile                  addressForm `json:"domicile"`
	MailingAddressCode        string      `json:"mailingAddressCode"`
	EducationCode             string      `json:"educationCode"`
	MaritalStatusCode         string      `json:"maritalStatusCode"`
	MotherMaidenName          string      `json:"motherMaidenName"`
	DependentsCode            string      `json:"dependentsCode"`
}

func validEKTP() addressForm {
	return addressForm{"Jl. Tebet Raya 1", "0812-3456-7890", "+62-21", "007", "012", "5", "11", "HO1", "31", "3171", "317101", "3171011"}
}

func validIndividualDataForm(marital string) individualDataForm {
	return individualDataForm{
		IDLifetime: true, Nationality: "Indonesia", NPWP: "123456789012345", JobCode: "J1", Workplace: "PT Maju",
		GenderCode: "G1", WorkplaceAddress: "Jl. Kerja 2", GrossIncome: "15000000", IncomeSourceCode: "SI1",
		RelationshipWithBankCode: "RB1", CustomerCategoryCode: "CC1", WorkplaceSectorCode: "1100", BirthPlace: "Jakarta",
		WorkplaceBusinessTypeCode: "BT1", EKTP: validEKTP(), SameAsEKTP: true, EducationCode: "ED1",
		MaritalStatusCode: marital, MotherMaidenName: "Siti", DependentsCode: "D0",
	}
}

func prospectPath(caseID string, id int64, form, to string) string {
	return submitPath(caseID, id, form+"?to="+url.QueryEscape(to))
}

func submitIndividualData(t *testing.T, c *apptest.Client, caseID string, id int64, to string, f any) apptest.Response {
	t.Helper()
	return c.Do(t, http.MethodPost, prospectPath(caseID, id, "individual-data", to), f)
}

// movedTo checks a move answered with the new opened assignment of name.
func movedTo(t *testing.T, h *apptest.Harness, caseID string, r apptest.Response, name string) int64 {
	t.Helper()
	if r.Status != http.StatusOK {
		t.Fatalf("move = %d %s", r.Status, r.Raw)
	}
	got := assignmentsNamed(t, h, caseID, name)
	last := got[len(got)-1]
	if last.Status != "sudah dibuka" || last.Holder != "AO001" || r.Body["assignmentId"] != float64(last.ID) {
		t.Fatalf("%s = %+v, answer %s", name, got, r.Raw)
	}
	return last.ID
}

type customerRowP struct {
	ID                                                                  int64
	NIK, Name, BirthPlace, Job, Education, Prenup, Mobile, WorkDuration *string
	Marital, MaritalPBS, Mailing, Gross, Relationship                   *string
	Lifetime, Same, Existing                                            *bool
}

func customerOf(t *testing.T, h *apptest.Harness, caseID, customerType string) (customerRowP, bool) {
	t.Helper()
	var c customerRowP
	err := h.DB.QueryRow(context.Background(), `
		SELECT customer_id, id_number, full_name, birth_place, job_code, education_code, prenup_code, mobile_phone, work_duration,
		       marital_status_code, marital_status_pbs_code, mailing_address_code, gross_income_yearly::text, relationship_code,
		       id_lifetime, same_address_as_ektp, is_existing_customer
		FROM fos.customer WHERE case_id = $1 AND customer_type = $2`, caseID, customerType).
		Scan(&c.ID, &c.NIK, &c.Name, &c.BirthPlace, &c.Job, &c.Education, &c.Prenup, &c.Mobile, &c.WorkDuration,
			&c.Marital, &c.MaritalPBS, &c.Mailing, &c.Gross, &c.Relationship, &c.Lifetime, &c.Same, &c.Existing)
	return c, err == nil
}

// addressOf is a customer_address row as "address|phone|office|mobile|rt|rw|years|months|ownership|province|city|regency|district|subdistrict|postal".
func addressOf(t *testing.T, h *apptest.Harness, customerID int64, addressType string) string {
	t.Helper()
	var s string
	err := h.DB.QueryRow(context.Background(), `
		SELECT concat_ws('|', COALESCE(address, ''), COALESCE(phone, ''), COALESCE(office_phone, ''), COALESCE(mobile_phone, ''),
		       COALESCE(rt, ''), COALESCE(rw, ''), COALESCE(years_of_stay::text, ''), COALESCE(months_of_stay::text, ''),
		       COALESCE(house_ownership_code, ''), COALESCE(province_code, ''), COALESCE(city_code, ''), COALESCE(regency_city_code, ''),
		       COALESCE(district_code, ''), COALESCE(subdistrict_code, ''), COALESCE(postal_code, ''))
		FROM fos.customer_address WHERE customer_id = $1 AND address_type = $2`, customerID, addressType).Scan(&s)
	if err != nil {
		return ""
	}
	return s
}

// AC-FR-APP-012-02, FR-APP-012 Business Rule 1: Data Pribadi shows the
// Pre-Screening NIK, name and birth date, the initial dropdowns, and whether
// the customer is new or existing, worked out and stored on every opening.
func TestDataPribadiPageShowsPreScreeningAndCustomerStatus(t *testing.T) {
	h, c, caseID, id := prospect(t, false)
	p := pipelinePage(t, c, caseID, id)
	if p["nik"] != "3171011234567890" || p["name"] != "Budi Santoso" || p["birthDate"] != yearsAgo(h, 30, 0) ||
		p["existingCustomer"] != false || p["editable"] != true {
		t.Fatalf("page = %v", p)
	}
	opts := p["options"].(map[string]any)
	if got := optionTexts(opts["mailingAddress"]); !reflect.DeepEqual(got, []string{"MA-1=e-KTP", "MA-2=Domisili"}) {
		t.Fatalf("mailing = %v", got)
	}
	if got := optionTexts(opts["province"]); !reflect.DeepEqual(got, []string{"31=DKI Jakarta", "32=Jawa Barat"}) {
		t.Fatalf("province = %v", got)
	}
	if cu, _ := customerOf(t, h, caseID, "Nasabah"); cu.Existing == nil || *cu.Existing {
		t.Fatalf("is_existing_customer = %v", cu.Existing)
	}
	exec(t, h, `INSERT INTO fos.existing_customers (id_number, customer_type, full_name, created_at, created_by, updated_at, updated_by)
		VALUES ('3171011234567890', 'Perorangan', 'Budi', now(), 'x', now(), 'x')`)
	if p := pipelinePage(t, c, caseID, id); p["existingCustomer"] != true {
		t.Fatalf("existing = %v", p["existingCustomer"])
	}
	if cu, _ := customerOf(t, h, caseID, "Nasabah"); cu.Existing == nil || !*cu.Existing {
		t.Fatal("is_existing_customer not stored")
	}
	// A Badan Usaha row with the same number does not count.
	exec(t, h, `DELETE FROM fos.existing_customers`)
	exec(t, h, `INSERT INTO fos.registered_customers (id_number, customer_type, full_name, created_at, created_by, updated_at, updated_by)
		VALUES ('3171011234567890', 'Badan Usaha', 'PT', now(), 'x', now(), 'x')`)
	if p := pipelinePage(t, c, caseID, id); p["existingCustomer"] != false {
		t.Fatalf("other type counted: %v", p["existingCustomer"])
	}
}

// FR-APP-012 Validation, AC-FR-APP-012-03, -05, -07: each wrong field is
// one error; a refused Next stores nothing and stays on Data Pribadi.
func TestDataPribadiValidation(t *testing.T) {
	h, c, caseID, id := prospect(t, false)
	for _, tc := range []struct {
		name   string
		change func(f *individualDataForm)
		want   fieldError
	}{
		{"lifetime", func(f *individualDataForm) { f.IDLifetime = false }, fieldError{"form", "Berlaku Sampai", "ERR-010", "Berlaku Sampai wajib diisi"}},
		{"nationality", func(f *individualDataForm) { f.Nationality = strings.Repeat("a", 51) }, fieldError{"form", "Kewarganegaraan", "ERR-040", "Kewarganegaraan maksimal 50 karakter"}},
		{"email", func(f *individualDataForm) { f.Email = "budi@mail" }, fieldError{"form", "Alamat Email", "ERR-037", "Harap masukkan Alamat Email yang valid."}},
		{"npwp letters", func(f *individualDataForm) { f.NPWP = "12345678901234a" }, fieldError{"form", "Nomor NPWP", "ERR-011", "Harap masukkan angka saja"}},
		{"npwp short", func(f *individualDataForm) { f.NPWP = "12345678901234" }, fieldError{"form", "Nomor NPWP", "ERR-012", "NPWP minimal 15 digit maksimal 22 digit"}},
		{"job", func(f *individualDataForm) { f.JobCode = "X" }, fieldError{"form", "Pekerjaan", "ERR-037", "Harap masukkan Pekerjaan yang valid."}},
		{"gender", func(f *individualDataForm) { f.GenderCode = "" }, fieldError{"form", "Jenis Kelamin", "ERR-010", "Jenis Kelamin wajib diisi"}},
		{"income", func(f *individualDataForm) { f.GrossIncome = "1000000000000000" }, fieldError{"form", "Penghasilan Kotor Pertahun", "ERR-037", "Harap masukkan Penghasilan Kotor Pertahun yang valid."}},
		{"sector", func(f *individualDataForm) { f.WorkplaceSectorCode = "kon" }, fieldError{"form", "Bidang Usaha Tempat Bekerja", "ERR-037", "Harap masukkan Bidang Usaha Tempat Bekerja yang valid."}},
		{"inactive sector", func(f *individualDataForm) { f.WorkplaceSectorCode = "9900" }, fieldError{"form", "Bidang Usaha Tempat Bekerja", "ERR-037", "Harap masukkan Bidang Usaha Tempat Bekerja yang valid."}},
		{"birth place", func(f *individualDataForm) { f.BirthPlace = "Jakarta1" }, fieldError{"form", "Tempat Lahir", "ERR-037", "Harap masukkan Tempat Lahir yang valid."}},
		{"rw", func(f *individualDataForm) { f.EKTP.RW = "0012" }, fieldError{"ektp", "RW", "ERR-040", "RW maksimal 3 karakter"}},
		{"rt letters", func(f *individualDataForm) { f.EKTP.RT = "0a" }, fieldError{"ektp", "RT", "ERR-037", "Harap masukkan RT yang valid."}},
		{"mobile", func(f *individualDataForm) { f.EKTP.MobilePhone = "0812a" }, fieldError{"ektp", "Nomor Handphone", "ERR-037", "Harap masukkan Nomor Handphone yang valid."}},
		{"phone digits", func(f *individualDataForm) { f.EKTP.Phone = "0812345678901234" }, fieldError{"ektp", "Nomor Telepon", "ERR-037", "Harap masukkan Nomor Telepon yang valid."}},
		{"years", func(f *individualDataForm) { f.EKTP.YearsOfStay = "100" }, fieldError{"ektp", "Lama Tinggal (Tahun)", "ERR-037", "Harap masukkan Lama Tinggal (Tahun) yang valid."}},
		{"months", func(f *individualDataForm) { f.EKTP.MonthsOfStay = "12" }, fieldError{"ektp", "Lama Tinggal (Bulan)", "ERR-037", "Harap masukkan Lama Tinggal (Bulan) yang valid."}},
		{"city of another province", func(f *individualDataForm) { f.EKTP.CityCode = "3273" }, fieldError{"ektp", "Kota", "ERR-037", "Harap masukkan Kota yang valid."}},
		{"no subdistrict", func(f *individualDataForm) { f.EKTP.SubdistrictCode = "" }, fieldError{"ektp", "Kelurahan", "ERR-010", "Kelurahan wajib diisi"}},
		{"domicile shown", func(f *individualDataForm) {
			f.SameAsEKTP = false
			f.Domicile = validEKTP()
			f.Domicile.Address = ""
			f.MailingAddressCode = "MA-1"
		}, fieldError{"domicile", "Alamat Rumah Tinggal Sekarang (Jika berbeda dengan e-KTP)", "ERR-010", "Alamat Rumah Tinggal Sekarang (Jika berbeda dengan e-KTP) wajib diisi"}},
		{"mailing shown", func(f *individualDataForm) { f.SameAsEKTP = false; f.Domicile = validEKTP() }, fieldError{"domicile", "Alamat Surat Menyurat", "ERR-010", "Alamat Surat Menyurat wajib diisi"}},
		{"mother", func(f *individualDataForm) { f.MotherMaidenName = strings.Repeat("a", 31) }, fieldError{"form", "Nama Gadis Ibu Kandung", "ERR-013", "Nama maksimal 30 karakter"}},
		{"marital", func(f *individualDataForm) { f.MaritalStatusCode = "MS-9" }, fieldError{"form", "Status Perkawinan", "ERR-037", "Harap masukkan Status Perkawinan yang valid."}},
	} {
		f := validIndividualDataForm("MS-1")
		tc.change(&f)
		if got := fieldErrors(t, submitIndividualData(t, c, caseID, id, "Data Pasangan", f)); !reflect.DeepEqual(got, []fieldError{tc.want}) {
			t.Errorf("%s: errors %v, want %v", tc.name, got, tc.want)
		}
	}
	if cu, _ := customerOf(t, h, caseID, "Nasabah"); cu.BirthPlace != nil || assignmentsNamed(t, h, caseID, "Data Pribadi")[0].Status == "sudah selesai dikerjakan" {
		t.Fatalf("refused Next stored or moved: %+v", cu)
	}
	// An empty form names every required field, top to bottom; the hidden
	// domicile block is not checked.
	got := fieldErrors(t, submitIndividualData(t, c, caseID, id, "Data Pasangan", individualDataForm{SameAsEKTP: true}))
	var fields []string
	for _, e := range got {
		fields = append(fields, e.Section+"/"+e.Field)
	}
	want := []string{"form/Berlaku Sampai", "form/Kewarganegaraan", "form/Nomor NPWP", "form/Pekerjaan", "form/Tempat Bekerja",
		"form/Jenis Kelamin", "form/Alamat Tempat Kerja", "form/Penghasilan Kotor Pertahun", "form/Sumber Penghasilan",
		"form/Hubungan dengan Bank", "form/Golongan Nasabah", "form/Bidang Usaha Tempat Bekerja", "form/Tempat Lahir",
		"form/Tipe Bisnis Tempat Bekerja", "ektp/Alamat Rumah Tinggal (Sesuai e-KTP)", "ektp/Nomor Handphone", "ektp/Nomor Telepon",
		"ektp/RT", "ektp/RW", "ektp/Lama Tinggal (Tahun)", "ektp/Lama Tinggal (Bulan)", "ektp/Status Kepemilikan Rumah",
		"ektp/Provinsi", "ektp/Kota", "ektp/Kecamatan", "ektp/Kelurahan", "form/Pendidikan Terakhir", "form/Status Perkawinan",
		"form/Nama Gadis Ibu Kandung", "form/Jumlah Tanggungan"}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("empty form errors\n got %v\nwant %v", fields, want)
	}
}

// AC-FR-APP-012-04, -05, -06, -09, DB-RULE-098: a valid Next stores Data
// Pribadi (code only, Value1 of the marital status, income in rupiah, RT
// with its zeros) and the two addresses; checked "sama dengan e-KTP" makes
// Domisili a copy and an empty Alamat Surat Menyurat the first option; Data
// Pasangan opens at once for the same operator.
func TestDataPribadiNextStoresAndOpensDataPasangan(t *testing.T) {
	h, c, caseID, id := prospect(t, false)
	f := validIndividualDataForm("MS-1")
	f.Domicile = addressForm{Address: "ignored", RT: "999"}
	r := submitIndividualData(t, c, caseID, id, "Data Pasangan", f)
	movedTo(t, h, caseID, r, "Data Pasangan")
	if got := assignmentsNamed(t, h, caseID, "Data Pribadi"); got[0].Status != "sudah selesai dikerjakan" || got[0].Completed == nil {
		t.Fatalf("Data Pribadi = %+v", got)
	}
	cu, _ := customerOf(t, h, caseID, "Nasabah")
	if deref(cu.Gross) != "15000000" || deref(cu.Marital) != "MS-1" || deref(cu.MaritalPBS) != "B" || deref(cu.Mailing) != "MA-1" ||
		cu.Lifetime == nil || !*cu.Lifetime || cu.Same == nil || !*cu.Same || deref(cu.BirthPlace) != "Jakarta" || deref(cu.Job) != "J1" {
		t.Fatalf("customer = %+v", cu)
	}
	wantAddress := "Jl. Tebet Raya 1|+62-21||0812-3456-7890|007|012|5|11|HO1|31|3171|JAKSEL|317101|3171011|12850"
	if got := addressOf(t, h, cu.ID, "e-KTP"); got != wantAddress {
		t.Fatalf("e-KTP = %s", got)
	}
	if got := addressOf(t, h, cu.ID, "Domisili"); got != wantAddress {
		t.Fatalf("Domisili = %s", got)
	}
	if !processStarted(t, h, caseID, "Data Pasangan") {
		t.Fatal("process Data Pasangan not started")
	}
}

// FR-APP-012 Business Rule 6: unchecked, Domisili is the operator's own and
// a chosen Alamat Surat Menyurat is kept; a request with another form's
// field is refused (AC-FR-APP-012-08, BR-UI-010).
func TestDataPribadiOwnDomicileAndOnlyItsFields(t *testing.T) {
	h, c, caseID, id := prospect(t, false)
	other := map[string]any{"idLifetime": true, "prenupCode": "PN1"}
	if r := submitIndividualData(t, c, caseID, id, "Data Pasangan", other); r.Status != http.StatusBadRequest {
		t.Fatalf("other form's field = %d", r.Status)
	}
	f := validIndividualDataForm("MS-1")
	f.SameAsEKTP, f.MailingAddressCode = false, "MA-2"
	f.Domicile = validEKTP()
	f.Domicile.Address, f.Domicile.RT = "Jl. Domisili 9", "001"
	movedTo(t, h, caseID, submitIndividualData(t, c, caseID, id, "Data Pasangan", f), "Data Pasangan")
	cu, _ := customerOf(t, h, caseID, "Nasabah")
	if deref(cu.Mailing) != "MA-2" || cu.Same == nil || *cu.Same {
		t.Fatalf("customer = %+v", cu)
	}
	if got := addressOf(t, h, cu.ID, "Domisili"); !strings.HasPrefix(got, "Jl. Domisili 9|") || !strings.Contains(got, "|001|") {
		t.Fatalf("Domisili = %s", got)
	}
}

type spouseForm struct {
	Name            string `json:"name"`
	NIK             string `json:"nik"`
	BirthDate       string `json:"birthDate"`
	BirthPlace      string `json:"birthPlace"`
	JobCode         string `json:"jobCode"`
	WorkDuration    string `json:"workDuration"`
	OfficeAddress   string `json:"officeAddress"`
	Phone           string `json:"phone"`
	MobilePhone     string `json:"mobilePhone"`
	ProvinceCode    string `json:"provinceCode"`
	CityCode        string `json:"cityCode"`
	DistrictCode    string `json:"districtCode"`
	SubdistrictCode string `json:"subdistrictCode"`
	EducationCode   string `json:"educationCode"`
	PrenupCode      string `json:"prenupCode"`
}

// toSpouse fills Data Pribadi with marital and returns Data Pasangan.
func toSpouse(t *testing.T, h *apptest.Harness, c *apptest.Client, caseID string, id int64, marital string) int64 {
	t.Helper()
	return movedTo(t, h, caseID, submitIndividualData(t, c, caseID, id, "Data Pasangan", validIndividualDataForm(marital)), "Data Pasangan")
}

// AC-FR-APP-013-01, -03, -04, -06: married, Data Pasangan shows the
// Pre-Screening spouse to edit; required fields are ERR-010, optional ones
// never; Next updates the same Pasangan row and stores the Kantor address
// and the PRENUP code, then Data Keluarga Dekat opens.
func TestDataPasanganMarried(t *testing.T) {
	h, c, caseID, id := prospect(t, true)
	sid := toSpouse(t, h, c, caseID, id, "MS-2")
	p := pipelinePage(t, c, caseID, sid)
	v := p["values"].(map[string]any)
	if p["married"] != true || v["name"] != "Siti Aminah" || v["nik"] != spouseNIK || v["birthDate"] != "1990-01-01" {
		t.Fatalf("page = %v", p)
	}
	f := spouseForm{Name: "Siti Aminah", NIK: spouseNIK, BirthDate: "1990-01-01", JobCode: "J1"}
	got := fieldErrors(t, c.Do(t, http.MethodPost, prospectPath(caseID, sid, "spouse", "Data Keluarga Dekat"), f))
	want := []fieldError{
		{"form", "Tempat Lahir", "ERR-010", "Tempat Lahir wajib diisi"},
		{"form", "Nomor Handphone", "ERR-010", "Nomor Handphone wajib diisi"},
		{"form", "Perjanjian Pisah Harta", "ERR-010", "Perjanjian Pisah Harta wajib diisi"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("errors %v", got)
	}
	f.Name, f.BirthPlace, f.MobilePhone, f.PrenupCode = "Siti Rahma", "Bogor", "0813", "PN2"
	f.OfficeAddress, f.ProvinceCode = "Jl. Kantor 3", "31"
	movedTo(t, h, caseID, c.Do(t, http.MethodPost, prospectPath(caseID, sid, "spouse", "Data Keluarga Dekat"), f), "Data Keluarga Dekat")
	if n := count(t, h, `SELECT count(*) FROM fos.customer WHERE case_id = $1 AND customer_type = 'Pasangan'`, caseID); n != 1 {
		t.Fatalf("Pasangan rows = %d", n)
	}
	sp, _ := customerOf(t, h, caseID, "Pasangan")
	if deref(sp.Name) != "Siti Rahma" || deref(sp.Prenup) != "PN2" || deref(sp.Mobile) != "0813" || deref(sp.BirthPlace) != "Bogor" {
		t.Fatalf("Pasangan = %+v", sp)
	}
	if got := addressOf(t, h, sp.ID, "Kantor"); got != "Jl. Kantor 3|||||||||31|||||" {
		t.Fatalf("Kantor = %q", got)
	}
}

// DB-RULE-102: a married spouse without any office field has no Kantor row,
// an earlier one is removed.
func TestDataPasanganWithoutOfficeHasNoKantor(t *testing.T) {
	h, c, caseID, id := prospect(t, true)
	sid := toSpouse(t, h, c, caseID, id, "MS-2")
	sp, _ := customerOf(t, h, caseID, "Pasangan")
	exec(t, h, `INSERT INTO fos.customer_address (customer_id, address_type, address) VALUES ($1, 'Kantor', 'Jl. Lama')`, sp.ID)
	f := spouseForm{Name: "Siti Aminah", NIK: spouseNIK, BirthDate: "1990-01-01", BirthPlace: "Bogor", JobCode: "J1", MobilePhone: "0813", PrenupCode: "PN1"}
	movedTo(t, h, caseID, c.Do(t, http.MethodPost, prospectPath(caseID, sid, "spouse", "Data Keluarga Dekat"), f), "Data Keluarga Dekat")
	if addressOf(t, h, sp.ID, "Kantor") != "" {
		t.Fatal("Kantor kept without office fields")
	}
}

// AC-FR-APP-013-02, agreed comment 2: not married, Data Pasangan shows only
// the status; Next checks nothing, clears the Prospect spouse data and the
// Kantor address and keeps the Pre-Screening spouse.
func TestDataPasanganNotMarried(t *testing.T) {
	h, c, caseID, id := prospect(t, true)
	sid := toSpouse(t, h, c, caseID, id, "MS-1")
	if p := pipelinePage(t, c, caseID, sid); p["married"] != false {
		t.Fatalf("page = %v", p)
	}
	sp, _ := customerOf(t, h, caseID, "Pasangan")
	exec(t, h, `UPDATE fos.customer SET job_code = 'J1', prenup_code = 'PN1', birth_place = 'Bogor' WHERE customer_id = $1`, sp.ID)
	exec(t, h, `INSERT INTO fos.customer_address (customer_id, address_type, address) VALUES ($1, 'Kantor', 'Jl. Lama')`, sp.ID)
	movedTo(t, h, caseID, c.Do(t, http.MethodPost, prospectPath(caseID, sid, "spouse", "Data Keluarga Dekat"), map[string]any{}), "Data Keluarga Dekat")
	sp, ok := customerOf(t, h, caseID, "Pasangan")
	if !ok || deref(sp.NIK) != spouseNIK || deref(sp.Name) != "Siti Aminah" || sp.Job != nil || sp.Prenup != nil || sp.BirthPlace != nil {
		t.Fatalf("Pasangan = %+v", sp)
	}
	if addressOf(t, h, sp.ID, "Kantor") != "" {
		t.Fatal("Kantor kept")
	}
}

type closeRelativeForm struct {
	Name             string `json:"name"`
	RelationshipCode string `json:"relationshipCode"`
	Address          string `json:"address"`
	HomePhone        string `json:"homePhone"`
	OfficePhone      string `json:"officePhone"`
	ProvinceCode     string `json:"provinceCode"`
	CityCode         string `json:"cityCode"`
	DistrictCode     string `json:"districtCode"`
	SubdistrictCode  string `json:"subdistrictCode"`
	MobilePhone      string `json:"mobilePhone"`
	EducationCode    string `json:"educationCode"`
}

// AC-FR-APP-014-01, -02, -03, FR-APP-014 Process 3 (agreed comment 5):
// Create checks the required fields, stores the Keluarga row and address,
// moves the case to Financing Application, Pending-Submission, with a new
// Data Usaha Nasabah assignment.
func TestDataKeluargaDekatCreate(t *testing.T) {
	h, c, caseID, id := prospect(t, false)
	sid := toSpouse(t, h, c, caseID, id, "MS-1")
	fid := movedTo(t, h, caseID, c.Do(t, http.MethodPost, prospectPath(caseID, sid, "spouse", "Data Keluarga Dekat"), map[string]any{}), "Data Keluarga Dekat")
	got := fieldErrors(t, c.Do(t, http.MethodPost, prospectPath(caseID, fid, "close-relative", "create"), closeRelativeForm{Name: "Andi", Address: "Jl. Saudara 4"}))
	var fields []string
	for _, e := range got {
		fields = append(fields, e.Field+" "+e.Code)
	}
	want := []string{"Hubungan Keluarga ERR-010", "Provinsi ERR-010", "Kota ERR-010", "Kecamatan ERR-010", "Kelurahan ERR-010", "No. HP ERR-010", "Pendidikan Terakhir ERR-010"}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("errors %v", fields)
	}
	f := closeRelativeForm{"Andi", "FR1", "Jl. Saudara 4", "021-123", "021-456", "31", "3171", "317101", "3171011", "0814", "ED1"}
	r := c.Do(t, http.MethodPost, prospectPath(caseID, fid, "close-relative", "create"), f)
	if r.Status != http.StatusOK {
		t.Fatalf("create = %d %s", r.Status, r.Raw)
	}
	kd, ok := customerOf(t, h, caseID, "Keluarga")
	if !ok || deref(kd.Name) != "Andi" || deref(kd.Relationship) != "FR1" || deref(kd.Education) != "ED1" {
		t.Fatalf("Keluarga = %+v", kd)
	}
	if got := addressOf(t, h, kd.ID, "Keluarga"); got != "Jl. Saudara 4|021-123|021-456|0814||||||31|3171|JAKSEL|317101|3171011|12850" {
		t.Fatalf("Keluarga address = %s", got)
	}
	if got := assignmentsNamed(t, h, caseID, "Data Keluarga Dekat"); got[0].Status != "sudah selesai dikerjakan" {
		t.Fatalf("Data Keluarga Dekat = %+v", got)
	}
	next := assignmentsNamed(t, h, caseID, "Data Usaha Nasabah")
	if len(next) != 1 || next[0].Status != "baru" || next[0].Holder != "AO001" {
		t.Fatalf("Data Usaha Nasabah = %+v", next)
	}
	if currentStage(t, h, caseID) != "Financing Application" || caseStatus(t, h, caseID) != "Pending-Submission" ||
		count(t, h, `SELECT count(*) FROM fos.case_stage_progress WHERE case_id = $1 AND stage_name = 'Financing Application'`, caseID) != 1 ||
		!processStarted(t, h, caseID, "Data Usaha Nasabah") {
		t.Fatalf("stage %s, status %s", currentStage(t, h, caseID), caseStatus(t, h, caseID))
	}
}

// Agreed comment 3: Back and a breadcrumb step validate and save the form,
// then open the target filled from the saved data; the active step itself
// is no target; Create belongs to Data Keluarga Dekat only.
func TestProspectBackAndBreadcrumb(t *testing.T) {
	h, c, caseID, id := prospect(t, false)
	if r := submitIndividualData(t, c, caseID, id, "Data Pribadi", validIndividualDataForm("MS-1")); r.Status != http.StatusBadRequest {
		t.Fatalf("to itself = %d", r.Status)
	}
	if r := submitIndividualData(t, c, caseID, id, "create", validIndividualDataForm("MS-1")); r.Status != http.StatusBadRequest {
		t.Fatalf("create from Data Pribadi = %d", r.Status)
	}
	// Breadcrumb straight to Data Keluarga Dekat, then Back twice.
	fid := movedTo(t, h, caseID, submitIndividualData(t, c, caseID, id, "Data Keluarga Dekat", validIndividualDataForm("MS-1")), "Data Keluarga Dekat")
	sid := movedTo(t, h, caseID, c.Do(t, http.MethodPost, prospectPath(caseID, fid, "close-relative", "Data Pasangan"),
		closeRelativeForm{"Andi", "FR1", "Jl. Saudara 4", "", "", "31", "3171", "317101", "3171011", "0814", "ED1"}), "Data Pasangan")
	pid := movedTo(t, h, caseID, c.Do(t, http.MethodPost, prospectPath(caseID, sid, "spouse", "Data Pribadi"), map[string]any{}), "Data Pribadi")
	v := pipelinePage(t, c, caseID, pid)["values"].(map[string]any)
	ektp := v["ektp"].(map[string]any)
	if v["birthPlace"] != "Jakarta" || v["grossIncome"] != "15000000" || ektp["rt"] != "007" || ektp["cityCode"] != "3171" || v["sameAsEktp"] != true {
		t.Fatalf("reopened values = %v", v)
	}
	// Back from Data Keluarga Dekat saved it: reopening shows it.
	fid2 := movedTo(t, h, caseID, submitIndividualData(t, c, caseID, pid, "Data Keluarga Dekat", validIndividualDataForm("MS-1")), "Data Keluarga Dekat")
	if fv := pipelinePage(t, c, caseID, fid2)["values"].(map[string]any); fv["name"] != "Andi" || fv["subdistrictCode"] != "3171011" {
		t.Fatalf("family values = %v", fv)
	}
	// Only the owner moves.
	tl := relogin(t, h, "TL01")
	if r := c.Do(t, http.MethodPost, prospectPath(caseID, fid2, "spouse", "Data Pribadi"), map[string]any{}); r.Status != http.StatusBadRequest {
		t.Fatalf("wrong form = %d", r.Status)
	}
	if r := tl.Do(t, http.MethodPost, prospectPath(caseID, fid2, "close-relative", "Data Pasangan"), closeRelativeForm{}); r.Status != http.StatusForbidden {
		t.Fatalf("Team Lead = %d", r.Status)
	}
}

func regionOptions(t *testing.T, c *apptest.Client, caseID string, id int64, query string) []string {
	t.Helper()
	r := c.Do(t, http.MethodGet, submitPath(caseID, id, query), nil)
	if r.Status != http.StatusOK {
		t.Fatalf("%s = %d %s", query, r.Status, r.Raw)
	}
	var out []string
	for _, o := range r.Body["options"].([]any) {
		m := o.(map[string]any)
		value1, _ := m["value1"].(string) // sectors have none
		out = append(out, m["code"].(string)+"="+m["description"].(string)+"="+value1)
	}
	return out
}

// FR-APP-005 rules 5-6, BR-UI-005, AC-FR-APP-012-07: regions under their
// parent (Kota with Dati II), the postal code of a Kelurahan, and sectors
// matching code or description, Active only.
func TestProspectRegionsAndSectors(t *testing.T) {
	_, c, caseID, id := prospect(t, false)
	if got := regionOptions(t, c, caseID, id, "regions?type=CITY&parent=31"); !reflect.DeepEqual(got, []string{"3171=Jakarta Selatan=JAKSEL"}) {
		t.Fatalf("cities = %v", got)
	}
	if got := regionOptions(t, c, caseID, id, "regions?type=POSTALCODE&parent=3171011"); !reflect.DeepEqual(got, []string{"12850=12850="}) {
		t.Fatalf("postal = %v", got)
	}
	if r := c.Do(t, http.MethodGet, submitPath(caseID, id, "regions?type=JOB&parent=x"), nil); r.Status != http.StatusBadRequest {
		t.Fatalf("other type = %d", r.Status)
	}
	if got := regionOptions(t, c, caseID, id, "economic-sectors?q=KON"); !reflect.DeepEqual(got, []string{"1100=Konstruksi Gedung="}) {
		t.Fatalf("sectors kon = %v", got)
	}
	if got := regionOptions(t, c, caseID, id, "economic-sectors?q=22"); !reflect.DeepEqual(got, []string{"2200=Perdagangan="}) {
		t.Fatalf("sectors 22 = %v", got)
	}
}
