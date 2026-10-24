package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"fos/internal/auth"
	"fos/internal/cases"
)

// caseErrors gives the SoT case errors their status and message (§12).
// ERR-005 and ERR-022 texts are TBD in the SoT (TBD-032); these are interim.
var caseErrors = map[string]struct {
	status  int
	message string
}{
	"ERR-005": {http.StatusServiceUnavailable, "Case ID gagal dibentuk. Silakan coba lagi."},
	"ERR-006": {http.StatusOK, "Branch ID operator tidak ditemukan"},
	"ERR-022": {http.StatusForbidden, "Anda tidak memiliki akses untuk membuat pengajuan pembiayaan."},
}

type errorJSON struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeCaseError(w http.ResponseWriter, code string) {
	e := caseErrors[code]
	writeJSON(w, e.status, errorJSON{code, e.message})
}

// session returns the request's session on ep, or writes 401.
func (s *server) session(w http.ResponseWriter, r *http.Request, ep endpoint) (auth.Session, bool) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if sess, ok := s.auth.Session(r.Context(), c.Value, ep.id); ok {
			return sess, true
		}
	}
	w.WriteHeader(http.StatusUnauthorized)
	return auth.Session{}, false
}

type caseTypeJSON struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

func toCaseTypesJSON(types []cases.CaseType) []caseTypeJSON {
	out := make([]caseTypeJSON, 0, len(types))
	for _, t := range types {
		out = append(out, caseTypeJSON{t.Code, t.Label})
	}
	return out
}

