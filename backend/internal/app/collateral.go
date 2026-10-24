package app

import (
	"context"
	"encoding/json"
	"net/http"

	"fos/internal/auth"
	"fos/internal/cases"
)

// collateralJSON is one Collateral Data card (FR-APP-021): codes, texts,
// amounts and years as digits, dates as yyyy-mm-dd.
type collateralJSON struct {
	TypeCode             string `json:"typeCode"`
	BuildingFormCode     string `json:"buildingFormCode"`
	OwnershipTypeCode    string `json:"ownershipTypeCode"`
	OwnerName            string `json:"ownerName"`
	SHMNumber            string `json:"shmNumber"`
	SHGBNumber           string `json:"shgbNumber"`
	SHGBValidUntil       string `json:"shgbValidUntil"`
	BPKBNumber           string `json:"bpkbNumber"`
	FiduciaryNumber      string `json:"fiduciaryNumber"`
	Location             string `json:"location"`
	LandArea             string `json:"landArea"`
	DueDate              string `json:"dueDate"`
	PhysicalValue        string `json:"physicalValue"`
	ObjectValue          string `json:"objectValue"`
	DepositAmount        string `json:"depositAmount"`
	VehicleBrand         string `json:"vehicleBrand"`
	VehicleYear          string `json:"vehicleYear"`
	Remarks              string `json:"remarks"`
	CollateralStatusCode string `json:"collateralStatusCode"`
	CollateralTypeText   string `json:"collateralTypeText"`
	BindingType          string `json:"bindingType"`
	BindingDate          string `json:"bindingDate"`
	AppraisalDate        string `json:"appraisalDate"`
	KJPPAppraisalDate    string `json:"kjppAppraisalDate"`
	NJOPValue            string `json:"njopValue"`
	KJPPValue            string `json:"kjppValue"`
	KJPPAppraiserName    string `json:"kjppAppraiserName"`
	PariPassuStatusCode  string `json:"pariPassuStatusCode"`
	PariPassuPercent     string `json:"pariPassuPercent"`
	JointFinancingCode   string `json:"jointFinancingCode"`
	InsuredCode          string `json:"insuredCode"`
}

type collateralFormJSON struct {
	Main    []collateralJSON `json:"main"`
	Support []collateralJSON `json:"support"`
}

func toCollateralJSONs(cards []cases.Collateral) []collateralJSON {
	out := make([]collateralJSON, 0, len(cards))
	for _, c := range cards {
		out = append(out, collateralJSON(c))
	}
	return out
}

func fromCollateralJSONs(cards []collateralJSON) []cases.Collateral {
	out := make([]cases.Collateral, 0, len(cards))
	for _, c := range cards {
		out = append(out, cases.Collateral(c))
	}
	return out
}

type collateralPageJSON struct {
	Values collateralFormJSON `json:"values"`
	pageOptionsJSON
	MaxCards          int64  `json:"maxCards"`
	Today             string `json:"today"`
	FacilityRequested bool   `json:"facilityRequested"`
	Editable          bool   `json:"editable"`
}

func toCollateralPageJSON(p cases.CollateralPage) collateralPageJSON {
	return collateralPageJSON{
		Values:          collateralFormJSON{Main: toCollateralJSONs(p.Values.Main), Support: toCollateralJSONs(p.Values.Support)},
		pageOptionsJSON: toPageOptionsJSON(p.PageOptions), MaxCards: p.MaxCards, Today: p.Today, FacilityRequested: p.FacilityRequested,
		Editable: p.Editable,
	}
}

func (s *server) submitCollateral(ep endpoint) http.HandlerFunc {
	return s.submitProspect(ep, func(ctx context.Context, dec *json.Decoder, sess auth.Session, caseID string, id int64, to string) (int64, error) {
		var f collateralFormJSON
		if err := dec.Decode(&f); err != nil {
			return 0, badBody{err}
		}
		return s.cases.SubmitCollateral(ctx, sess, caseID, id, to, cases.CollateralForm{Main: fromCollateralJSONs(f.Main), Support: fromCollateralJSONs(f.Support)})
	})
}
