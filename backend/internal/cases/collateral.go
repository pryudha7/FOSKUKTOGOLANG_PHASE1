package cases

import (
	"context"
	"strconv"
	"strings"
	"time"

	"fos/internal/auth"
)

// Collateral Data, the fifth Hot Prospect step (FR-APP-021, UI-034): the
// Agunan Utama and Jaminan Pendukung cards, each shaped by its collateral
// type.
const (
	AssignmentCollateralData = "Collateral Data"

	// The collateral types (BMV MAINCOLLATERAL, COLLATERALSUPPORT codes).
	collateralLandBuilding = "Tanah dan Bangunan"
	collateralDeposit      = "Deposito Bank Nano Syariah"
	collateralVacantLand   = "Tanah Kosong Produktif"
	collateralVehicle      = "Kendaraan"

	// Status Paripasu Ya (BMV STATUS_PARIPASU).
	pariPassuYes = "Y"

	maxCardsSetting     = "validation/collateral/max_cards_per_section"
	mainRequiredSetting = "validation/collateral/main_required_above_amount"
	earliestVehicleYear = 1900
	shgbValidityYears   = 100
)

// collateralSection is one section of the page: its §9.48
// collateral_section, the BMV type and options key of its collateral
// types, its label and the banner section prefix of its cards.
type collateralSection struct {
	code, typeBMV, typeKey, label, errorPrefix string
}

var (
	mainSection        = collateralSection{"MAIN", bmvMainCollateral, "mainCollateral", "Agunan Utama", "main-collateral-"}
	supportSection     = collateralSection{"SUPPORT", bmvSupportCollateral, "supportCollateral", "Jaminan Pendukung", "support-collateral-"}
	collateralSections = []collateralSection{mainSection, supportSection}
)

// Collateral is one card (FR-APP-021 Field A-F): codes, texts, amounts and
// years as digits, dates as yyyy-mm-dd; "" when empty.
type Collateral struct {
	TypeCode, BuildingFormCode, OwnershipTypeCode, OwnerName                  string
	SHMNumber, SHGBNumber, SHGBValidUntil, BPKBNumber, FiduciaryNumber        string
	Location, LandArea, DueDate, PhysicalValue, ObjectValue, DepositAmount    string
	VehicleBrand, VehicleYear, Remarks                                        string
	CollateralStatusCode, CollateralTypeText, BindingType, BindingDate        string
	AppraisalDate, KJPPAppraisalDate, NJOPValue, KJPPValue, KJPPAppraiserName string
	PariPassuStatusCode, PariPassuPercent, JointFinancingCode, InsuredCode    string
}

// collateralColumns map each §9.48 column to its card field, with the
// type a text value is cast to on insert.
var collateralColumns = []struct {
	column, cast string
	field        func(*Collateral) *string
}{
	{"collateral_type_code", "varchar", func(c *Collateral) *string { return &c.TypeCode }},
	{"building_form_code", "varchar", func(c *Collateral) *string { return &c.BuildingFormCode }},
	{"ownership_type_code", "varchar", func(c *Collateral) *string { return &c.OwnershipTypeCode }},
	{"owner_name", "varchar", func(c *Collateral) *string { return &c.OwnerName }},
	{"shm_number", "varchar", func(c *Collateral) *string { return &c.SHMNumber }},
	{"shgb_number", "varchar", func(c *Collateral) *string { return &c.SHGBNumber }},
	{"shgb_valid_until", "date", func(c *Collateral) *string { return &c.SHGBValidUntil }},
	{"bpkb_number", "varchar", func(c *Collateral) *string { return &c.BPKBNumber }},
	{"fiduciary_number", "varchar", func(c *Collateral) *string { return &c.FiduciaryNumber }},
	{"location", "varchar", func(c *Collateral) *string { return &c.Location }},
	{"land_area", "varchar", func(c *Collateral) *string { return &c.LandArea }},
	{"due_date", "date", func(c *Collateral) *string { return &c.DueDate }},
	{"physical_value", "numeric", func(c *Collateral) *string { return &c.PhysicalValue }},
	{"object_value", "numeric", func(c *Collateral) *string { return &c.ObjectValue }},
	{"deposit_amount", "numeric", func(c *Collateral) *string { return &c.DepositAmount }},
	{"vehicle_brand", "varchar", func(c *Collateral) *string { return &c.VehicleBrand }},
	{"vehicle_year", "smallint", func(c *Collateral) *string { return &c.VehicleYear }},
	{"remarks", "varchar", func(c *Collateral) *string { return &c.Remarks }},
	{"collateral_status_code", "varchar", func(c *Collateral) *string { return &c.CollateralStatusCode }},
	{"collateral_type_text", "varchar", func(c *Collateral) *string { return &c.CollateralTypeText }},
	{"binding_type", "varchar", func(c *Collateral) *string { return &c.BindingType }},
	{"binding_date", "date", func(c *Collateral) *string { return &c.BindingDate }},
	{"appraisal_date", "date", func(c *Collateral) *string { return &c.AppraisalDate }},
	{"kjpp_appraisal_date", "date", func(c *Collateral) *string { return &c.KJPPAppraisalDate }},
	{"njop_value", "numeric", func(c *Collateral) *string { return &c.NJOPValue }},
	{"kjpp_value", "numeric", func(c *Collateral) *string { return &c.KJPPValue }},
	{"kjpp_appraiser_name", "varchar", func(c *Collateral) *string { return &c.KJPPAppraiserName }},
	{"pari_passu_status_code", "varchar", func(c *Collateral) *string { return &c.PariPassuStatusCode }},
	{"pari_passu_percent", "smallint", func(c *Collateral) *string { return &c.PariPassuPercent }},
	{"joint_financing_code", "varchar", func(c *Collateral) *string { return &c.JointFinancingCode }},
	{"insured_code", "varchar", func(c *Collateral) *string { return &c.InsuredCode }},
}

