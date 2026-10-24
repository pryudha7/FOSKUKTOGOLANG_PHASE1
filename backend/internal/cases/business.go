package cases

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"fos/internal/auth"
)

// Prospect Data Entry Badan Usaha (FR-APP-005, UI-025): one assignment; Next
// ends Prospect Data Entry (agreed comment 1).
const (
	// partyBusinessEntity is the customer_type and address_type of the
	// entity itself (DB-RULE-099).
	partyBusinessEntity = customerTypeBusinessEntity
	partyCompanyOfficer = "Pengurus"

	// maxCompanyOfficers bounds Data Pengurus (FR-APP-002).
	maxCompanyOfficers = 10

	msgNoRole             = "Harap isi jabatan"                  // ERR-014
	msgNoCompanyOfficers  = "Minimal 1 data pengurus dibutuhkan" // ERR-015
	sectionCompanyOfficer = "company-officer-"                   // + card number from 1
)

// CompanyOfficer is one Data Pengurus card (FR-APP-002); BirthDate is yyyy-mm-dd.
type CompanyOfficer struct {
	RoleCode, NIK, Name, BirthDate string
}

// BusinessEntityForm is the Badan Usaha form (FR-APP-005 Field 2-12);
// LastDeedDate is yyyy-mm-dd and GoPublic the BMV description (§9.6).
type BusinessEntityForm struct {
	BusinessEntityCode, PlaceOfEstablishment, DeedNumber, LastDeedNumber, LastDeedDate string
	Phone, MobilePhone, Email, EconomicSectorCode, RelationshipWithBankCode            string
	BusinessTypeCode, CustomerCategoryCode, Address, GoPublic                          string
	Region
	BusinessGroupName string
	CompanyOfficers   []CompanyOfficer
}

// BusinessEntityPage is UI-025: the Pre-Screening identity, the customer
// status, the saved values and Pengurus, and the initial dropdowns.
type BusinessEntityPage struct {
	NPWP, Name, EstablishmentDate string
	ExistingCustomer              bool
	Values                        BusinessEntityForm
	// Sector and Category are the saved autocompletes, shown by description.
	Sector, Category *Option
	PageOptions
	Editable bool
	// Today and EarliestBirthDate bound the Pengurus birth dates;
	// LatestDeedDate is the last day Tanggal Akta Perubahan Terakhir allows
	// (yesterday), all the operator's dates.
	Today, EarliestBirthDate, LatestDeedDate string
}

// companyOfficerSection is the banner section of card n: "Pengurus <n> - ".
func companyOfficerSection(n int) string { return sectionCompanyOfficer + strconv.Itoa(n) }

// bmvDescription reads the description of a BMV code, whatever its status
// (BR-UI-012), or nil without a code.
func (s *Service) bmvDescription(ctx context.Context, bmvType, code string) (*Option, error) {
	if code == "" {
		return nil, nil
	}
	var description string
	err := s.db.QueryRow(ctx, `SELECT description FROM fos.business_mapping_value WHERE type = $1 AND sub_type = '' AND code = $2`,
		bmvType, code).Scan(&description)
	if err != nil && !isNotFound(err) {
		return nil, err
	}
	return &Option{code, description}, nil
}

