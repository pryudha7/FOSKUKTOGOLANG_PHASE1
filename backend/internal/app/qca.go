package app

import (
	"context"
	"encoding/json"
	"net/http"

	"fos/internal/auth"
	"fos/internal/cases"
)

// qcaJSON is QCA Data (FR-APP-020): the answer code of questions 1 to 10 in
// order, "" when not answered.
type qcaJSON struct {
	Answers []string `json:"answers"`
}

type qcaPageJSON struct {
	Values qcaJSON `json:"values"`
	pageOptionsJSON
	FacilityRequested bool `json:"facilityRequested"`
	Editable          bool `json:"editable"`
}

func toQCAPageJSON(p cases.QCAPage) qcaPageJSON {
	return qcaPageJSON{Values: qcaJSON{p.Answers}, pageOptionsJSON: toPageOptionsJSON(p.PageOptions), FacilityRequested: p.FacilityRequested,
		Editable: p.Editable}
}

func (s *server) submitQCA(ep endpoint) http.HandlerFunc {
	return s.submitProspect(ep, func(ctx context.Context, dec *json.Decoder, sess auth.Session, caseID string, id int64, to string) (int64, error) {
		var f qcaJSON
		if err := dec.Decode(&f); err != nil {
			return 0, badBody{err}
		}
		return s.cases.SubmitQCA(ctx, sess, caseID, id, to, f.Answers)
	})
}
