package cases

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"fos/internal/auth"
)

// Data Permohonan Fasilitas, the third Hot Prospect step (FR-APP-018,
// UI-032).
const (
	AssignmentFacilityRequest = "Data Permohonan Fasilitas"
	AssignmentQCAData         = "QCA Data"

	// FacilityRequestNext is Next: Back to Previous Step and the breadcrumb
	// name their step, while Next also checks the product limit and records
	// the recommendation (agreed comments 7 and 8).
	FacilityRequestNext = "next"

	recommendationFacilityRequest = "Data Permohonan Fasilitas"
	recommendationRecommend       = "RECOMMEND" // BMV RECOMMENDED (DEC-145)
	limitActionAsIs               = "SESUAILMT" // BMV RECOMMEND_LIMIT (DEC-145)

	// ERR-042 and ERR-043 (FR-APP-018 Validation).
	msgBelowProductLimit = "Total nilai pengajuan tidak memenuhi batas bawah limit pengajuan."
	msgAboveProductLimit = "Total nilai pengajuan tidak memenuhi batas atas limit pengajuan."
	msgNoProductLimit    = "Gagal mendapatkan limit pembiayaan produk, harap coba lagi atau hubungi helpdesk jika berulang."

	maxRequestedAmount = 999999999999999
	maxTenureMonths    = 9999
	maxFacilityNote    = 1000
)

// FacilityRequestForm is Data Permohonan Fasilitas: amounts and months as
// digits, MarginRate with a dot and at most five decimals; the totals and
// the simulation are always computed by the server (DB-RULE-119).
type FacilityRequestForm struct {
	PurposeCode, RequestedAmount, TenureMonths, ExistingFinancingAmount string
	MarginTypeCode, MarginRate                                          string
	DeviationNote, MitigationNote, Notes                                string
}

// SimulationInput are the four fields the Financing Simulation needs (DEC-146).
type SimulationInput struct {
	RequestedAmount, TenureMonths, MarginTypeCode, MarginRate string
}

// FacilityRequestPage is UI-032: the saved values, Tipe Akad and what the
// Credit Simulation names (Produk Name, Nama Debitur).
type FacilityRequestPage struct {
	Values                                FacilityRequestForm
	ContractType, ProductName, DebtorName string
	PageOptions
	Editable bool
}

// applicantName is the applicant's name, Nama Debitur on the simulation.
func applicantName(ctx context.Context, q rowQuerier, caseID string) (string, error) {
	party, err := applicantParty(ctx, q, caseID)
	if err != nil {
		return "", err
	}
	var name *string
	err = q.QueryRow(ctx, `SELECT full_name FROM fos.customer WHERE case_id = $1 AND customer_type = $2`, caseID, party).Scan(&name)
	return value(name), err
}

func (s *Service) facilityRequestPage(ctx context.Context, caseID string, editable bool) (FacilityRequestPage, error) {
	page := FacilityRequestPage{Editable: editable}
	var contract, product *string
	if err := s.db.QueryRow(ctx, `
		SELECT w.contract_type, p.name FROM fos.work w LEFT JOIN fos.product_management p ON p.code = w.product_code
		WHERE w.case_id = $1`, caseID).Scan(&contract, &product); err != nil {
		return FacilityRequestPage{}, err
	}
	page.ContractType, page.ProductName = value(contract), value(product)
	name, err := applicantName(ctx, s.db, caseID)
	if err != nil {
		return FacilityRequestPage{}, err
	}
	page.DebtorName = name
	// One Tujuan Pembiayaan line on screen (DB-RULE-118); the margin typed
	// is the side its type names (FR-APP-018 Process 6).
	var purpose, amount, tenure, existing, deviation, mitigation, notes, marginType, margin *string
	err = s.db.QueryRow(ctx, `
		SELECT r.purpose_code, r.requested_amount::text, r.tenure_months::text, f.existing_financing_amount::text,
		       f.deviation_note, f.mitigation_note, f.officer_notes, cs.margin_type_code,
		       CASE cs.margin_type_code WHEN $2 THEN cs.margin_flat ELSE cs.margin_effective END::text
		FROM fos.work w
		LEFT JOIN fos.case_financing_request r ON r.case_id = w.case_id AND r.sort_order = 1
		LEFT JOIN fos.case_financing f ON f.case_id = w.case_id
		LEFT JOIN fos.case_credit_simulation cs ON cs.case_id = w.case_id
		WHERE w.case_id = $1`, caseID, MarginFlat).Scan(&purpose, &amount, &tenure, &existing, &deviation, &mitigation, &notes, &marginType, &margin)
	if err != nil {
		return FacilityRequestPage{}, err
	}
	page.Values = FacilityRequestForm{PurposeCode: value(purpose), RequestedAmount: value(amount), TenureMonths: value(tenure),
		ExistingFinancingAmount: value(existing), MarginTypeCode: value(marginType), MarginRate: value(margin), DeviationNote: value(deviation),
		MitigationNote: value(mitigation), Notes: value(notes)}
	page.PageOptions = s.pageOptions(ctx, map[string]string{"purpose": "DPFHPDE", "marginType": "MARGINTYPE"})
	return page, nil
}

