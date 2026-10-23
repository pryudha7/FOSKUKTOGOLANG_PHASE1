package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"fos/internal/cases"
)

// The external service namespace (DB-RULE-067): every hit is logged in the
// Connect API Log as Service Rest, known API or not.
const serviceNamespace = "/api/NBS/"

// rpaRole is the role of the account the RPA uses (agreed for TBD-044).
const rpaRole = "RPA"

// maxPBSBytes bounds a PBSResponse request (one SLIK ZIP).
const maxPBSBytes = 64 << 20

// pbsResponse handles POST /api/NBS/v1/PBSResponse (FR-PSAPI-009): Basic
// auth of the RPA account, multipart Kode and File. The answer is sent at
// once; its log row is written after it (DEC-073), and a resumed run starts
// after both.
func (s *server) pbsResponse(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	entry := cases.InboundLog{
		Method: r.Method, URL: r.URL.RequestURI(), RequestHeaders: r.Header.Clone(),
		ClientIP: s.clientIP(r), SubmittedAt: s.cases.Now(),
	}
	user, password, ok := r.BasicAuth()
	entry.AuthUsername = user
	refuse := func(status int) {
		w.WriteHeader(status)
		entry.HTTPStatus, entry.Status, entry.Duration = status, "Fail", time.Since(start)
		entry.ResponseHeader = w.Header().Clone()
		s.cases.LogInbound(r.Context(), entry)
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		refuse(http.StatusMethodNotAllowed)
		return
	}
	rpaUser := ""
	if ok {
		var err error
		if rpaUser, ok, err = s.auth.VerifyServiceAccount(r.Context(), user, password, rpaRole); err != nil {
			refuse(http.StatusInternalServerError)
			return
		}
	}
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="FOS"`)
		refuse(http.StatusUnauthorized)
		return
	}
	entry.APIName = "SLIKPBSResponse"

	kode, fileName, data, err := readPBSForm(w, r)
	var res cases.PBSResult
	resume := func() {}
	if err != nil {
		res = cases.PBSResult{Code: "1", Message: cases.UnexpectedReason(err.Error()), LogStatus: "Fail"}
	} else {
		res, resume = s.cases.ReceiveSLIK(r.Context(), rpaUser, kode, data)
	}
	// The request is logged as JSON with a description of the file, never its
	// content (DB-RULE-086).
	note := ""
	if fileName != "" || data != nil {
		note = fmt.Sprintf("[file %s, application/zip, %d bytes", fileName, len(data))
		if res.Stored != nil {
			note += ", document " + res.Stored.String()
		}
		note += "]"
	}
	reqJSON, _ := json.Marshal(struct {
		Kode string `json:"Kode"`
		File string `json:"File"`
	}{kode, note})

	body := res.Answer(s.cases.Now())
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	entry.CaseID, entry.Status, entry.Request, entry.Response = res.CaseID, res.LogStatus, string(reqJSON), string(body)
	entry.HTTPStatus, entry.ResponseHeader, entry.Duration = http.StatusOK, w.Header().Clone(), time.Since(start)
	s.cases.LogInbound(r.Context(), entry)
	resume()
}

// readPBSForm reads the Kode field and the File part.
func readPBSForm(w http.ResponseWriter, r *http.Request) (kode, fileName string, data []byte, err error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPBSBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		return "", "", nil, errors.New("the request is not multipart/form-data")
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return kode, fileName, data, errors.New("the request could not be read")
		}
		switch part.FormName() {
		case "Kode":
			b, err := io.ReadAll(io.LimitReader(part, 1024))
			if err != nil {
				return kode, fileName, data, errors.New("the Kode could not be read")
			}
			kode = string(b)
		case "File":
			fileName = part.FileName()
			if data, err = io.ReadAll(part); err != nil {
				return kode, fileName, nil, errors.New("the File could not be read")
			}
		}
	}
	if kode == "" || len(data) == 0 {
		return kode, fileName, data, errors.New("the request needs Kode and File")
	}
	return kode, fileName, data, nil
}

// serviceUnknown answers and logs any other hit on the service namespace
// (DB-RULE-067): no case, no API name, status Fail.
func (s *server) serviceUnknown(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	user, _, _ := r.BasicAuth()
	w.WriteHeader(http.StatusNotFound)
	s.cases.LogInbound(r.Context(), cases.InboundLog{
		Method: r.Method, URL: r.URL.RequestURI(), HTTPStatus: http.StatusNotFound, Status: "Fail",
		RequestHeaders: r.Header.Clone(), ResponseHeader: w.Header().Clone(),
		ClientIP: s.clientIP(r), AuthUsername: user, SubmittedAt: s.cases.Now(), Duration: time.Since(start),
	})
}
