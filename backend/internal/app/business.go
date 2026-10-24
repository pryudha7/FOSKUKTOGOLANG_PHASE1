package app

import (
	"encoding/json"
	"net/http"

	"fos/internal/cases"
)

type officerJSON struct {
	RoleCode  string `json:"roleCode"`
	NIK       string `json:"nik"`
	Name      string `json:"name"`
	BirthDate string `json:"birthDate"`
}

// businessEntityJSON is the Badan Usaha form (FR-APP-005); goPublic is the
// BMV description.
type businessEntityJSON struct {
	BusinessEntityCode       string `json:"businessEntityCode"`
	PlaceOfEstablishment     string `json:"placeOfEstablishment"`
	DeedNumber               string `json:"deedNumber"`
	LastDeedNumber           string `json:"lastDeedNumber"`
	LastDeedDate             string `json:"lastDeedDate"`
	Phone                    string `json:"phone"`
	MobilePhone              string `json:"mobilePhone"`
	Email                    string `json:"email"`
	EconomicSectorCode       string `json:"economicSectorCode"`
	RelationshipWithBankCode string `json:"relationshipWithBankCode"`
	BusinessTypeCode         string `json:"businessTypeCode"`
	CustomerCategoryCode     string `json:"customerCategoryCode"`
	Address                  string `json:"address"`
	GoPublic                 string `json:"goPublic"`
	regionJSON
	BusinessGroupName string        `json:"businessGroupName"`
	Officers          []officerJSON `json:"officers"`
}

func (f businessEntityJSON) toForm() cases.BusinessEntityForm {
	officers := make([]cases.Officer, 0, len(f.Officers))
	for _, o := range f.Officers {
		officers = append(officers, cases.Officer(o))
	}
	return cases.BusinessEntityForm{
		BusinessEntityCode: f.BusinessEntityCode, PlaceOfEstablishment: f.PlaceOfEstablishment, DeedNumber: f.DeedNumber,
		LastDeedNumber: f.LastDeedNumber, LastDeedDate: f.LastDeedDate, Phone: f.Phone, MobilePhone: f.MobilePhone, Email: f.Email,
		EconomicSectorCode: f.EconomicSectorCode, RelationshipWithBankCode: f.RelationshipWithBankCode, BusinessTypeCode: f.BusinessTypeCode,
		CustomerCategoryCode: f.CustomerCategoryCode, Address: f.Address, GoPublic: f.GoPublic, Region: cases.Region(f.regionJSON),
		BusinessGroupName: f.BusinessGroupName, Officers: officers,
	}
}

func toBusinessEntityJSON(f cases.BusinessEntityForm) businessEntityJSON {
	officers := make([]officerJSON, 0, len(f.Officers))
	for _, o := range f.Officers {
		officers = append(officers, officerJSON(o))
	}
	return businessEntityJSON{
		BusinessEntityCode: f.BusinessEntityCode, PlaceOfEstablishment: f.PlaceOfEstablishment, DeedNumber: f.DeedNumber,
		LastDeedNumber: f.LastDeedNumber, LastDeedDate: f.LastDeedDate, Phone: f.Phone, MobilePhone: f.MobilePhone, Email: f.Email,
		EconomicSectorCode: f.EconomicSectorCode, RelationshipWithBankCode: f.RelationshipWithBankCode, BusinessTypeCode: f.BusinessTypeCode,
		CustomerCategoryCode: f.CustomerCategoryCode, Address: f.Address, GoPublic: f.GoPublic, regionJSON: regionJSON(f.Region),
		BusinessGroupName: f.BusinessGroupName, Officers: officers,
	}
}

type businessEntityPageJSON struct {
	NPWP              string                  `json:"npwp"`
	Name              string                  `json:"name"`
	EstablishmentDate string                  `json:"establishmentDate"`
	ExistingCustomer  bool                    `json:"existingCustomer"`
	Values            businessEntityJSON      `json:"values"`
	Sector            *optionJSON             `json:"sector"`
	Category          *optionJSON             `json:"category"`
	Options           map[string][]optionJSON `json:"options"`
	FailedOptions     []string                `json:"failedOptions"`
	Editable          bool                    `json:"editable"`
	Today             string                  `json:"today"`
	EarliestBirthDate string                  `json:"earliestBirthDate"`
	LatestDeedDate    string                  `json:"latestDeedDate"`
}

func toOptionJSON(o *cases.Option) *optionJSON {
	if o == nil {
		return nil
	}
	return &optionJSON{o.Code, o.Description}
}

func toBusinessEntityPageJSON(p cases.BusinessEntityPage) businessEntityPageJSON {
	return businessEntityPageJSON{
		NPWP: p.NPWP, Name: p.Name, EstablishmentDate: p.EstablishmentDate, ExistingCustomer: p.ExistingCustomer,
		Values: toBusinessEntityJSON(p.Values), Sector: toOptionJSON(p.Sector), Category: toOptionJSON(p.Category),
		Options: toOptionMapJSON(p.Options), FailedOptions: p.FailedOptions, Editable: p.Editable,
		Today: p.Today, EarliestBirthDate: p.EarliestBirthDate, LatestDeedDate: p.LatestDeedDate,
	}
}

// submitBusinessEntity handles POST .../assignments/{id}/business-entity:
// Next on Badan Usaha (FR-APP-005 Process 4). Only this form's fields are
// accepted (BR-UI-010).
func (s *server) submitBusinessEntity(ep endpoint) http.HandlerFunc {
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
		var f businessEntityJSON
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&f); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeMoveResult(w, 0, s.cases.SubmitBusinessEntity(r.Context(), sess, caseID, id, f.toForm()))
	}
}
