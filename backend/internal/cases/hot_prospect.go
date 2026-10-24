package cases

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"fos/internal/auth"
)

// Hot Prospect Data Entry (FR-APP-015 to FR-APP-017): six assignments in
// one tabbed flow on stage Financing Application.
const (
	AssignmentOtherFinancingData = "Data Pembiayaan Lain"

	// HotProspectSave is Save: the form is kept and the page stays (agreed
	// comment 2).
	HotProspectSave = "save"

	// addressBusiness is the applicant's business address (DB-RULE-105).
	addressBusiness = "Usaha"

	// sectionOtherFinancing is the banner section of card n: "Pembiayaan <n> - ".
	sectionOtherFinancing = "other-financing-"
)

// hotProspectSteps is the tabbed flow, in order (FR-APP-015).
var hotProspectSteps = []string{AssignmentBusinessData, AssignmentOtherFinancingData, "Data Permohonan Fasilitas", "QCA Data",
	"Collateral Data", "Document Upload"}

// BusinessDataForm is Data Usaha Nasabah (FR-APP-016): the business and its
// address; numbers come as digits.
type BusinessDataForm struct {
	BusinessField, BusinessCategoryCode, BusinessAgeMonths, EmployeeCount, BusinessLocationCode string
	Address, HouseOwnershipCode, YearsOfStay, MonthsOfStay, Phone, RT, RW                       string
	Region
}

// address is the business address block (DB-RULE-105); it has no
// Nomor Handphone.
func (f BusinessDataForm) address() Address {
	return Address{Address: f.Address, Phone: f.Phone, RT: f.RT, RW: f.RW, YearsOfStay: f.YearsOfStay, MonthsOfStay: f.MonthsOfStay,
		HouseOwnershipCode: f.HouseOwnershipCode, Region: f.Region}
}

// BusinessDataPage is UI-030.
type BusinessDataPage struct {
	Values BusinessDataForm
	PageOptions
	Editable bool
}

// OtherFinancing is one Data Pembiayaan Lain card (FR-APP-017): amounts and
// counts as digits, MarginRate with a dot and at most two decimals.
type OtherFinancing struct {
	BankName, FinancingType, FinancingAmount, OutstandingAmount, TenorMonths, MarginRate, SLIKCollect, Purpose string
}

func (o OtherFinancing) empty() bool { return o == OtherFinancing{} }

// OtherFinancingPage is UI-031: the saved cards (one empty card when none)
// and whether Isi Data dari SLIK shows.
type OtherFinancingPage struct {
	Cards         []OtherFinancing
	SLIKAvailable bool
	Editable      bool
}

// applicantParty is the customer_type of the case's applicant: Nasabah or
// Badan Usaha (DB-RULE-099).
func applicantParty(ctx context.Context, q rowQuerier, caseID string) (string, error) {
	customerType, err := caseCustomerType(ctx, q, caseID)
	return customerParty(customerType), err
}

func (s *Service) businessDataPage(ctx context.Context, caseID string, editable bool) (BusinessDataPage, error) {
	page := BusinessDataPage{Editable: editable}
	party, err := applicantParty(ctx, s.db, caseID)
	if err != nil {
		return BusinessDataPage{}, err
	}
	id, found, err := partyID(ctx, s.db, caseID, party)
	if err != nil {
		return BusinessDataPage{}, err
	}
	if found {
		var age, employees *string
		var field, category, location *string
		err := s.db.QueryRow(ctx, `
			SELECT business_field, business_category_code, business_age_months::text, employee_count::text, business_location_code
			FROM fos.customer_business WHERE customer_id = $1`, id).Scan(&field, &category, &age, &employees, &location)
		if err != nil && !isNotFound(err) {
			return BusinessDataPage{}, err
		}
		a, err := savedAddress(ctx, s.db, id, addressBusiness)
		if err != nil {
			return BusinessDataPage{}, err
		}
		page.Values = BusinessDataForm{
			BusinessField: value(field), BusinessCategoryCode: value(category), BusinessAgeMonths: value(age), EmployeeCount: value(employees),
			BusinessLocationCode: value(location), Address: a.Address, HouseOwnershipCode: a.HouseOwnershipCode, YearsOfStay: a.YearsOfStay,
			MonthsOfStay: a.MonthsOfStay, Phone: a.Phone, RT: a.RT, RW: a.RW, Region: a.Region,
		}
	}
	page.PageOptions = s.pageOptions(ctx, map[string]string{
		"businessCategory": "BUSINESSCATEGORY", "businessLocation": "BUSINESSLOCATION", "houseOwnership": "HOUSE_OWNERSHIP_STATUS", "province": "PROVINCE",
	})
	return page, nil
}