// The four inputs of the simulation, in their screen order (FR-APP-018
// Field 1 and 3).
func (c *fieldCheck) requestedAmount(v string) {
	c.positive("requestedAmount", "Pembiayaan yang di ajukan", v, maxRequestedAmount, true)
}

func (c *fieldCheck) tenureMonths(v string) {
	c.positive("tenureMonths", "Jangka Waktu (Bulan)", v, maxTenureMonths, true)
}

func (c *fieldCheck) marginType(v string) {
	c.code("marginTypeCode", "Tipe Margin", "MARGINTYPE", v, true)
}

func (c *fieldCheck) marginRate(v string) {
	c.margin("marginRate", "Margin / Ujrah (%)", v, true)
}

// simulation is the simulation of checked inputs: noSimulation unless the
// four are filled and the type is one the formulas know (DEC-146).
func (in SimulationInput) simulation() FinancingSimulation {
	months, err := strconv.Atoi(in.TenureMonths)
	if in.RequestedAmount == "" || err != nil || in.MarginRate == "" || (in.MarginTypeCode != MarginFlat && in.MarginTypeCode != MarginEffective) {
		return noSimulation
	}
	return simulate(in.RequestedAmount, months, in.MarginTypeCode, in.MarginRate)
}

// Simulate is the simulation of the values on screen, nothing saved
// (FR-APP-018 Process 2 and 7, agreed comment 11).
func (s *Service) Simulate(ctx context.Context, sess auth.Session, caseID string, id int64, in SimulationInput) (FinancingSimulation, error) {
	if err := s.mayView(ctx, sess, caseID, id); err != nil {
		return FinancingSimulation{}, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return FinancingSimulation{}, err
	}
	defer tx.Rollback(ctx)
	t := strings.TrimSpace
	in = SimulationInput{t(in.RequestedAmount), t(in.TenureMonths), t(in.MarginTypeCode), t(in.MarginRate)}
	c := &fieldCheck{ctx: ctx, tx: tx, section: SectionForm}
	c.requestedAmount(in.RequestedAmount)
	c.tenureMonths(in.TenureMonths)
	c.marginType(in.MarginTypeCode)
	c.marginRate(in.MarginRate)
	if c.err != nil {
		return FinancingSimulation{}, c.err
	}
	if len(c.errs) > 0 {
		return noSimulation, nil
	}
	return in.simulation(), nil
}

// note is a note as typed, a CRLF counting as one character (DB-RULE-037).
func note(v string) string {
	return strings.TrimSpace(strings.ReplaceAll(v, "\r\n", "\n"))
}