// The BMV type of each coded field.
const (
	bmvMainCollateral    = "MAINCOLLATERAL"
	bmvSupportCollateral = "COLLATERALSUPPORT"
	bmvBuildingForm      = "BENTUKTANAHBANGUNAN"
	bmvOwnershipType     = "OWNERSHIPTYPE"
	bmvCollateralStatus  = "COLLATERAL_STATUS"
	bmvJointFinancing    = "COLLATERAL_JOINT_ACCOUNT"
	bmvPariPassuStatus   = "STATUS_PARIPASU"
	bmvInsured           = "COLLATERAL_ASURANSI"
)

// codedField is a coded field of a card: its BMV type, the page options
// key and the code.
type codedField struct {
	bmvType, key, code string
}

// detailCodes are the coded fields other than the collateral type.
var detailCodes = []struct {
	bmvType, key string
	field        func(*Collateral) *string
}{
	{bmvBuildingForm, "buildingForm", func(c *Collateral) *string { return &c.BuildingFormCode }},
	{bmvOwnershipType, "ownershipType", func(c *Collateral) *string { return &c.OwnershipTypeCode }},
	{bmvCollateralStatus, "collateralStatus", func(c *Collateral) *string { return &c.CollateralStatusCode }},
	{bmvJointFinancing, "jointFinancing", func(c *Collateral) *string { return &c.JointFinancingCode }},
	{bmvPariPassuStatus, "pariPassuStatus", func(c *Collateral) *string { return &c.PariPassuStatusCode }},
	{bmvInsured, "insured", func(c *Collateral) *string { return &c.InsuredCode }},
}

// collateralCodes are a card's coded fields.
func collateralCodes(c *Collateral, s collateralSection) []codedField {
	out := []codedField{{s.typeBMV, s.typeKey, c.TypeCode}}
	for _, d := range detailCodes {
		out = append(out, codedField{d.bmvType, d.key, *d.field(c)})
	}
	return out
}

// collateralOptions are the BMV types of the page's dropdowns by options
// key.
func collateralOptions() map[string]string {
	out := map[string]string{}
	for _, s := range collateralSections {
		out[s.typeKey] = s.typeBMV
	}
	for _, d := range detailCodes {
		out[d.key] = d.bmvType
	}
	return out
}

// CollateralForm is Collateral Data: the cards of each section in screen
// order.
type CollateralForm struct {
	Main, Support []Collateral
}

// cards are the cards of a section.
func (f *CollateralForm) cards(s collateralSection) *[]Collateral {
	if s == supportSection {
		return &f.Support
	}
	return &f.Main
}

// CollateralPage is UI-034: the saved cards (one empty card per section
// when the case has none), the options (saved codes now Inactive or
// removed added, BR-UI-012), the card limit and the operator's today.
type CollateralPage struct {
	Values CollateralForm
	PageOptions
	MaxCards                    int64
	Today                       string
	FacilityRequested, Editable bool
}

