package app

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"fos/internal/cases"
)

type storedFileJSON struct {
	FileID   string `json:"fileId"`
	FileName string `json:"fileName"`
}

type documentJSON struct {
	DocumentType string          `json:"documentType"`
	Mandatory    bool            `json:"mandatory"`
	File         *storedFileJSON `json:"file"`
}

type categoryJSON struct {
	Code      string         `json:"code"`
	Label     string         `json:"label"`
	Documents []documentJSON `json:"documents"`
}

type underlyingJSON struct {
	CustomerType string         `json:"customerType"`
	MemoNumber   string         `json:"memoNumber"`
	TotalLimitMB int            `json:"totalLimitMb"`
	Categories   []categoryJSON `json:"categories"`
	ListError    *errorJSON     `json:"listError"`
	Editable     bool           `json:"editable"`
}

func toUnderlyingJSON(p cases.UnderlyingPage) underlyingJSON {
	out := underlyingJSON{CustomerType: p.CustomerType, MemoNumber: p.MemoNumber, TotalLimitMB: p.TotalLimitMB, Categories: []categoryJSON{}, Editable: p.Editable}
	if p.ListFailed {
		out.ListError = &errorJSON{"ERR-027", cases.MsgListFailed}
	}
	for _, c := range p.Categories {
		cat := categoryJSON{Code: c.Code, Label: c.Label, Documents: []documentJSON{}}
		for _, d := range c.Documents {
			doc := documentJSON{DocumentType: d.DocumentType, Mandatory: d.Mandatory}
			if d.File != nil {
				doc.File = &storedFileJSON{d.File.FileID, d.File.FileName}
			}
			cat.Documents = append(cat.Documents, doc)
		}
		out.Categories = append(out.Categories, cat)
	}
	return out
}

// maxFormField bounds each text field of the upload form.
const maxFormField = 4 << 10

// uploadDocument handles POST .../assignments/{id}/documents: Submit on the
// Upload Document modal (FR-FILE-002). The form carries category,
// documentType and name, then the file part "file"; only the first file is
// used (FR-FILE-002 rule 1). The file is read up to the per-file limit, so a
// larger request is ERR-041 without being read in full.
func (s *server) uploadDocument(ep endpoint) http.HandlerFunc {
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
		max, err := s.cases.MaxFileBytes(r.Context())
		if err != nil {
			slog.Error("upload refused: per-file limit setting unusable", "error", err)
			writeStorageFailed(w)
			return
		}
		// The body may exceed the file limit only by the form's own size.
		r.Body = http.MaxBytesReader(w, r.Body, max+1<<20)
		req, haveFile, err := readUploadForm(r, max)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !haveFile && !req.TooLarge {
			w.WriteHeader(http.StatusBadRequest) // Submit without a file: TBD-036a
			return
		}
		f, err := s.cases.UploadDocument(r.Context(), sess, caseID, id, req)
		if errors.Is(err, cases.ErrStorageFailed) {
			writeStorageFailed(w)
			return
		}
		if writeAssignmentError(w, "upload document", err) {
			return
		}
		writeJSON(w, http.StatusCreated, storedFileJSON{f.FileID, f.FileName})
	}
}

// readUploadForm reads the upload form, marking the request TooLarge when it
// passes the per-file limit (ERR-041).
func readUploadForm(r *http.Request, max int64) (req cases.UploadRequest, haveFile bool, err error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return req, false, err
	}
	var tooBig *http.MaxBytesError
	fields := map[string]*string{"category": &req.Category, "documentType": &req.DocumentType, "name": &req.Name}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return req, haveFile, nil
		}
		if errors.As(err, &tooBig) {
			req.TooLarge = true
			return req, haveFile, nil
		}
		if err != nil {
			return req, false, err
		}
		if field, ok := fields[part.FormName()]; ok {
			b, err := io.ReadAll(io.LimitReader(part, maxFormField))
			if errors.As(err, &tooBig) {
				req.TooLarge = true
				return req, haveFile, nil
			}
			if err != nil {
				return req, false, err
			}
			*field = string(b)
			continue
		}
		if part.FormName() != "file" || haveFile {
			continue
		}
		haveFile = true
		req.OriginalFileName, req.ContentType = part.FileName(), part.Header.Get("Content-Type")
		var buf bytes.Buffer
		n, err := io.Copy(&buf, io.LimitReader(part, max+1))
		if errors.As(err, &tooBig) || n > max {
			req.TooLarge = true
			return req, haveFile, nil
		}
		if err != nil {
			return req, false, err
		}
		req.Data = buf.Bytes()
	}
}

// writeStorageFailed answers ERR-025 in the modal's error list form.
func writeStorageFailed(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadGateway, struct {
		Errors []errorJSON `json:"errors"`
	}{[]errorJSON{{"ERR-025", cases.MsgStorageFailed}}})
}

// downloadDocument handles GET <desktop>/api/cases/{caseId}/documents/{fileId}:
// the file as an attachment named file_name, for every type (DEC-114).
func (s *server) downloadDocument(ep endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.session(w, r, ep)
		if !ok {
			return
		}
		d, err := s.cases.OpenDocument(r.Context(), sess, r.PathValue("caseId"), r.PathValue("fileId"))
		switch {
		case errors.Is(err, cases.ErrNotFound):
			w.WriteHeader(http.StatusNotFound)
			return
		case errors.Is(err, cases.ErrNoFileAccess):
			writeJSON(w, http.StatusForbidden, errorJSON{"ERR-035", "Unable to open the attachment. You lack the required permissions."})
			return
		case errors.Is(err, cases.ErrStorageFailed):
			writeStorageFailed(w)
			return
		case errors.Is(err, cases.ErrDownloadFailed):
			w.WriteHeader(http.StatusBadGateway) // message: TBD-037e
			return
		case err != nil:
			slog.Error("download document failed", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer d.Body.Close()
		ctype := d.ContentType
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": d.FileName}))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if _, err := io.Copy(w, d.Body); err != nil {
			slog.Error("sending the file failed", "error", err)
		}
	}
}