// SubmitFacilityRequest validates and stores Data Permohonan Fasilitas
// (FR-APP-018 Process 8, agreed comments 6-8): Next also checks the product
// limit, records the recommendation and opens QCA Data; Back to Previous
// Step and the breadcrumb open their step; Save stays.
func (s *Service) SubmitFacilityRequest(ctx context.Context, sess auth.Session, caseID string, id int64, to string, f FacilityRequestForm) (int64, error) {
	next := to == FacilityRequestNext
	if next {
		to = AssignmentQCAData
	}
	return s.moveHotProspect(ctx, sess, caseID, id, AssignmentFacilityRequest, to, func(c *fieldCheck) error {
		t := strings.TrimSpace
		f = FacilityRequestForm{t(f.PurposeCode), t(f.RequestedAmount), t(f.TenureMonths), t(f.ExistingFinancingAmount), t(f.MarginTypeCode),
			t(f.MarginRate), note(f.DeviationNote), note(f.MitigationNote), note(f.Notes)}
		c.code("purposeCode", "Tujuan Pembiayaan", "DPFHPDE", f.PurposeCode, true)
		c.requestedAmount(f.RequestedAmount)
		c.tenureMonths(f.TenureMonths)
		c.money("existingFinancingAmount", "Jumlah Pembiayaan Eksisting di Bank Nano", f.ExistingFinancingAmount, true)
		c.marginType(f.MarginTypeCode)
		c.marginRate(f.MarginRate)
		c.text("deviationNote", "Catatan Deviasi", f.DeviationNote, false, maxFacilityNote)
		c.text("mitigationNote", "Catatan Mitigasi", f.MitigationNote, false, maxFacilityNote)
		c.text("notes", "Notes", f.Notes, true, maxFacilityNote)
		if c.err != nil || len(c.errs) > 0 {
			return c.err
		}
		if next {
			if c.productLimit(caseID, f.RequestedAmount); len(c.errs) > 0 {
				return nil
			}
		}
		if err := saveFacilityRequest(c, caseID, f); err != nil {
			return err
		}
		if !next {
			return nil
		}
		return s.recommendFacilityRequest(c, sess, caseID, f)
	})
}

// productLimit is FR-APP-018 rule 6 (DEC-145): the Total Requested Amount
// (the one line on screen) within the case product's limits, the limits
// themselves accepted; ERR-042 outside, ERR-043 when a limit cannot be read.
func (c *fieldCheck) productLimit(caseID, amount string) {
	var below, above *bool
	err := c.tx.QueryRow(c.ctx, `
		SELECT $2::numeric < p.financing_limit_start, $2::numeric > p.financing_limit_end
		FROM fos.work w JOIN fos.product_management p ON p.code = w.product_code
		WHERE w.case_id = $1`, caseID, amount).Scan(&below, &above)
	if err != nil && !isNotFound(err) {
		slog.Error("product limit unreadable (ERR-043)", "case", caseID, "error", err)
	}
	const label = "Pembiayaan yang di ajukan"
	switch {
	case err != nil || below == nil || above == nil:
		c.errs = append(c.errs, FieldError{c.section, "", "", "ERR-043", msgNoProductLimit})
	case *below:
		c.add("requestedAmount", label, "ERR-042", msgBelowProductLimit)
	case *above:
		c.add("requestedAmount", label, "ERR-042", msgAboveProductLimit)
	}
}