// savedCollaterals are the case's cards by section, in sequence order.
func savedCollaterals(ctx context.Context, q querier, caseID string) (CollateralForm, error) {
	cols := make([]string, len(collateralColumns))
	for i, col := range collateralColumns {
		cols[i] = col.column + "::text"
	}
	rows, err := q.Query(ctx, `SELECT collateral_section, `+strings.Join(cols, ", ")+`
		FROM fos.case_collateral WHERE case_id = $1 ORDER BY collateral_section, sequence_number`, caseID)
	if err != nil {
		return CollateralForm{}, err
	}
	defer rows.Close()
	var form CollateralForm
	for rows.Next() {
		var section string
		values := make([]*string, len(collateralColumns))
		dest := []any{&section}
		for i := range values {
			dest = append(dest, &values[i])
		}
		if err := rows.Scan(dest...); err != nil {
			return CollateralForm{}, err
		}
		var card Collateral
		for i, col := range collateralColumns {
			*col.field(&card) = value(values[i])
		}
		for _, s := range collateralSections {
			if s.code == section {
				*form.cards(s) = append(*form.cards(s), card)
			}
		}
	}
	return form, rows.Err()
}

// savedCodes are the BMV codes the case's cards hold, by type: they pass on
// a new submit even when no longer Active (BR-UI-012, agreed comment 10).
func savedCodes(form CollateralForm) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, s := range collateralSections {
		cards := *form.cards(s)
		for i := range cards {
			for _, code := range collateralCodes(&cards[i], s) {
				if code.code == "" {
					continue
				}
				if out[code.bmvType] == nil {
					out[code.bmvType] = map[string]bool{}
				}
				out[code.bmvType][code.code] = true
			}
		}
	}
	return out
}

func (s *Service) collateralPage(ctx context.Context, sess auth.Session, caseID string, editable bool) (CollateralPage, error) {
	form, err := savedCollaterals(ctx, s.db, caseID)
	if err != nil {
		return CollateralPage{}, err
	}
	page := CollateralPage{Values: form, Editable: editable}
	page.PageOptions = s.pageOptions(ctx, collateralOptions())
	for _, sec := range collateralSections {
		cards := *form.cards(sec)
		for i := range cards {
			for _, code := range collateralCodes(&cards[i], sec) {
				if err := s.keepSavedOption(ctx, &page.PageOptions, code.key, code.bmvType, code.code); err != nil {
					return CollateralPage{}, err
				}
			}
		}
	}
	// Process 1: one empty card per section when the case has none.
	if len(form.Main) == 0 && len(form.Support) == 0 {
		page.Values = CollateralForm{Main: []Collateral{{}}, Support: []Collateral{{}}}
	}
	if page.MaxCards, err = s.settings.Int(ctx, maxCardsSetting); err != nil {
		return CollateralPage{}, err
	}
	today, err := s.operatorToday(ctx, s.db, sess.UserID)
	if err != nil {
		return CollateralPage{}, err
	}
	page.Today = today.Format(dateISO)
	page.FacilityRequested, err = facilityRequested(ctx, s.db, caseID)
	return page, err
}

// keepSavedOption adds a saved code missing from its options (Inactive or
// removed from BMV) so its description, or the code itself, still shows
// (BR-UI-012).
func (s *Service) keepSavedOption(ctx context.Context, opts *PageOptions, key, bmvType, code string) error {
	if code == "" {
		return nil
	}
	for _, o := range opts.Options[key] {
		if o.Code == code {
			return nil
		}
	}
	saved, err := s.bmvDescription(ctx, bmvType, code)
	if err != nil {
		return err
	}
	if saved.Description == "" {
		saved.Description = code
	}
	opts.Options[key] = append(opts.Options[key], *saved)
	return nil
}

// collateralCheck checks one card at a time; saved are the codes that pass
// though not Active, today the operator's date.
type collateralCheck struct {
	*fieldCheck
	saved map[string]map[string]bool
	today time.Time
}

func (c collateralCheck) code(key, label, bmvType, v string, mandatory bool) {
	if v != "" && c.saved[bmvType][v] {
		return
	}
	c.fieldCheck.code(key, label, bmvType, v, mandatory)
}

