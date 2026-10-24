package app

import (
	"encoding/json"
	"net/http"
	"time"

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
	Name        string    `json:"name"`
	At          time.Time `json:"at"`
	SubmittedAt string    `json:"submittedAt"`
	Status      string    `json:"status"`
	Request     string    `json:"request"`
	Response    string    `json:"response"`
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
	TruncateChars   int                  `json:"truncateChars"`
	TruncateLines   int                  `json:"truncateLines"`
}

func toReviewJSON(p cases.ReviewPage) reviewJSON {
	out := reviewJSON{DecisionOptions: toOptionsJSON(p.DecisionOptions), History: []recommendationJSON{}, Editable: p.Editable}
	for _, r := range p.History {
		out.History = append(out.History, recommendationJSON(r))
	}
	out.Retry, out.SLIKTimeout = toRetryJSON(p.Retry), toRetryJSON(p.SLIKTimeout)
	out.TruncateChars, out.TruncateLines = p.TruncateChars, p.TruncateLines
	return out
}

func toRetryJSON(v *cases.APILogView) *retryJSON {
	if v == nil {
		return nil
	}
	return &retryJSON{Logs: toLogsJSON(v.Logs)}
}

func toLogsJSON(logs []cases.APILogEntry) []apiLogJSON {
	out := []apiLogJSON{}
	for _, l := range logs {
		out = append(out, apiLogJSON{l.Name, l.At, l.SubmittedAt, l.Status, l.Request, l.Response})
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

type apiDataFieldJSON struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Indent bool   `json:"indent,omitempty"`
}

type apiDataProcessJSON struct {
	Name           string               `json:"name"`
	Rejected       bool                 `json:"rejected"`
	RejectedOnly   bool                 `json:"rejectedOnly"`
	Request        []apiDataFieldJSON   `json:"request"`
	Response       []apiDataFieldJSON   `json:"response"`
	Errors         []string             `json:"errors"`
	Results        [][]apiDataFieldJSON `json:"results"`
	DocumentID     string               `json:"documentId,omitempty"`
	DocumentName   string               `json:"documentName,omitempty"`
	DocumentAction string               `json:"documentAction,omitempty"`
}

func toFieldsJSON(fs []cases.APIDataField) []apiDataFieldJSON {
	out := []apiDataFieldJSON{}
	for _, f := range fs {
		out = append(out, apiDataFieldJSON(f))
	}
	return out
}

// history handles GET .../assignments/{id}/history: the History Details
// card under the assignment (UI-013): API Log History (UI-020) and
// Pre-Screening API Data (UI-021).
func (s *server) history(ep endpoint) http.HandlerFunc {
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
		h, err := s.cases.History(r.Context(), sess, caseID, id)
		if writeAssignmentError(w, "history details", err) {
			return
		}
		processes := []apiDataProcessJSON{}
		for _, p := range h.Processes {
			results := [][]apiDataFieldJSON{}
			for _, r := range p.Results {
				results = append(results, toFieldsJSON(r))
			}
			errs := p.Errors
			if errs == nil {
				errs = []string{}
			}
			var docID, docName string
			if p.Document != nil {
				docID, docName = p.Document.ID, p.Document.Name
			}
			processes = append(processes, apiDataProcessJSON{p.Name, p.Rejected, p.RejectedOnly, toFieldsJSON(p.Request),
				toFieldsJSON(p.Response), errs, results, docID, docName, p.DocumentAction})
		}
		writeJSON(w, http.StatusOK, struct {
			Logs          []apiLogJSON         `json:"logs"`
			TruncateChars int                  `json:"truncateChars"`
			TruncateLines int                  `json:"truncateLines"`
			Processes     []apiDataProcessJSON `json:"processes"`
		}{toLogsJSON(h.Logs), h.TruncateChars, h.TruncateLines, processes})
	}
}
