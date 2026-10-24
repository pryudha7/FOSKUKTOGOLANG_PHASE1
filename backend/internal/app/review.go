package app

import (
	"encoding/json"
	"net/http"

	"fos/internal/cases"
)

// notesJSON is a Pre-Screening Review decision as UI-010's Notes banner shows it.
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

type apiLogJSON struct {
	Name        string `json:"name"`
	SubmittedAt string `json:"submittedAt"`
	Status      string `json:"status"`
	Request     string `json:"request"`
	Response    string `json:"response"`
}

type retryJSON struct {
	Logs []apiLogJSON `json:"logs"`
}

type reviewJSON struct {
	DecisionOptions []optionJSON         `json:"decisionOptions"`
	History         []recommendationJSON `json:"history"`
	Editable        bool                 `json:"editable"`
	Retry           *retryJSON           `json:"retry"`
	SLIKTimeout     *retryJSON           `json:"slikTimeout"`
}

func toReviewJSON(p cases.ReviewPage) reviewJSON {
	out := reviewJSON{DecisionOptions: toOptionsJSON(p.DecisionOptions), History: []recommendationJSON{}, Editable: p.Editable}
	for _, r := range p.History {
		out.History = append(out.History, recommendationJSON(r))
	}
	out.Retry, out.SLIKTimeout = toRetryJSON(p.Retry), toRetryJSON(p.SLIKTimeout)
	return out
}

func toRetryJSON(v *cases.APILogView) *retryJSON {
	if v == nil {
		return nil
	}
	out := &retryJSON{Logs: []apiLogJSON{}}
	for _, l := range v.Logs {
		out.Logs = append(out.Logs, apiLogJSON(l))
	}
	return out
}

// retryReview handles POST .../assignments/{id}/retry: Submit on the Retry
// view of Pre Screening (FR-PSAPI-001 rule 5). It takes no body.
func (s *server) retryReview(ep endpoint) http.HandlerFunc {
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
		if writeAssignmentError(w, "pre-screening retry", s.cases.RetryReview(r.Context(), sess, caseID, id)) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// submitReview handles POST .../assignments/{id}/review: Submit on the Team
// Lead's Pre-Screening Review (FR-APP-010). Only Keputusan and Notes are accepted.
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

type waitingSLIKJSON struct {
	Editable   bool `json:"editable"`
	DueMinutes int  `json:"dueMinutes"`
}

func toWaitingSLIKJSON(p cases.WaitingSLIKPage) waitingSLIKJSON {
	return waitingSLIKJSON{p.Editable, p.DueMinutes}
}

// checkSLIKStatus handles POST .../assignments/{id}/slik-status: Check SLIK
// Status on the waiting assignment (FR-PSAPI-013, UI-023). It takes no body
// and answers each subject's message, Customer first.
func (s *server) checkSLIKStatus(ep endpoint) http.HandlerFunc {
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
		statuses, err := s.cases.CheckSLIKStatus(r.Context(), sess, caseID, id)
		if writeAssignmentError(w, "check SLIK status", err) {
			return
		}
		type statusJSON struct {
			Subject string `json:"subject"`
			Message string `json:"message"`
		}
		out := []statusJSON{}
		for _, st := range statuses {
			out = append(out, statusJSON(st))
		}
		writeJSON(w, http.StatusOK, struct {
			Statuses []statusJSON `json:"statuses"`
		}{out})
	}
}