func (s *Service) businessEntityPage(ctx context.Context, sess auth.Session, caseID string, editable bool) (BusinessEntityPage, error) {
	page := BusinessEntityPage{Editable: editable}
	var (
		id                                                                          int64
		npwp, name, established, entity, place, deed, lastDeed, lastDeedDate, email *string
		sector, relation, businessType, category, goPublic, group                   *string
	)
	if err := s.db.QueryRow(ctx, `
		SELECT customer_id, npwp, full_name, to_char(establishment_date, 'YYYY-MM-DD'), business_entity_code,
		       place_of_establishment, deed_number, last_deed_number, to_char(last_deed_date, 'YYYY-MM-DD'), email,
		       economic_sector_code, relationship_with_bank_code, business_type_code, customer_category_code, go_public, business_group_name
		FROM fos.customer WHERE case_id = $1 AND customer_type = $2`, caseID, partyBusinessEntity).
		Scan(&id, &npwp, &name, &established, &entity, &place, &deed, &lastDeed, &lastDeedDate, &email,
			&sector, &relation, &businessType, &category, &goPublic, &group); err != nil {
		return BusinessEntityPage{}, err
	}
	page.NPWP, page.Name, page.EstablishmentDate = value(npwp), value(name), value(established)
	existing, err := existingCustomer(ctx, s.db, page.NPWP, customerTypeBusinessEntity)
	if err != nil {
		return BusinessEntityPage{}, err
	}
	page.ExistingCustomer = existing
	// Stored on every opening by the operator working on it (FR-APP-005 rule 1).
	if editable {
		if _, err := s.db.Exec(ctx, `UPDATE fos.customer SET is_existing_customer = $2 WHERE customer_id = $1`, id, existing); err != nil {
			return BusinessEntityPage{}, err
		}
	}
	address, err := savedAddress(ctx, s.db, id, partyBusinessEntity)
	if err != nil {
		return BusinessEntityPage{}, err
	}
	page.Values = BusinessEntityForm{
		BusinessEntityCode: value(entity), PlaceOfEstablishment: value(place), DeedNumber: value(deed), LastDeedNumber: value(lastDeed),
		LastDeedDate: value(lastDeedDate), Phone: address.Phone, MobilePhone: address.MobilePhone, Email: value(email),
		EconomicSectorCode: value(sector), RelationshipWithBankCode: value(relation), BusinessTypeCode: value(businessType),
		CustomerCategoryCode: value(category), Address: address.Address, GoPublic: value(goPublic), Region: address.Region,
		BusinessGroupName: value(group),
	}
	// Pengurus are the Pre-Screening rows (DEC-112); none gives one empty card.
	page.Values.CompanyOfficers, err = collect(ctx, s, func(r pgx.Rows) (CompanyOfficer, error) {
		var o CompanyOfficer
		var role, nik, name, born *string
		err := r.Scan(&role, &nik, &name, &born)
		o = CompanyOfficer{value(role), value(nik), value(name), value(born)}
		return o, err
	}, `
		SELECT role_code, id_number, full_name, to_char(birth_date, 'YYYY-MM-DD') FROM fos.customer
		WHERE case_id = $1 AND customer_type = $2 ORDER BY sort_order, customer_id`, caseID, partyCompanyOfficer)
	if err != nil {
		return BusinessEntityPage{}, err
	}
	if len(page.Values.CompanyOfficers) == 0 {
		page.Values.CompanyOfficers = []CompanyOfficer{{}}
	}
	if page.Sector, err = s.bmvDescription(ctx, "ECONOMIC_SECTOR", page.Values.EconomicSectorCode); err != nil {
		return BusinessEntityPage{}, err
	}
	if page.Category, err = s.bmvDescription(ctx, "BUSINESSENTITY_CATEGORY", page.Values.CustomerCategoryCode); err != nil {
		return BusinessEntityPage{}, err
	}
	today, err := s.operatorToday(ctx, s.db, sess.UserID)
	if err != nil {
		return BusinessEntityPage{}, err
	}
	page.Today, page.EarliestBirthDate = today.Format(dateISO), EarliestBirthDate(today).Format(dateISO)
	page.LatestDeedDate = today.AddDate(0, 0, -1).Format(dateISO)
	page.PageOptions = s.pageOptions(ctx, map[string]string{
		"businessEntity": "BUSINESS_ENTITY", "relationshipWithBank": "RELATIONSHIP_WITH_BANK", "businessType": "BUSINESS_TYPE",
		"goPublic": "GO_PUBLIC_STATUS", "province": "PROVINCE", "businessRole": "BUSINESS_ROLE",
	})
	page.Options["goPublic"] = keyedByDescription(page.Options["goPublic"])
	return page, nil
}

