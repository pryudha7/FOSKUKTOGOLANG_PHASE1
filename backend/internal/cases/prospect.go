package cases

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"fos/internal/auth"
)

// Prospect Data Entry Perorangan: three assignments in one tabbed flow
// (FR-APP-012 to FR-APP-014). Create on the last one enters Financing
// Application (FR-APP-015).
const (
	AssignmentSpouseData   = "Data Pasangan"
	AssignmentFamilyData   = "Data Keluarga Dekat"
	AssignmentBusinessData = "Data Usaha Nasabah"

	// ProspectCreate is the move of Create on Data Keluarga Dekat.
	ProspectCreate = "create"

	// maritalMarried is the MARITAL_STATUS code that shows Data Pasangan (DEC-128).
	maritalMarried = "MS-2"

	// customer_type and address_type values set by the system (DB-RULE-099).
	partyFamily     = "Keluarga"
	addressEKTP     = "e-KTP"
	addressDomicile = "Domisili"
	addressOffice   = "Kantor"
	addressFamily   = "Keluarga"

	// Banner sections of the two Data Pribadi address blocks (PROPOSED prefixes).
	SectionEKTP     = "ektp"
	SectionDomicile = "domicile"
)

// prospectSteps is the tabbed flow, in order.
var prospectSteps = []string{AssignmentIndividualData, AssignmentSpouseData, AssignmentFamilyData}

// Address is an address block of a form: text, phones, stay and the
// region codes (§9.29). Dati II and Kode Pos are worked out by the server.
type Address struct {
	Address, MobilePhone, Phone, RT, RW, YearsOfStay, MonthsOfStay, HouseOwnershipCode string
	Region
}

// Region is the cascade Provinsi, Kota, Kecamatan, Kelurahan (FR-APP-005
// rules 5 and 6).
type Region struct {
	ProvinceCode, CityCode, DistrictCode, SubdistrictCode string
}

// PersonalForm is Data Pribadi (FR-APP-012). Numbers come as digits.
type PersonalForm struct {
	IDLifetime                                                                    bool
	Nationality, Email, NPWP, JobCode, Workplace, GenderCode, WorkplaceAddress    string
	GrossIncome, IncomeSourceCode, RelationshipWithBankCode, CustomerCategoryCode string
	WorkplaceSectorCode, BirthPlace, WorkplaceBusinessTypeCode                    string
	EKTP                                                                          Address
	SameAsEKTP                                                                    bool
	Domicile                                                                      Address
	MailingAddressCode, EducationCode, MaritalStatusCode, MotherMaidenName        string
	DependentsCode                                                                string
}

// SpouseForm is Data Pasangan (FR-APP-013); BirthDate is yyyy-mm-dd.
type SpouseForm struct {
	Name, NIK, BirthDate, BirthPlace, JobCode, WorkDuration, OfficeAddress, Phone, MobilePhone string
	Region
	EducationCode, PrenupCode string
}

// FamilyForm is Data Keluarga Dekat (FR-APP-014).
type FamilyForm struct {
	Name, RelationshipCode, Address, HomePhone, OfficePhone string
	Region
	MobilePhone, EducationCode string
}

// PersonalPage is UI-026: the Pre-Screening identity, the customer status
// banner, the saved values and the initial dropdowns.
type PersonalPage struct {
	NIK, Name, BirthDate string
	ExistingCustomer     bool
	Values               PersonalForm
	// Sector is the saved Bidang Usaha Tempat Bekerja, shown by description.
	Sector   *Option
	Options  map[string][]Option
	Editable bool
}

// SpousePage is UI-027: the form only when the customer is married.
type SpousePage struct {
	Married                  bool
	Values                   SpouseForm
	Options                  map[string][]Option
	Editable                 bool
	Today, EarliestBirthDate string
}

// FamilyPage is UI-028.
type FamilyPage struct {
	Values   FamilyForm
	Options  map[string][]Option
	Editable bool
}

// RegionOption is a cascading region; Value1 is a Kota's Dati II.
type RegionOption struct {
	Code, Description, Value1 string
}

// regionLevels are the cascade, top first: BMV type, label and field key.
var regionLevels = []struct{ bmvType, label, key string }{
	{"PROVINCE", "Provinsi", "province"},
	{"CITY", "Kota", "city"},
	{"DISTRICT", "Kecamatan", "district"},
	{"SUBDISTRICT", "Kelurahan", "subdistrict"},
}

func (r Region) codes() []string {
	return []string{r.ProvinceCode, r.CityCode, r.DistrictCode, r.SubdistrictCode}
}

// pageOptions reads the Active options of each BMV type, keyed by name.
func (s *Service) pageOptions(ctx context.Context, types map[string]string) (map[string][]Option, error) {
	out := map[string][]Option{}
	for key, bmvType := range types {
		opts, err := s.options(ctx, bmvType)
		if err != nil {
			return nil, err
		}
		out[key] = opts
	}
	return out, nil
}

// partyID is the customer_id of a party of the case, if it has a row.
func partyID(ctx context.Context, q rowQuerier, caseID, customerType string) (int64, bool, error) {
	var id int64
	err := q.QueryRow(ctx, `SELECT customer_id FROM fos.customer WHERE case_id = $1 AND customer_type = $2`, caseID, customerType).Scan(&id)
	if isNotFound(err) {
		return 0, false, nil
	}
	return id, err == nil, err
}

