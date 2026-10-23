package app

import (
	"encoding/json"
	"net/http"

	"fos/internal/cases"
)

// notesJSON is a Pre Screening decision as UI-010's Notes banner shows it.
type notesJSON struct {
	Decision string `json:"decision"`
	Notes    string `json:"notes"`
}

type recommendationJSON struct {
	DecidedBy   string `json:"decidedBy"`
	SubmittedAt string `json:"submittedAt"`
	Decision    string `json:"decision"`
	Notes       string `json:"notes"`
}

type reviewJSON struct {
	DecisionOptions []optionJSON         `json:"decisionOptions"`
	History         []recommendationJSON `json:"history"`
	Editable        bool                 `json:"editable"`
}

func toReviewJSON(p cases.ReviewPage) reviewJSON {
	out := reviewJSON{DecisionOptions: toOptionsJSON(p.DecisionOptions), History: []recommendationJSON{}, Editable: p.Editable}
	for _, r := range p.History {
		out.History = append(out.History, recommendationJSON(r))
	}
	return out
}

// submitReview handles POST .../assignments/{id}/review: Submit on the Team
// Lead's Pre Screening (FR-APP-010). Only Keputusan and Notes are accepted.
func (s *server) submitReview(ep endpoint) http.HandlerFunc {
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
		var req struct {
			DecisionCode string `json:"decisionCode"`
			Notes        string `json:"notes"`
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if writeAssignmentError(w, "pre-screening review", s.cases.SubmitReview(r.Context(), sess, caseID, id, req.DecisionCode, req.Notes)) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