// saveFacilityRequest writes §9.41-9.43, each only with its inputs complete
// (agreed comment 6): the request line (DB-RULE-118), the totals computed
// here from it (DB-RULE-119) and the simulation of the screen (DB-RULE-120).
func saveFacilityRequest(c *fieldCheck, caseID string, f FacilityRequestForm) error {
	ctx, tx := c.ctx, c.tx
	if f.PurposeCode != "" && f.RequestedAmount != "" && f.TenureMonths != "" {
		if _, err := tx.Exec(ctx, `DELETE FROM fos.case_financing_request WHERE case_id = $1`, caseID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO fos.case_financing_request (case_id, sort_order, purpose_code, requested_amount, tenure_months)
			VALUES ($1, 1, $2, $3::numeric, $4::integer)`, caseID, f.PurposeCode, f.RequestedAmount, f.TenureMonths); err != nil {
			return err
		}
	}
	if f.ExistingFinancingAmount != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO fos.case_financing (case_id, total_requested_amount, existing_financing_amount, total_obligor, deviation_note,
				mitigation_note, officer_notes)
			VALUES ($1, 0, $2::numeric, 0, $3, $4, $5)
			ON CONFLICT (case_id) DO UPDATE SET existing_financing_amount = EXCLUDED.existing_financing_amount,
				deviation_note = EXCLUDED.deviation_note, mitigation_note = EXCLUDED.mitigation_note, officer_notes = EXCLUDED.officer_notes`,
			caseID, f.ExistingFinancingAmount, orNil(f.DeviationNote), orNil(f.MitigationNote), orNil(f.Notes)); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE fos.case_financing f SET total_requested_amount = r.total, total_obligor = f.existing_financing_amount + r.total
		FROM (SELECT COALESCE(sum(requested_amount), 0) AS total FROM fos.case_financing_request WHERE case_id = $1) r
		WHERE f.case_id = $1`, caseID); err != nil {
		return err
	}
	sim := SimulationInput{f.RequestedAmount, f.TenureMonths, f.MarginTypeCode, f.MarginRate}.simulation()
	if !sim.Ready || f.ExistingFinancingAmount == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO fos.case_credit_simulation (case_id, product_code, total_obligor, amount, tenure_months, margin_type_code, margin_flat,
			margin_effective, installment_flat, installment_effective)
		SELECT case_id, product_code, $2::numeric + $3::numeric, $3::numeric, $4::integer, $5, $6::numeric, $7::numeric, $8::numeric, $9::numeric
		FROM fos.work WHERE case_id = $1
		ON CONFLICT (case_id) DO UPDATE SET product_code = EXCLUDED.product_code, total_obligor = EXCLUDED.total_obligor,
			amount = EXCLUDED.amount, tenure_months = EXCLUDED.tenure_months, margin_type_code = EXCLUDED.margin_type_code,
			margin_flat = EXCLUDED.margin_flat, margin_effective = EXCLUDED.margin_effective, installment_flat = EXCLUDED.installment_flat,
			installment_effective = EXCLUDED.installment_effective`,
		caseID, f.ExistingFinancingAmount, f.RequestedAmount, f.TenureMonths, f.MarginTypeCode, sim.MarginFlat, sim.MarginEffective,
		sim.InstallmentFlat, sim.InstallmentEffective)
	return err
}

// recommendFacilityRequest adds the AO's "Data Permohonan Fasilitas"
// recommendation (FR-APP-018 Rekomendasi, DB-RULE-122): RECOMMEND,
// SESUAILMT, the diajukan group from the saved request and simulation, the
// direkomendasi and disetujui groups 0, and its §9.44 simulation
// (DB-RULE-121) with the applicant's name.
func (s *Service) recommendFacilityRequest(c *fieldCheck, sess auth.Session, caseID string, f FacilityRequestForm) error {
	debtor, err := applicantName(c.ctx, c.tx, caseID)
	if err != nil {
		return err
	}
	var decisionID int64
	// DB-RULE-117: the AO's role as it is now, kept as a snapshot.
	err = c.tx.QueryRow(c.ctx, `
		INSERT INTO fos.case_pre_screening_decision (case_id, recommendation_type, notes, decided_by, decided_by_role_code, decided_at,
			recommendation_status, limit_action_code, total_obligor, requested_amount, recommended_amount, approved_amount,
			requested_margin_flat, requested_margin_effective, recommended_margin_flat, recommended_margin_effective,
			approved_margin_flat, approved_margin_effective, requested_tenure, recommended_tenure, approved_tenure)
		SELECT $1::varchar, $2, $3, o.user_id, o.role_code, $5, $6, $7, f.total_obligor, f.total_requested_amount, 0, 0,
			cs.margin_flat, cs.margin_effective, 0, 0, 0, 0, cs.tenure_months, 0, 0
		FROM fos.operator o, fos.case_financing f, fos.case_credit_simulation cs
		WHERE o.user_id = $4 AND f.case_id = $1::varchar AND cs.case_id = $1::varchar
		RETURNING decision_id`,
		caseID, recommendationFacilityRequest, f.Notes, sess.UserID, s.now(), recommendationRecommend, limitActionAsIs).Scan(&decisionID)
	if err != nil {
		return err
	}
	_, err = c.tx.Exec(c.ctx, `
		INSERT INTO fos.recommendation_credit_simulation (decision_id, purpose_code, debtor_name, product_code, total_obligor, amount,
			tenure_months, margin_type_code, margin_flat, margin_effective, installment_flat, installment_effective)
		SELECT $1, $3, $4, product_code, total_obligor, amount, tenure_months, margin_type_code, margin_flat, margin_effective,
			installment_flat, installment_effective
		FROM fos.case_credit_simulation WHERE case_id = $2`, decisionID, caseID, f.PurposeCode, debtor)
	return err
}