// date is a valid yyyy-mm-dd, any day (FR-APP-021 rule 2).
func (c collateralCheck) date(key, label, v string) {
	if !c.filled(key, label, v, false) {
		return
	}
	if _, err := time.Parse(dateISO, v); err != nil {
		c.invalid(key, label)
	}
}

// shgbValidUntil is Berlaku Sampai: after today, at most 100 years ahead
// (rule 3).
func (c collateralCheck) shgbValidUntil(v string) {
	const key, label = "shgbValidUntil", "Berlaku Sampai"
	if !c.filled(key, label, v, false) {
		return
	}
	d, err := time.Parse(dateISO, v)
	if err != nil || !d.After(c.today) || d.After(c.today.AddDate(shgbValidityYears, 0, 0)) {
		c.invalid(key, label)
	}
}

// vehicleYear is Tahun Pembuatan: four digits, 1900 to next year (rule 5).
func (c collateralCheck) vehicleYear(v string) {
	const key, label = "vehicleYear", "Tahun Pembuatan"
	if !c.filled(key, label, v, false) {
		return
	}
	year, err := strconv.Atoi(v)
	if !digits.MatchString(v) || len(v) != 4 || err != nil || year < earliestVehicleYear || year > c.today.Year()+1 {
		c.invalid(key, label)
	}
}

func (c collateralCheck) money(key, label, v string) {
	c.fieldCheck.money(key, label, v, false)
}

// card checks one card in its screen order and returns what is stored:
// only the fields of its type, and of its Tipe Bukti Kepemilikan, the
// others empty (FR-APP-021 Field A-F, Validation, DB-RULE-127).
func (c collateralCheck) card(s collateralSection, in Collateral) Collateral {
	out := Collateral{TypeCode: in.TypeCode}
	before := len(c.errs)
	c.code("typeCode", s.label, s.typeBMV, in.TypeCode, true)
	if len(c.errs) > before || c.err != nil {
		return out
	}
	switch in.TypeCode {
	case collateralLandBuilding:
		c.code("buildingFormCode", "Tanah dan Bangunan Yang Berbentuk", bmvBuildingForm, in.BuildingFormCode, true)
		out.BuildingFormCode = in.BuildingFormCode
		c.ownership(in, &out)
		c.text("ownerName", "Nama Pemilik", in.OwnerName, false, 100)
		out.OwnerName = in.OwnerName
		c.ownershipNumber(in, &out)
		c.money("physicalValue", "Nilai Aktual Fisik (LPA UTKP)", in.PhysicalValue)
		c.text("location", "Lokasi", in.Location, false, 255)
		out.PhysicalValue, out.Location = in.PhysicalValue, in.Location
		c.details(in, &out, true)
		c.text("remarks", "Agunan diperuntukan dan Jarak lokasi usaha dari kantor cabang Bank Nano Syariah", in.Remarks, false, 500)
		out.Remarks = in.Remarks
	case collateralDeposit:
		c.money("depositAmount", "Jumlah Deposito Bank Nano Syariah", in.DepositAmount)
		out.DepositAmount = in.DepositAmount
	case collateralVacantLand:
		c.ownership(in, &out)
		c.ownershipNumber(in, &out)
		c.text("ownerName", "Atas Nama", in.OwnerName, false, 100)
		c.text("location", "Lokasi", in.Location, false, 255)
		c.text("landArea", "Luas Tanah", in.LandArea, false, 50)
		c.date("dueDate", "Jatuh Tempo", in.DueDate)
		out.OwnerName, out.Location, out.LandArea, out.DueDate = in.OwnerName, in.Location, in.LandArea, in.DueDate
		c.details(in, &out, false)
	case collateralVehicle:
		c.text("bpkbNumber", "BPKB No.", in.BPKBNumber, false, 50)
		c.text("ownerName", "Atas Nama", in.OwnerName, false, 100)
		c.money("objectValue", "Nilai Objek Sebesar", in.ObjectValue)
		c.text("vehicleBrand", "Jenis / Merek Kendaraan", in.VehicleBrand, false, 100)
		c.vehicleYear(in.VehicleYear)
		out.BPKBNumber, out.OwnerName, out.ObjectValue, out.VehicleBrand, out.VehicleYear = in.BPKBNumber, in.OwnerName, in.ObjectValue, in.VehicleBrand, in.VehicleYear
		c.details(in, &out, false)
	}
	return out
}