// keyedByDescription makes each option's value its description, for a
// dropdown chosen and stored by description (Go Public, §9.6).
func keyedByDescription(opts []Option) []Option {
	out := make([]Option, len(opts))
	for i, o := range opts {
		out[i] = Option{o.Description, o.Description}
	}
	return out
}

// SubmitBusinessEntity validates and stores the Badan Usaha form and its
// Pengurus, then ends Prospect Data Entry (FR-APP-005 Process 4, agreed
// comment 1).
func (s *Service) SubmitBusinessEntity(ctx context.Context, sess auth.Session, caseID string, id int64, f BusinessEntityForm) error {
	if len(f.CompanyOfficers) > maxCompanyOfficers {
		return ErrUnsupported // more cards than the page allows
	}
	_, err := s.saveForm(ctx, sess, caseID, id, AssignmentBusinessEntity, func(c *fieldCheck) error {
		t := strings.TrimSpace
		today, err := s.operatorToday(c.ctx, c.tx, sess.UserID)
		if err != nil {
			return err
		}
		region := trimRegion(f.Region)
		c.code("businessEntity", "Kode Badan Usaha", "BUSINESS_ENTITY", t(f.BusinessEntityCode), false)
		c.text("placeOfEstablishment", "Tempat Pendirian", t(f.PlaceOfEstablishment), true, 50)
		c.text("deedNumber", "Nomor Akta Pendirian", t(f.DeedNumber), true, 50)
		c.text("lastDeedNumber", "Nomor Akta Perubahan Terakhir", t(f.LastDeedNumber), false, 50)
		c.pastDate("lastDeedDate", "Tanggal Akta Perubahan Terakhir", t(f.LastDeedDate), today)
		c.phone("phone", "Nomor Telepon", t(f.Phone), true)
		c.phone("mobilePhone", "Nomor HP", t(f.MobilePhone), false)
		c.email("email", "Alamat Email", t(f.Email))
		c.code("economicSector", "Kode Bidang Usaha", "ECONOMIC_SECTOR", t(f.EconomicSectorCode), true)
		c.code("relationshipWithBank", "Hubungan dengan Bank", "RELATIONSHIP_WITH_BANK", t(f.RelationshipWithBankCode), false)
		c.code("businessType", "Tipe Bisnis", "BUSINESS_TYPE", t(f.BusinessTypeCode), true)
		c.code("customerCategory", "Golongan Nasabah", "BUSINESSENTITY_CATEGORY", t(f.CustomerCategoryCode), true)
		c.text("address", "Alamat", t(f.Address), true, 350)
		c.description("goPublic", "Go Public", "GO_PUBLIC_STATUS", t(f.GoPublic))
		resolved := c.region(region, true)
		c.text("businessGroupName", "Nama Group Usaha Nasabah", t(f.BusinessGroupName), false, 100)
		officers := c.companyOfficers(f.CompanyOfficers, today)
		if c.err != nil || len(c.errs) > 0 {
			return c.err
		}

		var customerID int64
		if err := c.tx.QueryRow(c.ctx, `
			UPDATE fos.customer SET business_entity_code = $3, place_of_establishment = $4, deed_number = $5, last_deed_number = $6,
				last_deed_date = $7::date, email = $8, economic_sector_code = $9, relationship_with_bank_code = $10,
				business_type_code = $11, customer_category_code = $12, go_public = $13, business_group_name = $14
			WHERE case_id = $1 AND customer_type = $2 RETURNING customer_id`,
			caseID, partyBusinessEntity, orNil(t(f.BusinessEntityCode)), t(f.PlaceOfEstablishment), t(f.DeedNumber),
			orNil(t(f.LastDeedNumber)), orNil(t(f.LastDeedDate)), orNil(t(f.Email)), t(f.EconomicSectorCode),
			orNil(t(f.RelationshipWithBankCode)), t(f.BusinessTypeCode), t(f.CustomerCategoryCode), t(f.GoPublic),
			orNil(t(f.BusinessGroupName))).Scan(&customerID); err != nil {
			return err
		}
		if err := saveAddress(c.ctx, c.tx, customerID, partyBusinessEntity, addressRow{address: orNil(t(f.Address)),
			phone: orNil(t(f.Phone)), mobile: orNil(t(f.MobilePhone)), region: resolved}); err != nil {
			return err
		}
		return saveCompanyOfficers(c.ctx, c.tx, caseID, officers)
	}, func(tx pgx.Tx, from int64) (int64, error) {
		return 0, s.enterFinancingApplication(ctx, tx, sess, caseID, from)
	})
	return err
}

