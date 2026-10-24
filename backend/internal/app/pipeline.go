package app

import (
	"context"
	"encoding/json"
	"net/http"

	"fos/internal/auth"
	"fos/internal/cases"
)

// documentItemJSON is a file item of UI-024: name is shown as the link,
// caption under it; fileName names the download.
type documentItemJSON struct {
	FileID   string `json:"fileId"`
	FileName string `json:"fileName"`
	Name     string `json:"name"`
	Caption  string `json:"caption"`
}

func toDocumentItemJSON(d *cases.DocumentItem) *documentItemJSON {
	if d == nil {
		return nil
	}
	return &documentItemJSON{d.FileID, d.FileName, d.Name, d.Caption}
}

type pipelineJSON struct {
	BranchCode            string       `json:"branchCode"`
	BranchName            string       `json:"branchName"`
	CustomerType          string       `json:"customerType"`
	SubmissionType        string       `json:"submissionType"`
	ProductCode           string       `json:"productCode"`
	BankName              string       `json:"bankName"`
	SubmissionTypeOptions []optionJSON `json:"submissionTypeOptions"`
	ProductOptions        []optionJSON `json:"productOptions"`
	Identity              struct {
		Label string            `json:"label"`
		File  *documentItemJSON `json:"file"`
	} `json:"identity"`
	BankLetter *documentItemJSON `json:"bankLetter"`
	Editable   bool              `json:"editable"`
}

func toPipelineJSON(p cases.PipelinePage) pipelineJSON {
	out := pipelineJSON{
		BranchCode: p.BranchCode, BranchName: p.BranchName, CustomerType: p.CustomerType,
		SubmissionType: p.SubmissionType, ProductCode: p.ProductCode, BankName: p.BankName,
		SubmissionTypeOptions: toOptionsJSON(p.SubmissionTypeOptions), ProductOptions: toOptionsJSON(p.ProductOptions),
		BankLetter: toDocumentItemJSON(p.BankLetter), Editable: p.Editable,
	}
	out.Identity.Label, out.Identity.File = p.IdentityLabel, toDocumentItemJSON(p.Identity)
	return out
}

// submitPipeline handles POST .../assignments/{id}/pipeline: Submit on
// Pipeline Data Entry (FR-APP-004). Only this form's fields are accepted
// (BR-UI-010).
func (s *server) submitPipeline(ep endpoint) http.HandlerFunc {
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
			SubmissionType string `json:"submissionType"`
			ProductCode    string `json:"productCode"`
			BankName       string `json:"bankName"`
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		next, err := s.cases.SubmitPipeline(r.Context(), sess, caseID, id, cases.PipelineForm(req))
		writeMoveResult(w, next, err)
	}
}

// bankSuggestions handles GET .../assignments/{id}/banks?q=: the Nama Bank
// autocomplete (BR-UI-005).
func (s *server) bankSuggestions(ep endpoint) http.HandlerFunc {
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
		opts, err := s.cases.BankSuggestions(r.Context(), sess, caseID, id, r.URL.Query().Get("q"))
		if writeAssignmentError(w, "bank suggestions", err) {
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Options []optionJSON `json:"options"`
		}{toOptionsJSON(opts)})
	}
}

// changeSubmissionType handles POST .../assignments/{id}/submission-type-change:
// Jenis Pengajuan changed on the page (FR-APP-004 rule 3).
func (s *server) changeSubmissionType(ep endpoint) http.HandlerFunc {
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
		if writeAssignmentError(w, "change submission type", s.cases.ChangeSubmissionType(r.Context(), sess, caseID, id)) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// uploadBankLetter handles POST .../assignments/{id}/bank-letter: the file
// part "file" chosen for Bukti Surat dari Bank (FR-APP-004 rules 6-8),
// answered with the item the page shows.
func (s *server) uploadBankLetter(ep endpoint) http.HandlerFunc {
	return s.uploadFile(ep, "upload bank letter", func(ctx context.Context, sess auth.Session, caseID string, id int64, req cases.UploadRequest) (any, error) {
		d, err := s.cases.UploadBankLetter(ctx, sess, caseID, id, req)
		return toDocumentItemJSON(&d), err
	})
}
