package app

import (
	"context"
	"encoding/json"
	"net/http"

	"fos/internal/auth"
	"fos/internal/cases"
)

// businessDataJSON is Data Usaha Nasabah (FR-APP-016); numbers are digits.
type businessDataJSON struct {
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
	regionJSON
}

type businessDataPageJSON struct {
	Values businessDataJSON `json:"values"`
	pageOptionsJSON
	FacilityRequested bool `json:"facilityRequested"`
	Editable          bool `json:"editable"`
}

func toBusinessDataPageJSON(p cases.BusinessDataPage) businessDataPageJSON {
	v := p.Values
	return businessDataPageJSON{
		Values: businessDataJSON{BusinessField: v.BusinessField, BusinessCategoryCode: v.BusinessCategoryCode, BusinessAgeMonths: v.BusinessAgeMonths,
			EmployeeCount: v.EmployeeCount, BusinessLocationCode: v.BusinessLocationCode, Address: v.Address, HouseOwnershipCode: v.HouseOwnershipCode,
			YearsOfStay: v.YearsOfStay, MonthsOfStay: v.MonthsOfStay, Phone: v.Phone, RT: v.RT, RW: v.RW, regionJSON: regionJSON(v.Region)},
		pageOptionsJSON: toPageOptionsJSON(p.PageOptions), FacilityRequested: p.FacilityRequested, Editable: p.Editable,
	}
}

// otherFinancingJSON is one Data Pembiayaan Lain card (FR-APP-017): amounts
// and counts as digits, marginRate with a dot.
type otherFinancingJSON struct {
	BankName          string `json:"bankName"`
	FinancingType     string `json:"financingType"`
	FinancingAmount   string `json:"financingAmount"`
	OutstandingAmount string `json:"outstandingAmount"`
	TenorMonths       string `json:"tenorMonths"`
	MarginRate        string `json:"marginRate"`
	SLIKCollect       string `json:"slikCollect"`
	Purpose           string `json:"purpose"`
}

type otherFinancingPageJSON struct {
	Values struct {
		OtherFinancings []otherFinancingJSON `json:"otherFinancings"`
	} `json:"values"`
	SLIKAvailable     bool `json:"slikAvailable"`
	FacilityRequested bool `json:"facilityRequested"`
	Editable          bool `json:"editable"`
}

func toOtherFinancingPageJSON(p cases.OtherFinancingPage) otherFinancingPageJSON {
	out := otherFinancingPageJSON{SLIKAvailable: p.SLIKAvailable, FacilityRequested: p.FacilityRequested, Editable: p.Editable}
	out.Values.OtherFinancings = make([]otherFinancingJSON, 0, len(p.Cards))
	for _, c := range p.Cards {
		out.Values.OtherFinancings = append(out.Values.OtherFinancings, otherFinancingJSON(c))
	}
	return out
}

func (s *server) submitBusinessData(ep endpoint) http.HandlerFunc {
	return s.submitProspect(ep, func(ctx context.Context, dec *json.Decoder, sess auth.Session, caseID string, id int64, to string) (int64, error) {
		var f businessDataJSON
		if err := dec.Decode(&f); err != nil {
			return 0, badBody{err}
		}
		return s.cases.SubmitBusinessData(ctx, sess, caseID, id, to, cases.BusinessDataForm{
			BusinessField: f.BusinessField, BusinessCategoryCode: f.BusinessCategoryCode, BusinessAgeMonths: f.BusinessAgeMonths,
			EmployeeCount: f.EmployeeCount, BusinessLocationCode: f.BusinessLocationCode, Address: f.Address, HouseOwnershipCode: f.HouseOwnershipCode,
			YearsOfStay: f.YearsOfStay, MonthsOfStay: f.MonthsOfStay, Phone: f.Phone, RT: f.RT, RW: f.RW, Region: cases.Region(f.regionJSON),
		})
	})
}

func (s *server) submitOtherFinancing(ep endpoint) http.HandlerFunc {
	return s.submitProspect(ep, func(ctx context.Context, dec *json.Decoder, sess auth.Session, caseID string, id int64, to string) (int64, error) {
		var f struct {
			OtherFinancings []otherFinancingJSON `json:"otherFinancings"`
		}
		if err := dec.Decode(&f); err != nil {
			return 0, badBody{err}
		}
		cards := make([]cases.OtherFinancing, 0, len(f.OtherFinancings))
		for _, c := range f.OtherFinancings {
			cards = append(cards, cases.OtherFinancing(c))
		}
		return s.cases.SubmitOtherFinancing(ctx, sess, caseID, id, to, cards)
	})
}

// slikFacilities handles GET .../assignments/{id}/slik-facilities: the
// customer's SLIK facilities for Isi Data dari SLIK (FR-APP-017).
func (s *server) slikFacilities(ep endpoint) http.HandlerFunc {
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
		groups, err := s.cases.SLIKFacilities(r.Context(), sess, caseID, id)
		if writeAssignmentError(w, "slik facilities", err) {
			return
		}
		type facilityJSON struct {
			FinancingAmount   string             `json:"financingAmount"`
			OutstandingAmount string             `json:"outstandingAmount"`
			MarginRate        string             `json:"marginRate"`
			Card              otherFinancingJSON `json:"card"`
		}
		type groupJSON struct {
			Title      string         `json:"title"`
			Facilities []facilityJSON `json:"facilities"`
		}
		out := make([]groupJSON, 0, len(groups))
		for _, g := range groups {
			fs := make([]facilityJSON, 0, len(g.Facilities))
			for _, f := range g.Facilities {
				fs = append(fs, facilityJSON{f.FinancingAmount, f.OutstandingAmount, f.MarginRate, otherFinancingJSON(f.Card)})
			}
			out = append(out, groupJSON{g.Title, fs})
		}
		writeJSON(w, http.StatusOK, struct {
			Groups []groupJSON `json:"groups"`
		}{out})
	}
}