// slikAvailable reports whether Isi Data dari SLIK shows: a Perorangan case
// with its customer's SLIK received from the API (FR-APP-017, agreed
// comment 10).
func slikAvailable(ctx context.Context, q rowQuerier, caseID string) (bool, error) {
	var available bool
	err := q.QueryRow(ctx, `
		SELECT w.customer_type = $2 AND EXISTS (SELECT 1 FROM fos.slik_report r WHERE r.case_id = w.case_id AND r.subject_type = $3 AND r.status = $4)
		FROM fos.work w WHERE w.case_id = $1`, caseID, customerTypeIndividual, subjectCustomer, slikReceived).Scan(&available)
	return available, err
}

func (s *Service) otherFinancingPage(ctx context.Context, caseID string, editable bool) (OtherFinancingPage, error) {
	page := OtherFinancingPage{Editable: editable}
	cards, err := collect(ctx, s, func(r pgx.Rows) (OtherFinancing, error) {
		var bank, financingType, amount, outstanding, tenor, rate, collectibility, purpose *string
		err := r.Scan(&bank, &financingType, &amount, &outstanding, &tenor, &rate, &collectibility, &purpose)
		return OtherFinancing{BankName: value(bank), FinancingType: value(financingType), FinancingAmount: value(amount),
			OutstandingAmount: value(outstanding), TenorMonths: value(tenor), MarginRate: value(rate), SLIKCollect: value(collectibility),
			Purpose: value(purpose)}, err
	}, `
		SELECT bank_name, financing_type, financing_amount::text, outstanding_amount::text, tenor_months::text, margin_rate::text,
		       slik_collect::text, purpose
		FROM fos.other_financing WHERE case_id = $1 ORDER BY sort_order`, caseID)
	if err != nil {
		return OtherFinancingPage{}, err
	}
	if len(cards) == 0 {
		cards = []OtherFinancing{{}}
	}
	page.Cards = cards
	page.SLIKAvailable, err = slikAvailable(ctx, s.db, caseID)
	return page, err
}

// moveHotProspect validates and saves form, then opens the step `to`
// (FR-ASSIGN-001 rule 5), or with Save keeps the page (agreed comments 2, 3
// and 5).
func (s *Service) moveHotProspect(ctx context.Context, sess auth.Session, caseID string, id int64, form, to string, save func(c *fieldCheck) error) (int64, error) {
	saving := to == HotProspectSave
	if !saving && (to == form || !slices.Contains(hotProspectSteps, to)) {
		return 0, ErrUnsupported
	}
	return s.saveForm(ctx, sess, caseID, id, form, func(c *fieldCheck) error {
		c.lenient = saving
		return save(c)
	}, func(tx pgx.Tx, from int64) (int64, error) {
		if saving {
			return 0, nil
		}
		next, err := s.moveTo(ctx, tx, sess, caseID, from, to)
		if err != nil {
			return 0, err
		}
		return next, startProcess(ctx, tx, caseID, StageFinancingApplication, to, s.now())
	})
}