// createCase handles POST <desktop>/api/cases (FR-APP-001).
func (s *server) createCase(ep endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.session(w, r, ep)
		if !ok {
			return
		}
		var req struct {
			CaseType string `json:"caseType"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		created, err := s.cases.Create(r.Context(), sess, req.CaseType)
		switch {
		case errors.Is(err, cases.ErrNotPermitted):
			writeCaseError(w, "ERR-022")
		case errors.Is(err, cases.ErrUnknownCaseType):
			w.WriteHeader(http.StatusBadRequest)
		case errors.Is(err, cases.ErrCaseIDNotFormed):
			writeCaseError(w, "ERR-005")
		case err != nil:
			slog.Error("create case failed", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
		default:
			writeJSON(w, http.StatusCreated, struct {
				CaseID       string `json:"caseId"`
				AssignmentID int64  `json:"assignmentId"`
			}{created.CaseID, created.AssignmentID})
		}
	}
}

type personJSON struct {
	UserID string    `json:"userId"`
	Name   string    `json:"name"`
	At     time.Time `json:"at"`
}

// caseFrame handles GET <desktop>/api/cases/{caseId}: the Case View frame
// shared by both modes. Who may open a case is decided in ticket 32
// (TBD-035a); until then any authenticated desktop operator may read it.
func (s *server) caseFrame(ep endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.session(w, r, ep); !ok {
			return
		}
		f, err := s.cases.Frame(r.Context(), r.PathValue("caseId"))
		if errors.Is(err, cases.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err != nil {
			slog.Error("case frame failed", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		type stageJSON struct {
			Name    string `json:"name"`
			Current bool   `json:"current"`
			Entered bool   `json:"entered"`
		}
		type processJSON struct {
			Name    string `json:"name"`
			Started bool   `json:"started"`
		}
		type lifecycleJSON struct {
			Stage     string        `json:"stage"`
			Entered   bool          `json:"entered"`
			Processes []processJSON `json:"processes"`
		}
		stages := make([]stageJSON, 0, len(f.Stages))
		for _, st := range f.Stages {
			stages = append(stages, stageJSON(st))
		}
		lifecycle := make([]lifecycleJSON, 0, len(f.Lifecycle))
		for _, ls := range f.Lifecycle {
			procs := make([]processJSON, 0, len(ls.Processes))
			for _, p := range ls.Processes {
				procs = append(procs, processJSON(p))
			}
			lifecycle = append(lifecycle, lifecycleJSON{ls.Stage, ls.Entered, procs})
		}
		type attachmentJSON struct {
			FileID        string    `json:"fileId"`
			FileName      string    `json:"fileName"`
			CategoryLabel string    `json:"categoryLabel"`
			UploadedBy    string    `json:"uploadedBy"`
			CreatedAt     time.Time `json:"createdAt"`
		}
		attachments := make([]attachmentJSON, 0, len(f.Attachments))
		for _, a := range f.Attachments {
			attachments = append(attachments, attachmentJSON(a))
		}
		writeJSON(w, http.StatusOK, struct {
			CaseID          string           `json:"caseId"`
			CaseType        string           `json:"caseType"`
			CaseTypeLabel   string           `json:"caseTypeLabel"`
			Priority        int              `json:"priority"`
			Status          string           `json:"status"`
			Created         personJSON       `json:"created"`
			Updated         personJSON       `json:"updated"`
			Stages          []stageJSON      `json:"stages"`
			Lifecycle       []lifecycleJSON  `json:"lifecycle"`
			AttachmentCount int              `json:"attachmentCount"`
			Attachments     []attachmentJSON `json:"attachments"`
		}{
			f.CaseID, f.CaseType.Code, f.CaseType.Label, f.Priority, f.CaseStatus,
			personJSON(f.Created), personJSON(f.Updated), stages, lifecycle, f.AttachmentCount, attachments,
		})
	}
}

// openAssignment handles GET <desktop>/api/cases/{caseId}/assignments/{id}:
// the assignment in mode Assignment (FR-CASE-008).
func (s *server) openAssignment(ep endpoint) http.HandlerFunc {
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
		p, err := s.cases.OpenAssignment(r.Context(), sess, caseID, id)
		if writeAssignmentError(w, "open assignment", err) {
			return
		}
		type preScreeningJSON struct {
			BranchName          string       `json:"branchName"`
			BranchError         *errorJSON   `json:"branchError"`
			CustomerType        string       `json:"customerType"`
			CustomerTypeOptions []optionJSON `json:"customerTypeOptions"`
			Customer            personForm   `json:"customer"`
			Spouse              personForm   `json:"spouse"`
			Editable            bool         `json:"editable"`
			Today               string       `json:"today"`
			EarliestBirthDate   string       `json:"earliestBirthDate"`
			NotesBanner         *notesJSON   `json:"notesBanner"`
		}
		var page *preScreeningJSON
		if ps := p.PreScreening; ps != nil {
			page = &preScreeningJSON{BranchName: ps.BranchName, CustomerType: ps.CustomerType, CustomerTypeOptions: toOptionsJSON(ps.CustomerTypeOptions),
				Customer: personForm(ps.Customer), Spouse: personForm(ps.Spouse),
				Editable: ps.Editable, Today: ps.Today, EarliestBirthDate: ps.EarliestBirthDate}
			if ps.NotesBanner != nil {
				page.NotesBanner = &notesJSON{ps.NotesBanner.Decision, ps.NotesBanner.Notes}
			}
			if ps.BranchMissing {
				page.BranchError = &errorJSON{"ERR-006", caseErrors["ERR-006"].message}
			}
		}
		var body any
		switch {
		case page != nil:
			body = page
		case p.Underlying != nil:
			body = toUnderlyingJSON(*p.Underlying)
		case p.Review != nil:
			body = toReviewJSON(*p.Review)
		case p.WaitingSLIK != nil:
			body = toWaitingSLIKJSON(*p.WaitingSLIK)
		case p.Pipeline != nil:
			body = toPipelineJSON(*p.Pipeline)
		case p.IndividualData != nil:
			body = toIndividualDataPageJSON(*p.IndividualData)
		case p.Spouse != nil:
			body = toSpousePageJSON(*p.Spouse)
		case p.BusinessData != nil:
			body = toBusinessDataPageJSON(*p.BusinessData)
		case p.OtherFinancing != nil:
			body = toOtherFinancingPageJSON(*p.OtherFinancing)
		case p.FacilityRequest != nil:
			body = toFacilityRequestPageJSON(*p.FacilityRequest)
		case p.QCA != nil:
			body = toQCAPageJSON(*p.QCA)
		case p.Collateral != nil:
			body = toCollateralPageJSON(*p.Collateral)
		case p.BusinessEntity != nil:
			body = toBusinessEntityPageJSON(*p.BusinessEntity)
		case p.CloseRelative != nil:
			body = toCloseRelativePageJSON(*p.CloseRelative)
		}
		writeJSON(w, http.StatusOK, struct {
			AssignmentID int64  `json:"assignmentId"`
			Name         string `json:"name"`
			Status       string `json:"status"`
			Page         any    `json:"page"`
		}{p.AssignmentID, p.Name, string(p.Status), body})
	}
}

// personForm is a Data Pribadi or Data Pasangan block; birthDate is yyyy-mm-dd.
type personForm struct {
	NIK       string `json:"nik"`
	Name      string `json:"name"`
	BirthDate string `json:"birthDate"`
}

// writeAssignmentError answers a failed assignment request and reports
// whether err was a failure: validation errors (422) or a refusal.
func writeAssignmentError(w http.ResponseWriter, action string, err error) bool {
	var invalid *cases.ValidationError
	switch {
	case err == nil:
		return false
	case errors.As(err, &invalid):
		type fieldErrorJSON struct {
			Section string `json:"section"`
			Key     string `json:"key"`
			Field   string `json:"field"`
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		out := make([]fieldErrorJSON, 0, len(invalid.Fields))
		for _, f := range invalid.Fields {
			out = append(out, fieldErrorJSON(f))
		}
		writeJSON(w, http.StatusUnprocessableEntity, struct {
			Errors []fieldErrorJSON `json:"errors"`
		}{out})
	case errors.Is(err, cases.ErrNotFound):
		w.WriteHeader(http.StatusNotFound)
	case errors.Is(err, cases.ErrForbidden):
		w.WriteHeader(http.StatusForbidden)
	case errors.Is(err, cases.ErrCompleted):
		w.WriteHeader(http.StatusConflict)
	case errors.Is(err, cases.ErrUnsupported):
		w.WriteHeader(http.StatusBadRequest)
	default:
		slog.Error(action+" failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	return true
}

// writeMoveResult answers a submit or back with the opened next assignment.
func writeMoveResult(w http.ResponseWriter, next int64, err error) {
	if writeAssignmentError(w, "move assignment", err) {
		return
	}
	writeJSON(w, http.StatusOK, struct {
		AssignmentID int64 `json:"assignmentId"`
	}{next})
}

// pathAssignment reads {caseId} and {assignmentId}.
func pathAssignment(r *http.Request) (string, int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("assignmentId"), 10, 64)
	return r.PathValue("caseId"), id, err == nil
}

// submitAssignment handles POST .../assignments/{id}/submit: Next or the
// breadcrumb on Pre-Screening Submission. Only this form's fields are
// accepted (BR-UI-010).
func (s *server) submitAssignment(ep endpoint) http.HandlerFunc {
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
			CustomerType string     `json:"customerType"`
			Customer     personForm `json:"customer"`
			Spouse       personForm `json:"spouse"`
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		next, err := s.cases.SubmitPreScreening(r.Context(), sess, caseID, id, cases.PreScreeningForm{
			CustomerType: req.CustomerType,
			Customer:     cases.Person(req.Customer),
			Spouse:       cases.Person(req.Spouse),
		})
		writeMoveResult(w, next, err)
	}
}

// backAssignment handles POST .../assignments/{id}/back: the breadcrumb back
// from Document Underlying Upload.
func (s *server) backAssignment(ep endpoint) http.HandlerFunc {
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
		next, err := s.cases.BackToPreScreening(r.Context(), sess, caseID, id)
		writeMoveResult(w, next, err)
	}
}
