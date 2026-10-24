package app

import (
	"context"
	"encoding/json"
	"net/http"

	"fos/internal/auth"
	"fos/internal/cases"
)

// facilityRequestJSON is Data Permohonan Fasilitas (FR-APP-018): amounts
// and months as digits, marginRate with a dot. No total is accepted: the
// server computes them (DB-RULE-119).
type facilityRequestJSON struct {
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

func (f facilityRequestJSON) toForm() cases.FacilityRequestForm {
	return cases.FacilityRequestForm{PurposeCode: f.PurposeCode,
		SimulationInput: cases.SimulationInput{RequestedAmount: f.RequestedAmount, TenureMonths: f.TenureMonths, MarginTypeCode: f.MarginTypeCode,
			MarginRate: f.MarginRate},
		ExistingFinancingAmount: f.ExistingFinancingAmount, DeviationNote: f.DeviationNote, MitigationNote: f.MitigationNote, Notes: f.Notes}
}

type facilityRequestPageJSON struct {
	Values            facilityRequestJSON `json:"values"`
	ContractType      string              `json:"contractType"`
	ProductName       string              `json:"productName"`
	ApplicantName     string              `json:"applicantName"`
	FacilityRequested bool                `json:"facilityRequested"`
	pageOptionsJSON
	Editable bool `json:"editable"`
}

func toFacilityRequestPageJSON(p cases.FacilityRequestPage) facilityRequestPageJSON {
	v := p.Values
	return facilityRequestPageJSON{
		Values: facilityRequestJSON{PurposeCode: v.PurposeCode, RequestedAmount: v.RequestedAmount, TenureMonths: v.TenureMonths,
			ExistingFinancingAmount: v.ExistingFinancingAmount, MarginTypeCode: v.MarginTypeCode, MarginRate: v.MarginRate,
			DeviationNote: v.DeviationNote, MitigationNote: v.MitigationNote, Notes: v.Notes},
		ContractType: p.ContractType, ProductName: p.ProductName, ApplicantName: p.ApplicantName, FacilityRequested: p.FacilityRequested,
		pageOptionsJSON: toPageOptionsJSON(p.PageOptions), Editable: p.Editable,
	}
}

func (s *server) submitFacilityRequest(ep endpoint) http.HandlerFunc {
	return s.submitProspect(ep, func(ctx context.Context, dec *json.Decoder, sess auth.Session, caseID string, id int64, to string) (int64, error) {
		var f facilityRequestJSON
		if err := dec.Decode(&f); err != nil {
			return 0, badBody{err}
		}
		return s.cases.SubmitFacilityRequest(ctx, sess, caseID, id, to, f.toForm())
	})
}

// facilitySimulation handles POST .../assignments/{id}/facility-simulation:
// the Financing Simulation of the values on screen, nothing saved (FR-APP-018
// Process 2 and 7).
func (s *server) facilitySimulation(ep endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.session(w, r, ep)
		if !ok {
			return
		}
		caseID, id, ok := pathAssignment(r)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var in struct {
			RequestedAmount string `json:"requestedAmount"`
			TenureMonths    string `json:"tenureMonths"`
			MarginTypeCode  string `json:"marginTypeCode"`
			MarginRate      string `json:"marginRate"`
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sim, err := s.cases.Simulate(r.Context(), sess, caseID, id, cases.SimulationInput(in))
		if writeAssignmentError(w, "facility simulation", err) {
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Ready                bool   `json:"ready"`
			InstallmentFlat      string `json:"installmentFlat"`
			InstallmentEffective string `json:"installmentEffective"`
			MarginFlat           string `json:"marginFlat"`
			MarginEffective      string `json:"marginEffective"`
		}{sim.Ready, sim.InstallmentFlat, sim.InstallmentEffective, sim.MarginFlat, sim.MarginEffective})
	}
}