// SubmitBusinessData validates and stores Data Usaha Nasabah (FR-APP-016):
// the business on customer_business and its address type "Usaha" on the
// applicant (DB-RULE-105/106), then moves to `to` or stays on Save.
func (s *Service) SubmitBusinessData(ctx context.Context, sess auth.Session, caseID string, id int64, to string, f BusinessDataForm) (int64, error) {
	return s.moveHotProspect(ctx, sess, caseID, id, AssignmentBusinessData, to, func(c *fieldCheck) error {
		t := strings.TrimSpace
		a := trimAddress(f.address())
		f.BusinessField, f.BusinessCategoryCode, f.BusinessAgeMonths = t(f.BusinessField), t(f.BusinessCategoryCode), t(f.BusinessAgeMonths)
		f.EmployeeCount, f.BusinessLocationCode = t(f.EmployeeCount), t(f.BusinessLocationCode)
		c.text("businessField", "Jenis Bidang Usaha", f.BusinessField, true, 100)
		c.code("businessCategory", "Kategori Bidang Usaha", "BUSINESSCATEGORY", f.BusinessCategoryCode, true)
		c.number("businessAgeMonths", "Lama Usaha (Bulan)", f.BusinessAgeMonths, 99999, true)
		c.number("employeeCount", "Jumlah Karyawan", f.EmployeeCount, 9999999, true)
		c.code("businessLocation", "Jenis Tempat Usaha", "BUSINESSLOCATION", f.BusinessLocationCode, true)
		c.text("address", "Alamat Usaha", a.Address, true, 350)
		c.code("houseOwnership", "Status Kepemilikan Tempat Usaha", "HOUSE_OWNERSHIP_STATUS", a.HouseOwnershipCode, true)
		c.number("yearsOfStay", "Lama Tempat Usaha (Tahun)", a.YearsOfStay, 999, true)
		c.number("monthsOfStay", "Lama Tempat Usaha (Bulan)", a.MonthsOfStay, 11, true)
		c.phone("phone", "Nomor Telepon", a.Phone, false)
		c.rtRW("rt", "RT", a.RT)
		c.rtRW("rw", "RW", a.RW)
		resolved := c.region(a.Region, true)
		if c.err != nil || len(c.errs) > 0 {
			return c.err
		}

		party, err := applicantParty(c.ctx, c.tx, caseID)
		if err != nil {
			return err
		}
		customerID, _, err := partyID(c.ctx, c.tx, caseID, party)
		if err != nil {
			return err
		}
		if err := saveAddress(c.ctx, c.tx, customerID, addressBusiness, stayAddress(a, resolved)); err != nil {
			return err
		}
		// §9.30 columns are NOT NULL: the row only with all five (agreed comment 16).
		if f.BusinessField == "" || f.BusinessCategoryCode == "" || f.BusinessAgeMonths == "" || f.EmployeeCount == "" || f.BusinessLocationCode == "" {
			return nil
		}
		_, err = c.tx.Exec(c.ctx, `
			INSERT INTO fos.customer_business (customer_id, business_field, business_category_code, business_age_months, employee_count, business_location_code)
			VALUES ($1, $2, $3, $4::integer, $5::integer, $6)
			ON CONFLICT (customer_id) DO UPDATE SET business_field = EXCLUDED.business_field, business_category_code = EXCLUDED.business_category_code,
				business_age_months = EXCLUDED.business_age_months, employee_count = EXCLUDED.employee_count,
				business_location_code = EXCLUDED.business_location_code`,
			customerID, f.BusinessField, f.BusinessCategoryCode, f.BusinessAgeMonths, f.EmployeeCount, f.BusinessLocationCode)
		return err
	})
}

// SubmitOtherFinancing validates the filled cards of Data Pembiayaan Lain
// and replaces the case's rows with them, all-empty cards dropped
// (FR-APP-017 Process 4-5, DB-RULE-107), then moves to `to` or stays on Save.
func (s *Service) SubmitOtherFinancing(ctx context.Context, sess auth.Session, caseID string, id int64, to string, cards []OtherFinancing) (int64, error) {
	return s.moveHotProspect(ctx, sess, caseID, id, AssignmentOtherFinancingData, to, func(c *fieldCheck) error {
		t := strings.TrimSpace
		var kept []OtherFinancing
		for i, o := range cards {
			o = OtherFinancing{t(o.BankName), t(o.FinancingType), t(o.FinancingAmount), t(o.OutstandingAmount), t(o.TenorMonths), t(o.MarginRate),
				t(o.SLIKCollect), t(o.Purpose)}
			if o.empty() {
				continue
			}
			// Errors name the card by its place on screen.
			c.section = sectionOtherFinancing + strconv.Itoa(i+1)
			c.text("bankName", "Nama Bank", o.BankName, false, 100)
			c.text("financingType", "Jenis Pembiayaan", o.FinancingType, false, 100)
			c.money("financingAmount", "Jumlah Pembiayaan", o.FinancingAmount, false)
			c.money("outstandingAmount", "Jumlah Outstanding", o.OutstandingAmount, false)
			c.number("tenorMonths", "Jangka Waktu (Bulan)", o.TenorMonths, 9999, false)
			c.percent("marginRate", "Margin / Ujrah (%)", o.MarginRate)
			c.number("slikCollect", "SLIK Collect", o.SLIKCollect, 5, false)
			c.text("purpose", "Tujuan", o.Purpose, false, 500)
			kept = append(kept, o)
		}
		c.section = SectionForm
		if c.err != nil || len(c.errs) > 0 {
			return c.err
		}
		if _, err := c.tx.Exec(c.ctx, `DELETE FROM fos.other_financing WHERE case_id = $1`, caseID); err != nil {
			return err
		}
		for i, o := range kept {
			if _, err := c.tx.Exec(c.ctx, `
				INSERT INTO fos.other_financing (case_id, sort_order, bank_name, financing_type, financing_amount, outstanding_amount,
					tenor_months, margin_rate, slik_collect, purpose)
				VALUES ($1, $2, $3, $4, $5::numeric, $6::numeric, $7::integer, $8::numeric, $9::smallint, $10)`,
				caseID, i+1, orNil(o.BankName), orNil(o.FinancingType), orNil(o.FinancingAmount), orNil(o.OutstandingAmount),
				orNil(o.TenorMonths), orNil(o.MarginRate), orNil(o.SLIKCollect), orNil(o.Purpose)); err != nil {
				return err
			}
		}
		return nil
	})
}