func (c collateralCheck) ownership(in Collateral, out *Collateral) {
	c.code("ownershipTypeCode", "Tipe Bukti Kepemilikan", bmvOwnershipType, in.OwnershipTypeCode, false)
	out.OwnershipTypeCode = in.OwnershipTypeCode
}

// ownershipNumber is the number field of the Tipe Bukti Kepemilikan
// (Field E): 1 and 4 Nomor SHM, 2 Nomor SHGB and Berlaku Sampai, 3 Nomor
// BPKB, 5 Nomor Fidusia.
func (c collateralCheck) ownershipNumber(in Collateral, out *Collateral) {
	switch in.OwnershipTypeCode {
	case "1", "4":
		c.text("shmNumber", "Nomor SHM", in.SHMNumber, false, 50)
		out.SHMNumber = in.SHMNumber
	case "2":
		c.text("shgbNumber", "Nomor SHGB", in.SHGBNumber, false, 50)
		c.shgbValidUntil(in.SHGBValidUntil)
		out.SHGBNumber, out.SHGBValidUntil = in.SHGBNumber, in.SHGBValidUntil
	case "3":
		c.text("bpkbNumber", "Nomor BPKB", in.BPKBNumber, false, 50)
		out.BPKBNumber = in.BPKBNumber
	case "5":
		c.text("fiduciaryNumber", "Nomor Fidusia", in.FiduciaryNumber, false, 50)
		out.FiduciaryNumber = in.FiduciaryNumber
	}
}

// details is the white card of collateral details (Field F), in its
// screen order; Status Paripasu is required on Tanah dan Bangunan;
// Presentase Paripasu is 1 to 100 and required with Status Paripasu Y
// (empty on Save stays empty), else stored 0 (rule 4, DB-RULE-127).
func (c collateralCheck) details(in Collateral, out *Collateral, pariPassuRequired bool) {
	c.code("collateralStatusCode", "Status Agunan", bmvCollateralStatus, in.CollateralStatusCode, false)
	c.text("collateralTypeText", "Jenis Agunan", in.CollateralTypeText, false, 100)
	c.text("bindingType", "Jenis Pengikatan", in.BindingType, false, 100)
	c.date("bindingDate", "Tanggal Pengikatan", in.BindingDate)
	c.money("njopValue", "Nilai Agunan Sesuai NJOP / Nilai Wajar", in.NJOPValue)
	c.date("appraisalDate", "Tanggal Penilaian", in.AppraisalDate)
	c.money("kjppValue", "Nilai Agunan KJPP", in.KJPPValue)
	c.date("kjppAppraisalDate", "Tanggal Penilaian KJPP", in.KJPPAppraisalDate)
	c.text("kjppAppraiserName", "Nama Penilaian KJPP", in.KJPPAppraiserName, false, 100)
	out.PariPassuPercent = "0"
	if in.PariPassuStatusCode == pariPassuYes {
		c.positive("pariPassuPercent", "Presentase Paripasu", in.PariPassuPercent, 100)
		out.PariPassuPercent = in.PariPassuPercent
	}
	c.code("jointFinancingCode", "Status Agunan Join Financing", bmvJointFinancing, in.JointFinancingCode, false)
	c.code("pariPassuStatusCode", "Status Paripasu", bmvPariPassuStatus, in.PariPassuStatusCode, pariPassuRequired)
	c.code("insuredCode", "Diasuransikan ?", bmvInsured, in.InsuredCode, false)
	out.CollateralStatusCode, out.CollateralTypeText, out.BindingType, out.BindingDate = in.CollateralStatusCode, in.CollateralTypeText, in.BindingType, in.BindingDate
	out.NJOPValue, out.AppraisalDate, out.KJPPValue, out.KJPPAppraisalDate = in.NJOPValue, in.AppraisalDate, in.KJPPValue, in.KJPPAppraisalDate
	out.KJPPAppraiserName, out.JointFinancingCode, out.PariPassuStatusCode, out.InsuredCode = in.KJPPAppraiserName, in.JointFinancingCode, in.PariPassuStatusCode, in.InsuredCode
}

func trimCollateral(in Collateral) Collateral {
	for _, col := range collateralColumns {
		f := col.field(&in)
		*f = strings.TrimSpace(*f)
	}
	return in
}