// savedAddress reads an address of a customer; empty when there is none.
func savedAddress(ctx context.Context, q rowQuerier, customerID int64, addressType string) (Address, error) {
	var a Address
	var cols [12]*string
	err := q.QueryRow(ctx, `
		SELECT address, mobile_phone, phone, rt, rw, years_of_stay::text, months_of_stay::text, house_ownership_code,
		       province_code, city_code, district_code, subdistrict_code
		FROM fos.customer_address WHERE customer_id = $1 AND address_type = $2`, customerID, addressType).
		Scan(&cols[0], &cols[1], &cols[2], &cols[3], &cols[4], &cols[5], &cols[6], &cols[7], &cols[8], &cols[9], &cols[10], &cols[11])
	if isNotFound(err) {
		return a, nil
	}
	a.Address, a.MobilePhone, a.Phone, a.RT, a.RW = value(cols[0]), value(cols[1]), value(cols[2]), value(cols[3]), value(cols[4])
	a.YearsOfStay, a.MonthsOfStay, a.HouseOwnershipCode = value(cols[5]), value(cols[6]), value(cols[7])
	a.Region = Region{value(cols[8]), value(cols[9]), value(cols[10]), value(cols[11])}
	return a, err
}

// existingCustomer is FR-APP-005 rule 1 for a Perorangan NIK: listed on
// Registered Customers or Existing Customers (DB-RULE-095).
func existingCustomer(ctx context.Context, q rowQuerier, idNumber string) (bool, error) {
	var found bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM fos.registered_customers WHERE id_number = $1 AND customer_type = $2)
		    OR EXISTS (SELECT 1 FROM fos.existing_customers WHERE id_number = $1 AND customer_type = $2)`,
		idNumber, customerTypeIndividual).Scan(&found)
	return found, err
}

func (s *Service) personalPage(ctx context.Context, caseID string, editable bool) (PersonalPage, error) {
	page := PersonalPage{Editable: editable}
	var (
		id                                                                                   int64
		nik, name, birthPlace, nationality, email, npwp, job, workplace, gender, workAddress *string
		gross, income, relation, category, sector, businessType, mailing, education, marital *string
		mother, dependents                                                                   *string
		born                                                                                 *string
		lifetime, same                                                                       *bool
	)
	if err := s.db.QueryRow(ctx, `
		SELECT customer_id, id_number, full_name, to_char(birth_date, 'YYYY-MM-DD'), birth_place, nationality, email, npwp, job_code,
		       workplace, gender_code, workplace_address, gross_income_yearly::text, income_source_code, relationship_with_bank_code,
		       customer_category_code, workplace_sector_code, workplace_business_type_code, mailing_address_code, education_code,
		       marital_status_code, mother_maiden_name, dependents_code, id_lifetime, same_address_as_ektp
		FROM fos.customer WHERE case_id = $1 AND customer_type = $2`, caseID, partyCustomer).
		Scan(&id, &nik, &name, &born, &birthPlace, &nationality, &email, &npwp, &job, &workplace, &gender, &workAddress, &gross,
			&income, &relation, &category, &sector, &businessType, &mailing, &education, &marital, &mother, &dependents, &lifetime, &same); err != nil {
		return PersonalPage{}, err
	}
	page.NIK, page.Name, page.BirthDate = value(nik), value(name), value(born)
	existing, err := existingCustomer(ctx, s.db, page.NIK)
	if err != nil {
		return PersonalPage{}, err
	}
	page.ExistingCustomer = existing
	// Stored on every opening by the operator working on it (FR-APP-012 rule 1).
	if editable {
		if _, err := s.db.Exec(ctx, `UPDATE fos.customer SET is_existing_customer = $2 WHERE customer_id = $1`, id, existing); err != nil {
			return PersonalPage{}, err
		}
	}
	page.Values = PersonalForm{
		IDLifetime: lifetime != nil && *lifetime, Nationality: value(nationality), Email: value(email), NPWP: value(npwp),
		JobCode: value(job), Workplace: value(workplace), GenderCode: value(gender), WorkplaceAddress: value(workAddress),
		GrossIncome: value(gross), IncomeSourceCode: value(income), RelationshipWithBankCode: value(relation),
		CustomerCategoryCode: value(category), WorkplaceSectorCode: value(sector), BirthPlace: value(birthPlace),
		WorkplaceBusinessTypeCode: value(businessType), SameAsEKTP: same != nil && *same, MailingAddressCode: value(mailing),
		EducationCode: value(education), MaritalStatusCode: value(marital), MotherMaidenName: value(mother), DependentsCode: value(dependents),
	}
	if page.Values.EKTP, err = savedAddress(ctx, s.db, id, addressEKTP); err != nil {
		return PersonalPage{}, err
	}
	if page.Values.Domicile, err = savedAddress(ctx, s.db, id, addressDomicile); err != nil {
		return PersonalPage{}, err
	}
	// An Inactive sector still shows its description (BR-UI-012).
	if code := page.Values.WorkplaceSectorCode; code != "" {
		var description string
		err := s.db.QueryRow(ctx, `SELECT description FROM fos.business_mapping_value WHERE type = 'ECONOMIC_SECTOR' AND sub_type = '' AND code = $1`, code).Scan(&description)
		if err != nil && !isNotFound(err) {
			return PersonalPage{}, err
		}
		page.Sector = &Option{code, description}
	}
	page.Options, err = s.pageOptions(ctx, map[string]string{
		"job": "JOB", "gender": "GENDER", "sourceIncome": "SOURCE_INCOME", "relationshipWithBank": "RELATIONSHIP_WITH_BANK",
		"customerCategory": "CUSTOMER_CATEGORY", "businessType": "BUSINESS_TYPE", "houseOwnership": "HOUSE_OWNERSHIP_STATUS",
		"mailingAddress": "MAILING_ADDRESS", "education": "LAST_EDUCATION", "maritalStatus": "MARITAL_STATUS",
		"dependents": "DEPENDENTSLIST", "province": "PROVINCE",
	})
	return page, err
}

func (s *Service) spousePage(ctx context.Context, sess auth.Session, caseID string, editable bool) (SpousePage, error) {
	page := SpousePage{Editable: editable}
	var marital *string
	if err := s.db.QueryRow(ctx, `SELECT marital_status_code FROM fos.customer WHERE case_id = $1 AND customer_type = $2`,
		caseID, partyCustomer).Scan(&marital); err != nil {
		return SpousePage{}, err
	}
	page.Married = value(marital) == maritalMarried
	today, err := s.operatorToday(ctx, s.db, sess.UserID)
	if err != nil {
		return SpousePage{}, err
	}
	page.Today, page.EarliestBirthDate = today.Format(dateISO), EarliestBirthDate(today).Format(dateISO)
	var (
		id                                                                int64
		nik, name, born, birthPlace, job, duration, mobile, education, pn *string
	)
	err = s.db.QueryRow(ctx, `
		SELECT customer_id, id_number, full_name, to_char(birth_date, 'YYYY-MM-DD'), birth_place, job_code, work_duration,
		       mobile_phone, education_code, prenup_code
		FROM fos.customer WHERE case_id = $1 AND customer_type = $2`, caseID, partySpouse).
		Scan(&id, &nik, &name, &born, &birthPlace, &job, &duration, &mobile, &education, &pn)
	switch {
	case isNotFound(err):
	case err != nil:
		return SpousePage{}, err
	default:
		office, err := savedAddress(ctx, s.db, id, addressOffice)
		if err != nil {
			return SpousePage{}, err
		}
		page.Values = SpouseForm{
			Name: value(name), NIK: value(nik), BirthDate: value(born), BirthPlace: value(birthPlace), JobCode: value(job),
			WorkDuration: value(duration), OfficeAddress: office.Address, Phone: office.Phone, MobilePhone: value(mobile),
			Region: office.Region, EducationCode: value(education), PrenupCode: value(pn),
		}
	}
	page.Options, err = s.pageOptions(ctx, map[string]string{"job": "JOB", "education": "LAST_EDUCATION", "prenup": "PRENUP", "province": "PROVINCE"})
	return page, err
}

func (s *Service) familyPage(ctx context.Context, caseID string, editable bool) (FamilyPage, error) {
	page := FamilyPage{Editable: editable}
	var (
		id                            int64
		name, relationship, education *string
	)
	err := s.db.QueryRow(ctx, `SELECT customer_id, full_name, relationship_code, education_code FROM fos.customer WHERE case_id = $1 AND customer_type = $2`,
		caseID, partyFamily).Scan(&id, &name, &relationship, &education)
	switch {
	case isNotFound(err):
	case err != nil:
		return FamilyPage{}, err
	default:
		a, err := savedAddress(ctx, s.db, id, addressFamily)
		if err != nil {
			return FamilyPage{}, err
		}
		var office *string
		if err := s.db.QueryRow(ctx, `SELECT office_phone FROM fos.customer_address WHERE customer_id = $1 AND address_type = $2`,
			id, addressFamily).Scan(&office); err != nil && !isNotFound(err) {
			return FamilyPage{}, err
		}
		page.Values = FamilyForm{
			Name: value(name), RelationshipCode: value(relationship), Address: a.Address, HomePhone: a.Phone, OfficePhone: value(office),
			Region: a.Region, MobilePhone: a.MobilePhone, EducationCode: value(education),
		}
	}
	page.Options, err = s.pageOptions(ctx, map[string]string{"relationship": "FAMILY_RELATIONSHIP", "education": "LAST_EDUCATION", "province": "PROVINCE"})
	return page, err
}

// fieldCheck collects a form's errors in field order (one per field) and
// keeps the first database error; section is the banner section of the
// fields being checked.
type fieldCheck struct {
	ctx     context.Context
	tx      pgx.Tx
	section string
	errs    []FieldError
	err     error
}

func (c *fieldCheck) add(key, label, code, message string) {
	c.errs = append(c.errs, FieldError{c.section, key, label, code, message})
}

func (c *fieldCheck) addError(e *FieldError) {
	if e != nil {
		c.errs = append(c.errs, *e)
	}
}

// invalid is ERR-037 on a field (BR-UI-007).
func (c *fieldCheck) invalid(key, label string) {
	c.errs = append(c.errs, invalid(c.section, key, label))
}

// filled reports ERR-010 for an empty mandatory value; false when empty.
func (c *fieldCheck) filled(key, label, v string, mandatory bool) bool {
	if v != "" {
		return true
	}
	if mandatory {
		c.errs = append(c.errs, required(c.section, key, label))
	}
	return false
}

// text is a free text of at most max characters (ERR-040).
func (c *fieldCheck) text(key, label, v string, mandatory bool, max int) {
	if c.filled(key, label, v, mandatory) && utf8.RuneCountInString(v) > max {
		c.add(key, label, "ERR-040", fmt.Sprintf("%s maksimal %d karakter", label, max))
	}
}

// code is a value that must be an Active row of a BMV type (BR-UI-007).
func (c *fieldCheck) code(key, label, bmvType, v string, mandatory bool) {
	if !c.filled(key, label, v, mandatory) || c.err != nil {
		return
	}
	known, err := activeCode(c.ctx, c.tx, bmvType, v)
	switch {
	case err != nil:
		c.err = err
	case !known:
		c.invalid(key, label)
	}
}

var (
	phonePattern = regexp.MustCompile(`^[0-9+\-]+$`)
	digits       = regexp.MustCompile(`^[0-9]+$`)
	// Letters and spaces only (Tempat Lahir).
	placePattern = regexp.MustCompile(`^[\p{L} ]+$`)
)

// phone is FR-APP-005 rule 8: 0-9, "+" and "-" only, one digit at least, at
// most 15 digits and 20 characters.
func (c *fieldCheck) phone(key, label, v string, mandatory bool) {
	if !c.filled(key, label, v, mandatory) {
		return
	}
	n := 0
	for _, r := range v {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	if !phonePattern.MatchString(v) || n == 0 || n > 15 || len(v) > 20 {
		c.invalid(key, label)
	}
}

// email is FR-APP-005 rule 9: optional, one "@", local part and a domain
// with a dot, no spaces, at most 254 characters.
func (c *fieldCheck) email(key, label, v string) {
	if !c.filled(key, label, v, false) {
		return
	}
	local, domain, ok := strings.Cut(v, "@")
	switch {
	case utf8.RuneCountInString(v) > 254:
		c.add(key, label, "ERR-040", fmt.Sprintf("%s maksimal %d karakter", label, 254))
	case !ok || local == "" || domain == "" || strings.Contains(domain, "@") || !strings.Contains(domain, ".") || strings.ContainsAny(v, " \t"):
		c.invalid(key, label)
	}
}

// npwp is the Pre-Screening NPWP rule: digits only (ERR-011), 15 to 22 of
// them (ERR-012).
func (c *fieldCheck) npwp(key, label, v string) {
	switch {
	case !c.filled(key, label, v, true):
	case !digits.MatchString(v):
		c.add(key, label, "ERR-011", "Harap masukkan angka saja")
	case len(v) < 15 || len(v) > 22:
		c.add(key, label, "ERR-012", "NPWP minimal 15 digit maksimal 22 digit")
	}
}

// number is a whole number from 0 to max (FR-APP-012 rule 10); over max is
// ERR-037.
func (c *fieldCheck) number(key, label, v string, max int) {
	if !c.filled(key, label, v, true) {
		return
	}
	n, err := strconv.Atoi(v)
	if !digits.MatchString(v) || err != nil || n > max {
		c.invalid(key, label)
	}
}

// rtRW is RT or RW: digits only, at most 3, leading zeros kept.
func (c *fieldCheck) rtRW(key, label, v string) {
	switch {
	case !c.filled(key, label, v, true):
	case !digits.MatchString(v):
		c.invalid(key, label)
	case len(v) > 3:
		c.add(key, label, "ERR-040", label+" maksimal 3 karakter")
	}
}

// birthPlace is Tempat Lahir: letters and spaces, at most 50.
func (c *fieldCheck) birthPlace(key, label, v string) {
	switch {
	case !c.filled(key, label, v, true):
	case utf8.RuneCountInString(v) > 50:
		c.add(key, label, "ERR-040", label+" maksimal 50 karakter")
	case !placePattern.MatchString(v):
		c.invalid(key, label)
	}
}

// resolvedRegion is a checked region with what the server works out:
// Dati II (the Kota's Value1) and Kode Pos (DB-RULE-096).
type resolvedRegion struct {
	Region
	regency, postal string
}

// region checks the cascade (FR-APP-005 rules 5 and 6): each unit must be an
// Active row under the unit above it. Below an empty unit a filled one is
// ERR-037 when the region is optional (when mandatory the empty unit is
// already ERR-010); below an invalid one nothing is judged again, but an
// empty mandatory unit is still ERR-010.
func (c *fieldCheck) region(r Region, mandatory bool) resolvedRegion {
	const (
		fine = iota
		missing
		broken
	)
	out := resolvedRegion{Region: r}
	parent, above := "", fine
	for i, code := range r.codes() {
		level := regionLevels[i]
		if !c.filled(level.key, level.label, code, mandatory) {
			above = missing
			continue
		}
		if above == missing && !mandatory {
			c.invalid(level.key, level.label)
		}
		if above != fine {
			continue
		}
		var value1 *string
		err := c.tx.QueryRow(c.ctx, `
			SELECT value1 FROM fos.business_mapping_value
			WHERE type = $1 AND sub_type = $2 AND code = $3 AND `+activeStatus, level.bmvType, parent, code).Scan(&value1)
		switch {
		case isNotFound(err):
			c.invalid(level.key, level.label)
			above = broken
			continue
		case err != nil:
			c.err = err
			return out
		}
		if level.bmvType == "CITY" {
			out.regency = value(value1)
		}
		parent = code
	}
	if above == fine && r.SubdistrictCode != "" {
		err := c.tx.QueryRow(c.ctx, `
			SELECT code FROM fos.business_mapping_value WHERE type = 'POSTALCODE' AND sub_type = $1 AND `+activeStatus+`
			ORDER BY sort_order, code LIMIT 1`, r.SubdistrictCode).Scan(&out.postal)
		if err != nil && !isNotFound(err) {
			c.err = err
		}
	}
	return out
}

// address checks a Data Pribadi address block in its layout order.
func (c *fieldCheck) address(a Address, addressLabel string, mailing *string) resolvedRegion {
	c.text("address", addressLabel, a.Address, true, 350)
	c.phone("mobilePhone", "Nomor Handphone", a.MobilePhone, true)
	c.phone("phone", "Nomor Telepon", a.Phone, true)
	if mailing == nil {
		c.rtRW("rt", "RT", a.RT)
		c.rtRW("rw", "RW", a.RW)
		c.number("yearsOfStay", "Lama Tinggal (Tahun)", a.YearsOfStay, 99)
		c.number("monthsOfStay", "Lama Tinggal (Bulan)", a.MonthsOfStay, 11)
		c.code("houseOwnership", "Status Kepemilikan Rumah", "HOUSE_OWNERSHIP_STATUS", a.HouseOwnershipCode, true)
	} else {
		// Domisili: Bulan, Tahun and Alamat Surat Menyurat come first (Gambar 52).
		c.number("monthsOfStay", "Lama Tinggal (Bulan)", a.MonthsOfStay, 11)
		c.number("yearsOfStay", "Lama Tinggal (Tahun)", a.YearsOfStay, 99)
		c.code("mailingAddress", "Alamat Surat Menyurat", "MAILING_ADDRESS", *mailing, true)
		c.code("houseOwnership", "Status Kepemilikan Rumah", "HOUSE_OWNERSHIP_STATUS", a.HouseOwnershipCode, true)
		c.rtRW("rt", "RT", a.RT)
		c.rtRW("rw", "RW", a.RW)
	}
	return c.region(a.Region, true)
}

// addressRow is a customer_address row to store; nil columns stay NULL.
type addressRow struct {
	address, phone, officePhone, mobile, rt, rw, years, months, ownership *string
	region                                                                resolvedRegion
}

// orNil is v, or nil when empty.
func orNil(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func saveAddress(ctx context.Context, tx pgx.Tx, customerID int64, addressType string, a addressRow) error {
	r := a.region
	_, err := tx.Exec(ctx, `
		INSERT INTO fos.customer_address (customer_id, address_type, address, phone, office_phone, mobile_phone, rt, rw,
			years_of_stay, months_of_stay, house_ownership_code, province_code, city_code, regency_city_code, district_code,
			subdistrict_code, postal_code)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::smallint, $10::smallint, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (customer_id, address_type) DO UPDATE SET address = EXCLUDED.address, phone = EXCLUDED.phone,
			office_phone = EXCLUDED.office_phone, mobile_phone = EXCLUDED.mobile_phone, rt = EXCLUDED.rt, rw = EXCLUDED.rw,
			years_of_stay = EXCLUDED.years_of_stay, months_of_stay = EXCLUDED.months_of_stay,
			house_ownership_code = EXCLUDED.house_ownership_code, province_code = EXCLUDED.province_code,
			city_code = EXCLUDED.city_code, regency_city_code = EXCLUDED.regency_city_code, district_code = EXCLUDED.district_code,
			subdistrict_code = EXCLUDED.subdistrict_code, postal_code = EXCLUDED.postal_code`,
		customerID, addressType, a.address, a.phone, a.officePhone, a.mobile, a.rt, a.rw, a.years, a.months, a.ownership,
		orNil(r.ProvinceCode), orNil(r.CityCode), orNil(r.regency), orNil(r.DistrictCode), orNil(r.SubdistrictCode), orNil(r.postal))
	return err
}

// stayAddress is a Data Pribadi address block as a row.
func stayAddress(a Address, r resolvedRegion) addressRow {
	return addressRow{address: orNil(a.Address), phone: orNil(a.Phone), mobile: orNil(a.MobilePhone), rt: orNil(a.RT), rw: orNil(a.RW),
		years: orNil(a.YearsOfStay), months: orNil(a.MonthsOfStay), ownership: orNil(a.HouseOwnershipCode), region: r}
}

// trimAddress drops surrounding spaces, so spaces count as empty.
func trimAddress(a Address) Address {
	t := strings.TrimSpace
	return Address{t(a.Address), t(a.MobilePhone), t(a.Phone), t(a.RT), t(a.RW), t(a.YearsOfStay), t(a.MonthsOfStay),
		t(a.HouseOwnershipCode), trimRegion(a.Region)}
}

func trimRegion(r Region) Region {
	t := strings.TrimSpace
	return Region{t(r.ProvinceCode), t(r.CityCode), t(r.DistrictCode), t(r.SubdistrictCode)}
}

// SubmitPersonal validates and stores Data Pribadi, then moves to `to`
// (FR-APP-012 Process 4, agreed comment 3).
func (s *Service) SubmitPersonal(ctx context.Context, sess auth.Session, caseID string, id int64, to string, f PersonalForm) (int64, error) {
	return s.moveProspect(ctx, sess, caseID, id, AssignmentIndividualData, to, func(c *fieldCheck) error {
		t := strings.TrimSpace
		f.EKTP, f.Domicile = trimAddress(f.EKTP), trimAddress(f.Domicile)
		if !f.IDLifetime {
			c.errs = append(c.errs, required(SectionForm, "idLifetime", "Berlaku Sampai"))
		}
		c.text("nationality", "Kewarganegaraan", t(f.Nationality), true, 50)
		c.email("email", "Alamat Email", t(f.Email))
		c.npwp("npwp", "Nomor NPWP", t(f.NPWP))
		c.code("job", "Pekerjaan", "JOB", t(f.JobCode), true)
		c.text("workplace", "Tempat Bekerja", t(f.Workplace), true, 100)
		c.code("gender", "Jenis Kelamin", "GENDER", t(f.GenderCode), true)
		c.text("workplaceAddress", "Alamat Tempat Kerja", t(f.WorkplaceAddress), true, 350)
		// At most 15 digits, 999.999.999.999.999 (rule 5).
		if gross := t(f.GrossIncome); c.filled("grossIncome", "Penghasilan Kotor Pertahun", gross, true) && (!digits.MatchString(gross) || len(gross) > 15) {
			c.invalid("grossIncome", "Penghasilan Kotor Pertahun")
		}
		c.code("incomeSource", "Sumber Penghasilan", "SOURCE_INCOME", t(f.IncomeSourceCode), true)
		c.code("relationshipWithBank", "Hubungan dengan Bank", "RELATIONSHIP_WITH_BANK", t(f.RelationshipWithBankCode), true)
		c.code("customerCategory", "Golongan Nasabah", "CUSTOMER_CATEGORY", t(f.CustomerCategoryCode), true)
		c.code("workplaceSector", "Bidang Usaha Tempat Bekerja", "ECONOMIC_SECTOR", t(f.WorkplaceSectorCode), true)
		c.birthPlace("birthPlace", "Tempat Lahir", t(f.BirthPlace))
		c.code("workplaceBusinessType", "Tipe Bisnis Tempat Bekerja", "BUSINESS_TYPE", t(f.WorkplaceBusinessTypeCode), true)
		c.section = SectionEKTP
		ektp := c.address(f.EKTP, "Alamat Rumah Tinggal (Sesuai e-KTP)", nil)
		mailing := t(f.MailingAddressCode)
		domicile := ektp
		if !f.SameAsEKTP {
			c.section = SectionDomicile
			domicile = c.address(f.Domicile, "Alamat Rumah Tinggal Sekarang (Jika berbeda dengan e-KTP)", &mailing)
		}
		c.section = SectionForm
		c.code("education", "Pendidikan Terakhir", "LAST_EDUCATION", t(f.EducationCode), true)
		c.code("maritalStatus", "Status Perkawinan", "MARITAL_STATUS", t(f.MaritalStatusCode), true)
		c.addError(nameError(SectionForm, "motherMaidenName", "Nama Gadis Ibu Kandung", t(f.MotherMaidenName)))
		c.code("dependents", "Jumlah Tanggungan", "DEPENDENTSLIST", t(f.DependentsCode), true)
		if c.err != nil || len(c.errs) > 0 {
			return c.err
		}

		// Business Rule 6: an empty Alamat Surat Menyurat is the first option.
		if mailing == "" {
			opts, err := optionsIn(c.ctx, c.tx, "MAILING_ADDRESS")
			if err != nil {
				return err
			}
			if len(opts) > 0 {
				mailing = opts[0].Code
			}
		}
		var pbs *string
		if err := c.tx.QueryRow(c.ctx, `SELECT value1 FROM fos.business_mapping_value WHERE type = 'MARITAL_STATUS' AND sub_type = '' AND code = $1`,
			t(f.MaritalStatusCode)).Scan(&pbs); err != nil {
			return err
		}
		var customerID int64
		if err := c.tx.QueryRow(c.ctx, `
			UPDATE fos.customer SET id_lifetime = TRUE, nationality = $3, email = $4, npwp = $5, job_code = $6, workplace = $7,
				gender_code = $8, workplace_address = $9, gross_income_yearly = $10::numeric, income_source_code = $11,
				relationship_with_bank_code = $12, customer_category_code = $13, workplace_sector_code = $14, birth_place = $15,
				workplace_business_type_code = $16, same_address_as_ektp = $17, mailing_address_code = $18, education_code = $19,
				marital_status_code = $20, marital_status_pbs_code = $21, mother_maiden_name = $22, dependents_code = $23
			WHERE case_id = $1 AND customer_type = $2 RETURNING customer_id`,
			caseID, partyCustomer, t(f.Nationality), orNil(t(f.Email)), t(f.NPWP), t(f.JobCode), t(f.Workplace), t(f.GenderCode),
			t(f.WorkplaceAddress), t(f.GrossIncome), t(f.IncomeSourceCode), t(f.RelationshipWithBankCode), t(f.CustomerCategoryCode),
			t(f.WorkplaceSectorCode), t(f.BirthPlace), t(f.WorkplaceBusinessTypeCode), f.SameAsEKTP, orNil(mailing), t(f.EducationCode),
			t(f.MaritalStatusCode), pbs, t(f.MotherMaidenName), t(f.DependentsCode)).Scan(&customerID); err != nil {
			return err
		}
		if err := saveAddress(c.ctx, c.tx, customerID, addressEKTP, stayAddress(f.EKTP, ektp)); err != nil {
			return err
		}
		// DB-RULE-098: checked, Domisili is a copy of e-KTP; sent values are ignored.
		domicileForm := f.Domicile
		if f.SameAsEKTP {
			domicileForm = f.EKTP
		}
		return saveAddress(c.ctx, c.tx, customerID, addressDomicile, stayAddress(domicileForm, domicile))
	})
}

// SubmitSpouse validates and stores Data Pasangan when the customer is
// married; otherwise nothing is checked and the Prospect spouse data and
// the Kantor address are cleared, the Pre-Screening spouse kept (FR-APP-013
// Process 3, agreed comment 2).
func (s *Service) SubmitSpouse(ctx context.Context, sess auth.Session, caseID string, id int64, to string, f SpouseForm) (int64, error) {
	return s.moveProspect(ctx, sess, caseID, id, AssignmentSpouseData, to, func(c *fieldCheck) error {
		var marital *string
		if err := c.tx.QueryRow(c.ctx, `SELECT marital_status_code FROM fos.customer WHERE case_id = $1 AND customer_type = $2`,
			caseID, partyCustomer).Scan(&marital); err != nil {
			return err
		}
		spouseID, found, err := partyID(c.ctx, c.tx, caseID, partySpouse)
		if err != nil {
			return err
		}
		if value(marital) != maritalMarried {
			if !found {
				return nil
			}
			if _, err := c.tx.Exec(c.ctx, `
				UPDATE fos.customer SET birth_place = NULL, job_code = NULL, work_duration = NULL, mobile_phone = NULL,
					education_code = NULL, prenup_code = NULL WHERE customer_id = $1`, spouseID); err != nil {
				return err
			}
			_, err := c.tx.Exec(c.ctx, `DELETE FROM fos.customer_address WHERE customer_id = $1 AND address_type = $2`, spouseID, addressOffice)
			return err
		}

		t := strings.TrimSpace
		today, err := s.operatorToday(c.ctx, c.tx, sess.UserID)
		if err != nil {
			return err
		}
		region := trimRegion(f.Region)
		c.addError(nameError(SectionForm, "name", "Nama Pasangan (Sesuai e-KTP)", t(f.Name)))
		c.addError(nikError(SectionForm, "nik", "No. e-KTP", t(f.NIK)))
		c.addError(birthDateError(SectionForm, "birthDate", "Tanggal Lahir", t(f.BirthDate), today))
		c.birthPlace("birthPlace", "Tempat Lahir", t(f.BirthPlace))
		c.code("job", "Pekerjaan", "JOB", t(f.JobCode), true)
		c.text("workDuration", "Lama Bekerja", t(f.WorkDuration), false, 50)
		c.text("officeAddress", "Alamat Kantor", t(f.OfficeAddress), false, 350)
		c.phone("phone", "Nomor Telepon", t(f.Phone), false)
		c.phone("mobilePhone", "Nomor Handphone", t(f.MobilePhone), true)
		resolved := c.region(region, false)
		c.code("education", "Pendidikan Terakhir", "LAST_EDUCATION", t(f.EducationCode), false)
		c.code("prenup", "Perjanjian Pisah Harta", "PRENUP", t(f.PrenupCode), true)
		if c.err != nil || len(c.errs) > 0 {
			return c.err
		}

		// DB-RULE-101: the Pre-Screening spouse row itself, no copy.
		if err := c.tx.QueryRow(c.ctx, `
			INSERT INTO fos.customer (case_id, customer_type, id_number, full_name, birth_date, birth_place, job_code, work_duration,
				mobile_phone, education_code, prenup_code)
			VALUES ($1, $2, $3, $4, $5::date, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (case_id, customer_type) WHERE customer_type <> 'Pengurus'
			DO UPDATE SET id_number = EXCLUDED.id_number, full_name = EXCLUDED.full_name, birth_date = EXCLUDED.birth_date,
				birth_place = EXCLUDED.birth_place, job_code = EXCLUDED.job_code, work_duration = EXCLUDED.work_duration,
				mobile_phone = EXCLUDED.mobile_phone, education_code = EXCLUDED.education_code, prenup_code = EXCLUDED.prenup_code
			RETURNING customer_id`,
			caseID, partySpouse, t(f.NIK), t(f.Name), t(f.BirthDate), t(f.BirthPlace), t(f.JobCode), orNil(t(f.WorkDuration)),
			t(f.MobilePhone), orNil(t(f.EducationCode)), t(f.PrenupCode)).Scan(&spouseID); err != nil {
			return err
		}
		// DB-RULE-102: the Kantor row exists only with something in it.
		if t(f.OfficeAddress) == "" && t(f.Phone) == "" && region.ProvinceCode == "" {
			_, err := c.tx.Exec(c.ctx, `DELETE FROM fos.customer_address WHERE customer_id = $1 AND address_type = $2`, spouseID, addressOffice)
			return err
		}
		return saveAddress(c.ctx, c.tx, spouseID, addressOffice, addressRow{address: orNil(t(f.OfficeAddress)), phone: orNil(t(f.Phone)), region: resolved})
	})
}

// SubmitFamily validates and stores Data Keluarga Dekat, then moves to `to`;
// Create completes Prospect Data Entry (FR-APP-014 Process 3).
func (s *Service) SubmitFamily(ctx context.Context, sess auth.Session, caseID string, id int64, to string, f FamilyForm) (int64, error) {
	return s.moveProspect(ctx, sess, caseID, id, AssignmentFamilyData, to, func(c *fieldCheck) error {
		t := strings.TrimSpace
		region := trimRegion(f.Region)
		c.addError(nameError(SectionForm, "name", "Nama Lengkap (Sesuai e-KTP)", t(f.Name)))
		c.code("relationship", "Hubungan Keluarga", "FAMILY_RELATIONSHIP", t(f.RelationshipCode), true)
		c.text("address", "Alamat Tempat Tinggal Saat Ini", t(f.Address), true, 350)
		c.phone("homePhone", "No. Telepon Rumah", t(f.HomePhone), false)
		c.phone("officePhone", "No. Telepon Kantor", t(f.OfficePhone), false)
		resolved := c.region(region, true)
		c.phone("mobilePhone", "No. HP", t(f.MobilePhone), true)
		c.code("education", "Pendidikan Terakhir", "LAST_EDUCATION", t(f.EducationCode), true)
		if c.err != nil || len(c.errs) > 0 {
			return c.err
		}
		var familyID int64
		if err := c.tx.QueryRow(c.ctx, `
			INSERT INTO fos.customer (case_id, customer_type, full_name, relationship_code, education_code)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (case_id, customer_type) WHERE customer_type <> 'Pengurus'
			DO UPDATE SET full_name = EXCLUDED.full_name, relationship_code = EXCLUDED.relationship_code, education_code = EXCLUDED.education_code
			RETURNING customer_id`, caseID, partyFamily, t(f.Name), t(f.RelationshipCode), t(f.EducationCode)).Scan(&familyID); err != nil {
			return err
		}
		// DB-RULE-103: phones of the family address.
		return saveAddress(c.ctx, c.tx, familyID, addressFamily, addressRow{address: orNil(t(f.Address)), phone: orNil(t(f.HomePhone)),
			officePhone: orNil(t(f.OfficePhone)), mobile: orNil(t(f.MobilePhone)), region: resolved})
	})
}

// moveProspect validates and saves form (save fills c.errs) on the
// operator's open assignment, then completes it and opens the step `to`
// (FR-ASSIGN-001 rule 5), or, for Create, enters Financing Application.
func (s *Service) moveProspect(ctx context.Context, sess auth.Session, caseID string, id int64, form, to string, save func(c *fieldCheck) error) (int64, error) {
	create := to == ProspectCreate
	if (create && form != AssignmentFamilyData) || (!create && (to == form || !slices.Contains(prospectSteps, to))) {
		return 0, ErrUnsupported
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	a, err := ownOpenAssignment(ctx, tx, sess, caseID, id)
	if err != nil {
		return 0, err
	}
	if a.name != form {
		return 0, ErrUnsupported
	}
	c := &fieldCheck{ctx: ctx, tx: tx, section: SectionForm}
	if err := save(c); err != nil {
		return 0, err
	}
	if len(c.errs) > 0 {
		return 0, &ValidationError{c.errs}
	}
	if create {
		if err := s.enterFinancingApplication(ctx, tx, sess, caseID, a.id); err != nil {
			return 0, err
		}
		return 0, tx.Commit(ctx)
	}
	next, err := s.moveTo(ctx, tx, sess, caseID, a.id, to)
	if err != nil {
		return 0, err
	}
	if err := startProcess(ctx, tx, caseID, StageFinancingSubmission, to, s.now()); err != nil {
		return 0, err
	}
	return next, tx.Commit(ctx)
}

// enterFinancingApplication ends Prospect Data Entry: the assignment is
// completed, the case enters Financing Application, Pending-Submission
// (DEC-132), and the Branch Officer gets a new Data Usaha Nasabah (agreed
// comment 5, TBD-053b).
func (s *Service) enterFinancingApplication(ctx context.Context, tx pgx.Tx, sess auth.Session, caseID string, from int64) error {
	now := s.now()
	if _, err := tx.Exec(ctx, `UPDATE fos.assignments SET assignment_status = $2, completed_at = $3 WHERE assignment_id = $1`,
		from, string(AssignmentCompleted), now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fos.case_stage_progress (case_id, stage_name, first_entered_at, entered_from_stage)
		SELECT case_id, $2, $3, current_stage FROM fos.work WHERE case_id = $1
		ON CONFLICT (case_id, stage_name) DO NOTHING`, caseID, StageFinancingApplication, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE fos.work SET current_stage = $2, case_status = $3, updated_at = $4, updated_by = $5 WHERE case_id = $1`,
		caseID, StageFinancingApplication, CaseStatusPendingSubmission, now, sess.UserID); err != nil {
		return err
	}
	_, err := newBranchOfficerAssignment(ctx, tx, caseID, StageFinancingApplication, AssignmentBusinessData, now)
	return err
}

// regionTypes are the units loaded on demand under their parent; Provinsi
// comes with the page.
var regionTypes = []string{"CITY", "DISTRICT", "SUBDISTRICT", "POSTALCODE"}

// mayView checks the operator may open the assignment (worklist holder or
// Authors, as OpenAssignment).
func (s *Service) mayView(ctx context.Context, sess auth.Session, caseID string, id int64) error {
	var kind string
	var assignee *string
	err := s.db.QueryRow(ctx, `SELECT assignment_type, assigned_user_id FROM fos.assignments WHERE assignment_id = $1 AND case_id = $2`,
		id, caseID).Scan(&kind, &assignee)
	if isNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, may := mayOpen(sess, AssignmentType(kind), value(assignee)); !may {
		return ErrForbidden
	}
	return nil
}

// Regions lists the Active units of a cascading type under parent (Kota
// with its Dati II as Value1; Kode Pos under a Kelurahan), by sort_order
// then code (FR-APP-005 rules 5 and 6).
func (s *Service) Regions(ctx context.Context, sess auth.Session, caseID string, id int64, bmvType, parent string) ([]RegionOption, error) {
	if !slices.Contains(regionTypes, bmvType) || parent == "" {
		return nil, ErrUnsupported
	}
	if err := s.mayView(ctx, sess, caseID, id); err != nil {
		return nil, err
	}
	opts, err := collect(ctx, s, func(r pgx.Rows) (RegionOption, error) {
		var o RegionOption
		return o, r.Scan(&o.Code, &o.Description, &o.Value1)
	}, `
		SELECT code, description, COALESCE(value1, '') FROM fos.business_mapping_value
		WHERE type = $1 AND sub_type = $2 AND `+activeStatus+` ORDER BY sort_order, code`, bmvType, parent)
	if opts == nil {
		opts = []RegionOption{}
	}
	return opts, err
}

// EconomicSectors are the Bidang Usaha Tempat Bekerja suggestions: Active
// ECONOMIC_SECTOR rows whose code or description contains text, ignoring
// case, at most ui/autocomplete/max_results (BR-UI-005, AC-FR-APP-012-07).
func (s *Service) EconomicSectors(ctx context.Context, sess auth.Session, caseID string, id int64, text string) ([]Option, error) {
	if err := s.mayView(ctx, sess, caseID, id); err != nil {
		return nil, err
	}
	limit, err := s.settings.Int(ctx, maxResultsSetting)
	if err != nil {
		return nil, err
	}
	return optionList(collect(ctx, s, scanOption, `
		SELECT code, description FROM fos.business_mapping_value
		WHERE type = 'ECONOMIC_SECTOR' AND sub_type = '' AND `+activeStatus+`
		  AND (strpos(lower(code), lower($1)) > 0 OR strpos(lower(description), lower($1)) > 0)
		ORDER BY sort_order, code LIMIT $2`, text, limit))
}