// SLIKFacility is one facility of the customer's SLIK as the modal shows it:
// amounts and margin with two decimals as SLIK has them, and Card, the
// card it fills (FR-APP-017 Isi Data dari SLIK rule 6).
type SLIKFacility struct {
	FinancingAmount, OutstandingAmount, MarginRate string
	Card                                           OtherFinancing
}

// SLIKFacilityGroup is one accordion: the facilities of one condition.
type SLIKFacilityGroup struct {
	Title      string
	Facilities []SLIKFacility
}

// fullMonths is the whole months from start to end (agreed comment 11).
func fullMonths(start, end time.Time) int {
	months := (end.Year()-start.Year())*12 + int(end.Month()) - int(start.Month())
	if end.Day() < start.Day() {
		months--
	}
	return max(months, 0)
}

// SLIKFacilities are the customer's received SLIK facilities grouped by
// condition, ordered by its code, for Isi Data dari SLIK (FR-APP-017 rules
// 1-6); ErrUnsupported when the button does not show.
func (s *Service) SLIKFacilities(ctx context.Context, sess auth.Session, caseID string, id int64) ([]SLIKFacilityGroup, error) {
	if err := s.mayView(ctx, sess, caseID, id); err != nil {
		return nil, err
	}
	available, err := slikAvailable(ctx, s.db, caseID)
	if err != nil {
		return nil, err
	}
	if !available {
		return nil, ErrUnsupported
	}
	// One facility as read: the condition (code and kondisi_ket), the
	// amounts as SLIK has them and rounded to rupiah, and the card's sources.
	type facility struct {
		condition, conditionDescription                string
		bank, financingType, amount, outstanding       *string
		roundedAmount, roundedOutstanding, rate, usage *string
		quality                                        *int
		started, due                                   *time.Time
	}
	rows, err := collect(ctx, s, func(r pgx.Rows) (facility, error) {
		var x facility
		return x, r.Scan(&x.condition, &x.conditionDescription, &x.bank, &x.financingType, &x.amount, &x.outstanding, &x.roundedAmount,
			&x.roundedOutstanding, &x.rate, &x.quality, &x.usage, &x.started, &x.due)
	}, `
		SELECT COALESCE(f.condition, ''), COALESCE(f.condition_description, ''), f.financial_institution_description, f.credit_financing_type_description,
		       f.initial_credit_limit::numeric(20, 2)::text, f.outstanding_principal::numeric(20, 2)::text,
		       round(f.initial_credit_limit)::text, round(f.outstanding_principal)::text, f.interest_rate::numeric(20, 2)::text,
		       f.quality, f.usage_type_description, f.credit_start_date, f.due_date
		FROM fos.slik_facility f JOIN fos.slik_report r ON r.slik_report_id = f.slik_report_id
		WHERE r.case_id = $1 AND r.subject_type = $2 AND r.status = $3
		ORDER BY NULLIF(regexp_replace(COALESCE(f.condition, ''), '[^0-9]', '', 'g'), '')::integer NULLS LAST, f.condition, f.sequence_number`,
		caseID, subjectCustomer, slikReceived)
	if err != nil {
		return nil, err
	}
	groups := []SLIKFacilityGroup{}
	at := map[string]int{}
	for _, x := range rows {
		card := OtherFinancing{BankName: value(x.bank), FinancingType: value(x.financingType), FinancingAmount: value(x.roundedAmount),
			OutstandingAmount: value(x.roundedOutstanding), MarginRate: value(x.rate), Purpose: value(x.usage)}
		if x.started != nil && x.due != nil {
			card.TenorMonths = strconv.Itoa(fullMonths(*x.started, *x.due))
		}
		if x.quality != nil {
			card.SLIKCollect = strconv.Itoa(*x.quality)
		}
		// One accordion per condition; titled by kondisi_ket, or by the code
		// when it has none.
		i, ok := at[x.condition]
		if !ok {
			i = len(groups)
			at[x.condition] = i
			title := x.conditionDescription
			if title == "" {
				title = x.condition
			}
			groups = append(groups, SLIKFacilityGroup{Title: title})
		}
		groups[i].Facilities = append(groups[i].Facilities, SLIKFacility{FinancingAmount: value(x.amount), OutstandingAmount: value(x.outstanding),
			MarginRate: value(x.rate), Card: card})
	}
	for i := range groups {
		groups[i].Title = "Fasilitas " + groups[i].Title + " (" + strconv.Itoa(len(groups[i].Facilities)) + ")"
	}
	return groups, nil
}