// officers checks the Pengurus cards (FR-APP-002): Jabatan (ERR-014), the
// Pre-Screening person rules, and NIK unique across cards (ERR-018 on every
// duplicate); none at all is ERR-015. It returns the trimmed cards.
func (c *fieldCheck) companyOfficers(cards []CompanyOfficer, today time.Time) []CompanyOfficer {
	if len(cards) == 0 {
		c.errs = append(c.errs, FieldError{SectionForm, "companyOfficers", "Data Pengurus", "ERR-015", msgNoCompanyOfficers})
		return nil
	}
	t := strings.TrimSpace
	out := make([]CompanyOfficer, len(cards))
	seen := map[string]int{}
	for i, o := range cards {
		out[i] = CompanyOfficer{t(o.RoleCode), t(o.NIK), t(o.Name), t(o.BirthDate)}
		seen[out[i].NIK]++
	}
	for i, o := range out {
		c.section = companyOfficerSection(i + 1)
		if o.RoleCode == "" {
			c.add("role", "Jabatan", "ERR-014", msgNoRole)
		} else {
			c.code("role", "Jabatan", "BUSINESS_ROLE", o.RoleCode, true)
		}
		if e := nikError(c.section, KeyNIK, labelNIK, o.NIK); e != nil {
			c.addError(e)
		} else if seen[o.NIK] > 1 {
			c.add(KeyNIK, labelNIK, "ERR-018", msgSameNIK)
		}
		c.addError(nameError(c.section, KeyName, labelName, o.Name))
		c.addError(birthDateError(c.section, KeyBirthDate, labelBirthDate, o.BirthDate, today))
	}
	c.section = SectionForm
	return out
}

// saveCompanyOfficers writes the cards onto the case's Pengurus rows in order: the
// same rows are updated, extra cards inserted, rows of removed cards deleted
// (DEC-112, agreed comment 5).
func saveCompanyOfficers(ctx context.Context, tx pgx.Tx, caseID string, cards []CompanyOfficer) error {
	rows, err := collectIn(ctx, tx, func(r pgx.Rows) (int64, error) {
		var id int64
		return id, r.Scan(&id)
	}, `SELECT customer_id FROM fos.customer WHERE case_id = $1 AND customer_type = $2 ORDER BY sort_order, customer_id`, caseID, partyCompanyOfficer)
	if err != nil {
		return err
	}
	for i, o := range cards {
		if i < len(rows) {
			_, err = tx.Exec(ctx, `UPDATE fos.customer SET sort_order = $2, role_code = $3, id_number = $4, full_name = $5, birth_date = $6::date
				WHERE customer_id = $1`, rows[i], i+1, o.RoleCode, o.NIK, o.Name, o.BirthDate)
		} else {
			_, err = tx.Exec(ctx, `INSERT INTO fos.customer (case_id, customer_type, sort_order, role_code, id_number, full_name, birth_date)
				VALUES ($1, $2, $3, $4, $5, $6, $7::date)`, caseID, partyCompanyOfficer, i+1, o.RoleCode, o.NIK, o.Name, o.BirthDate)
		}
		if err != nil {
			return err
		}
	}
	for _, id := range rows[min(len(cards), len(rows)):] {
		if _, err := tx.Exec(ctx, `DELETE FROM fos.customer WHERE customer_id = $1`, id); err != nil {
			return err
		}
	}
	return nil
}