// rupiah is "Rp100.000.000" (BR-UI-009).
func rupiah(n int64) string {
	s := strconv.FormatInt(n, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return "Rp" + s
}

// SubmitCollateral validates and stores Collateral Data (FR-APP-021
// Process 6, agreed comments 3-5): cards without a type are dropped, the
// rest checked Agunan Utama first, then Jaminan Pendukung; a forward move
// needs an Agunan Utama above the Total Requested Amount threshold
// (ERR-044); the case's rows are replaced (DB-RULE-127), then the form
// moves to `to` or stays on Save.
func (s *Service) SubmitCollateral(ctx context.Context, sess auth.Session, caseID string, id int64, to string, f CollateralForm) (int64, error) {
	return s.moveHotProspect(ctx, sess, caseID, id, AssignmentCollateralData, to, func(fc *fieldCheck) error {
		saved, err := savedCollaterals(fc.ctx, fc.tx, caseID)
		if err != nil {
			return err
		}
		today, err := s.operatorToday(fc.ctx, fc.tx, sess.UserID)
		if err != nil {
			return err
		}
		maxCards, err := s.settings.Int(fc.ctx, maxCardsSetting)
		if err != nil {
			return err
		}
		c := collateralCheck{fieldCheck: fc, saved: savedCodes(saved), today: today}
		var kept CollateralForm
		for _, sec := range collateralSections {
			cards := *f.cards(sec)
			var typed []int
			for i, card := range cards {
				if strings.TrimSpace(card.TypeCode) != "" {
					typed = append(typed, i)
				}
			}
			c.section = SectionForm
			if int64(len(typed)) > maxCards {
				c.invalid(sec.typeKey, sec.label)
				continue
			}
			for _, i := range typed {
				c.section = sec.errorPrefix + strconv.Itoa(i+1)
				*kept.cards(sec) = append(*kept.cards(sec), c.card(sec, trimCollateral(cards[i])))
			}
		}
		c.section = SectionForm
		if c.err != nil || len(c.errs) > 0 {
			return c.err
		}
		if stepAfter(to, AssignmentCollateralData) && len(kept.Main) == 0 {
			if err := s.mainCollateralRequired(c.fieldCheck, caseID); err != nil || len(c.errs) > 0 {
				return err
			}
		}
		return saveCollaterals(c.fieldCheck, caseID, kept)
	})
}

// mainCollateralRequired is rule 1: above the threshold Total Requested
// Amount (§9.42; 0 without a row) an Agunan Utama is needed (ERR-044).
func (s *Service) mainCollateralRequired(c *fieldCheck, caseID string) error {
	threshold, err := s.settings.Int(c.ctx, mainRequiredSetting)
	if err != nil {
		return err
	}
	var above bool
	if err := c.tx.QueryRow(c.ctx, `
		SELECT COALESCE((SELECT total_requested_amount FROM fos.case_financing WHERE case_id = $1), 0) > $2`,
		caseID, threshold).Scan(&above); err != nil {
		return err
	}
	if above {
		c.add(mainSection.typeKey, mainSection.label, "ERR-044", "Agunan Utama minimal 1 untuk total nilai pengajuan lebih dari "+rupiah(threshold)+".")
	}
	return nil
}

// saveCollaterals replaces the case's rows with the kept cards, numbered
// per section in screen order (DB-RULE-127).
func saveCollaterals(c *fieldCheck, caseID string, form CollateralForm) error {
	if _, err := c.tx.Exec(c.ctx, `DELETE FROM fos.case_collateral WHERE case_id = $1`, caseID); err != nil {
		return err
	}
	cols, params := make([]string, len(collateralColumns)), make([]string, len(collateralColumns))
	for i, col := range collateralColumns {
		cols[i] = col.column
		params[i] = "$" + strconv.Itoa(i+4) + "::" + col.cast
	}
	insert := `INSERT INTO fos.case_collateral (case_id, collateral_section, sequence_number, ` + strings.Join(cols, ", ") + `)
		VALUES ($1, $2, $3, ` + strings.Join(params, ", ") + `)`
	for _, sec := range collateralSections {
		cards := *form.cards(sec)
		for i := range cards {
			args := []any{caseID, sec.code, i + 1}
			for _, col := range collateralColumns {
				args = append(args, orNil(*col.field(&cards[i])))
			}
			if _, err := c.tx.Exec(c.ctx, insert, args...); err != nil {
				return err
			}
		}
	}
	return nil
}
