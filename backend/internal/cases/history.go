package cases

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"fos/internal/auth"
)

// History Details (UI-013) under an opened assignment: API Log History
// (UI-020, every Connect API Log row of the case) and Pre-Screening API Data
// (UI-021, FR-PSAPI-005: the latest request and response of each
// Pre-Screening Check process, read from the log when the page opens,
// DEC-057).

// noValue shows a field missing from a request or response (FR-PSAPI-005
// rule 4).
const noValue = "--"

// dhnMessage is the fixed DHN Message while DHN does not reject
// (FR-PSAPI-005 rule 8).
const dhnMessage = "Pengecekan DHN tidak dilakukan, mohon pastikan cabang memberikan bukti DHN"

// HistoryDetails is the History Details card of a case.
type HistoryDetails struct {
	Logs      []APILogEntry
	Cut       LogCut
	Processes []APIDataProcess
}

// LogCut is how API Log History cuts long Request and Response on display
// (ui/apilog/truncate_chars and truncate_lines, UI-020).
type LogCut struct{ Chars, Lines int }

// Document kinds of a process: the file View SLIK or Download PDF fetches.
const (
	DocumentSLIKReport    = "slik"
	DocumentPefindoReport = "pefindo"
)

// APIDataProcess is one sub-accordion of Pre-Screening API Data. Rejected
// shows the ERR-034 banner on REQUEST; RejectedOnly is the Rejected List
// variant (banner only, no tabs). Document is the file of View SLIK or
// Download PDF, when there is one.
type APIDataProcess struct {
	Name         string
	Rejected     bool
	RejectedOnly bool
	Request      []APIDataField
	Response     []APIDataField
	Errors       []string         // SLIK: one line each
	Results      [][]APIDataField // Pefindo: the data blocks, highest Similarity Score first
	Document     *CaseFile
	DocumentKind string // DocumentSLIKReport or DocumentPefindoReport
}

// CaseFile is a case document to download: its id and file_name.
type CaseFile struct{ ID, Name string }

// APIDataField is one read-only field; Indent puts it under Params.
type APIDataField struct {
	Label, Value string
	Indent       bool
}

// History returns the History Details card for whoever may open the
// assignment (worklist holder or Authors, as OpenAssignment).
func (s *Service) History(ctx context.Context, sess auth.Session, caseID string, assignmentID int64) (HistoryDetails, error) {
	var kind string
	var assignee *string
	err := s.db.QueryRow(ctx, `SELECT assignment_type, assigned_user_id FROM fos.assignments WHERE assignment_id = $1 AND case_id = $2`,
		assignmentID, caseID).Scan(&kind, &assignee)
	if isNotFound(err) {
		return HistoryDetails{}, ErrNotFound
	}
	if err != nil {
		return HistoryDetails{}, err
	}
	if _, may := mayOpen(sess, AssignmentType(kind), value(assignee)); !may {
		return HistoryDetails{}, ErrForbidden
	}

	var h HistoryDetails
	if h.Cut, err = s.logCut(ctx); err != nil {
		return h, err
	}
	loc, err := operatorLocationOf(ctx, s.db, sess.UserID)
	if err != nil {
		slog.Error("operator timezone unreadable; showing times in UTC", "user_id", sess.UserID, "error", err)
		loc = time.UTC
	}
	// Newest first (Gambar 36; agreed for TBD-042k).
	if h.Logs, err = collect(ctx, s, logEntryIn(loc), `
		SELECT api_name, submitted_at, api_status, COALESCE(request, ''), COALESCE(response, '')
		FROM fos.connect_api_log WHERE case_id = $1
		ORDER BY submitted_at DESC, attempt_number DESC NULLS LAST`, caseID); err != nil {
		return h, err
	}
	if h.Logs == nil {
		h.Logs = []APILogEntry{}
	}
	h.Processes, err = s.apiData(ctx, caseID)
	return h, err
}

// logCut reads how API Log History cuts long values on display.
func (s *Service) logCut(ctx context.Context) (LogCut, error) {
	chars, err := s.settings.Int(ctx, "ui/apilog/truncate_chars")
	if err != nil {
		return LogCut{}, err
	}
	lines, err := s.settings.Int(ctx, "ui/apilog/truncate_lines")
	return LogCut{int(chars), int(lines)}, err
}

// mayOpen is who may open an assignment and see its History Details: the
// worklist holder (owner), or any member of Authors (FR-CASE-009).
func mayOpen(sess auth.Session, kind AssignmentType, assignee string) (owner, may bool) {
	owner = kind == Worklist && assignee == sess.UserID
	authors := sess.AccessGroup != nil && sess.AccessGroup.Name == "Authors"
	return owner, owner || authors
}

// logEntryIn scans a log row with Waktu Connect in the viewer's timezone,
// dd/MM/yy HH:mm (UI-020).
func logEntryIn(loc *time.Location) func(pgx.Rows) (APILogEntry, error) {
	return func(r pgx.Rows) (APILogEntry, error) {
		var (
			l  APILogEntry
			at time.Time
		)
		err := r.Scan(&l.Name, &at, &l.Status, &l.Request, &l.Response)
		l.SubmittedAt = at.In(loc).Format("02/01/06 15:04")
		l.At = at
		return l, err
	}
}

// latestLog is the latest row of one api_name.
type latestLog struct {
	status, request, response string
}

// apiData builds Pre-Screening API Data in FR-PSAPI-001's order; a process
// without a log row is left out (agreed for TBD-042).
func (s *Service) apiData(ctx context.Context, caseID string) ([]APIDataProcess, error) {
	latest := map[string]latestLog{}
	type row struct {
		name string
		l    latestLog
	}
	rows, err := collect(ctx, s, func(r pgx.Rows) (row, error) {
		var x row
		return x, r.Scan(&x.name, &x.l.status, &x.l.request, &x.l.response)
	}, `
		SELECT DISTINCT ON (api_name) api_name, api_status, COALESCE(request, ''), COALESCE(response, '')
		FROM fos.connect_api_log WHERE case_id = $1 AND api_name IS NOT NULL
		ORDER BY api_name, submitted_at DESC, attempt_number DESC NULLS LAST`, caseID)
	if err != nil {
		return nil, err
	}
	for _, x := range rows {
		latest[x.name] = x.l
	}
	docs, err := s.documentFiles(ctx, caseID, docPDFCustomer, docPDFSpouse, docPefindoCustomer, docPefindoSpouse)
	if err != nil {
		return nil, err
	}

	var out []APIDataProcess
	if l, ok := latest[dukcapilAPI.logName]; ok {
		out = append(out, dukcapilData(l))
	}
	if l, ok := latest[dhnAPI.logName]; ok {
		out = append(out, dhnData(l))
	}
	// Rejected List: only after it rejected the case (FR-PSAPI-005 rule 6).
	var rejectedByList bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM fos.prescreening_runs WHERE run_id = `+latestRun+` AND state = $2 AND current_step = $3)`,
		caseID, runStoppedRejected, stepRejectedList).Scan(&rejectedByList); err != nil {
		return nil, err
	}
	if rejectedByList {
		out = append(out, APIDataProcess{Name: stepRejectedList, Rejected: true, RejectedOnly: true})
	}
	for _, code := range []string{subjectCustomer, subjectSpouse} {
		subject := slikSubjects[code]
		if l, ok := latest[subject.api.logName]; ok {
			p, err := s.slikData(ctx, caseID, subject, l)
			if err != nil {
				return nil, err
			}
			if f, ok := docs[subject.pdfDoc]; ok {
				p.Document, p.DocumentKind = &f, DocumentSLIKReport
			}
			out = append(out, p)
		}
	}
	for _, code := range []string{subjectCustomer, subjectSpouse} {
		subject := pefindoSubjects[code]
		l, ok := latest[subject.search.logName]
		if !ok {
			continue
		}
		p := pefindoData(subject.search.logName, l)
		// The sub-accordion stands for every Pefindo API of the subject
		// (FR-PSAPI-005 rules 7 and 9).
		for _, api := range []outboundAPI{subject.search, subject.generate, subject.download, subject.report} {
			if latest[api.logName].status == apiRejected {
				p.Rejected = true
			}
		}
		if f, ok := docs[subject.pdfDoc]; ok {
			p.Document, p.DocumentKind = &f, DocumentPefindoReport
		}
		out = append(out, p)
	}
	if out == nil {
		out = []APIDataProcess{}
	}
	return out, nil
}

// documentFiles maps the case's automatic documents of the given types to
// their files.
func (s *Service) documentFiles(ctx context.Context, caseID string, types ...string) (map[string]CaseFile, error) {
	type doc struct {
		docType string
		file    CaseFile
	}
	docs, err := collect(ctx, s, func(r pgx.Rows) (doc, error) {
		var d doc
		return d, r.Scan(&d.docType, &d.file.ID, &d.file.Name)
	}, `SELECT document_type, file_id::text, file_name FROM fos.case_document WHERE case_id = $1 AND document_category = $2 AND document_type = ANY($3)`,
		caseID, categoryResponse, types)
	out := map[string]CaseFile{}
	for _, d := range docs {
		out[d.docType] = d.file
	}
	return out, err
}

// jsonObject parses a logged request or response; nil when it is not a
// JSON object (the fields then show "--", FR-PSAPI-005 rule 4).
func jsonObject(text string) map[string]any {
	var m map[string]any
	if json.Unmarshal([]byte(text), &m) != nil {
		return nil
	}
	return m
}

// shown is a JSON value as a field shows it: text as is, numbers and
// booleans as written, "--" when missing or empty.
func shown(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		if v != "" {
			return v
		}
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}
	return noValue
}

func fields(m map[string]any, pairs ...string) []APIDataField {
	var out []APIDataField
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, APIDataField{Label: pairs[i], Value: shown(m, pairs[i+1])})
	}
	return out
}

// dukcapilData: FR-PSAPI-005 rule 5.
func dukcapilData(l latestLog) APIDataProcess {
	req, resp := jsonObject(l.request), jsonObject(l.response)
	return APIDataProcess{
		Name: dukcapilAPI.logName, Rejected: l.status == apiRejected,
		Request:  fields(req, "NIK", "nik", "Nama Lengkap", "namaLengkap", "Tanggal Lahir", "tanggalLahir"),
		Response: fields(resp, "Response NIK", "NIK", "Response Nama Lengkap", "NAMA_LGKP", "Response Tanggal Lahir", "TGL_LHR"),
	}
}

// dhnData: FR-PSAPI-005 rule 8; the request column holds the identity
// number only.
func dhnData(l latestLog) APIDataProcess {
	identity := l.request
	if identity == "" {
		identity = noValue
	}
	resp := jsonObject(l.response)
	return APIDataProcess{
		Name:    dhnAPI.logName,
		Request: []APIDataField{{Label: "Identity Number", Value: identity}},
		Response: []APIDataField{
			{Label: "Code", Value: shown(resp, "code")},
			{Label: "Message", Value: dhnMessage},
			{Label: "Blacklisted", Value: shown(obj(resp, "data"), "blacklisted")},
		},
	}
}

// slikData: FR-PSAPI-005 rule 10; dropdown codes show their BMV
// description, read without a status filter (agreed for ticket 14). A SLIK
// reject is logged on the result's SLIKPBSResponse row, not on the request
// (FR-PSAPI-010 rule 4), so the banner comes from there (rule 9).
func (s *Service) slikData(ctx context.Context, caseID string, subject slikSubject, l latestLog) (APIDataProcess, error) {
	name := subject.api.logName
	req, resp := jsonObject(l.request), jsonObject(l.response)
	described := func(bmvType, key string) (string, error) {
		code := shown(req, key)
		if code == noValue {
			return code, nil
		}
		var description *string
		err := s.db.QueryRow(ctx, `SELECT description FROM fos.business_mapping_value WHERE type = $1 AND sub_type = '' AND code = $2`,
			bmvType, code).Scan(&description)
		if isNotFound(err) || (err == nil && value(description) == "") {
			return code, nil
		}
		return value(description), err
	}
	rejected, err := s.slikRejected(ctx, caseID, subject)
	if err != nil {
		return APIDataProcess{}, err
	}
	p := APIDataProcess{Name: name, Rejected: rejected}
	for _, f := range []struct{ label, key, bmvType string }{
		{"Kode Reff", "kode_reff_pengguna", ""},
		{"Tujuan Permintaan", "tujuan_permintaan", bmvRequestPurpose},
		{"Nomor Identitas", "nomor_identitas", ""},
		{"Pilihan Fasilitas", "pilihan_fasilitas", bmvFacilityOption},
		{"Nomor Memo", "nomor_memo", ""},
		{"Alamat", "alamat_sesuai_identitas", ""},
		{"Tipe Pencarian", "tipe_pencarian", bmvSearchType},
		{"Segmentasi", "segmentasi", bmvSegmentation},
	} {
		v := shown(req, f.key)
		if f.bmvType != "" {
			var err error
			if v, err = described(f.bmvType, f.key); err != nil {
				return p, err
			}
		}
		p.Request = append(p.Request, APIDataField{Label: f.label, Value: v})
	}
	p.Response = fields(resp, "Code", "code", "Message", "message")
	if list, ok := resp["errors"].([]any); ok {
		for _, e := range list {
			if text, ok := e.(string); ok {
				p.Errors = append(p.Errors, text)
				continue
			}
			b, _ := json.Marshal(e) // an object or number shows as JSON
			p.Errors = append(p.Errors, string(b))
		}
	}
	return p, nil
}

// slikRejected reports whether the subject's SLIK result was rejected: a
// Rejected SLIKPBSResponse row for its reference code.
func (s *Service) slikRejected(ctx context.Context, caseID string, subject slikSubject) (bool, error) {
	var code *string
	err := s.db.QueryRow(ctx, `SELECT reference_code FROM fos.slik_report WHERE case_id = $1 AND subject_type = $2`, caseID, subject.code).Scan(&code)
	if isNotFound(err) || code == nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	requests, err := collect(ctx, s, func(r pgx.Rows) (string, error) {
		var req string
		return req, r.Scan(&req)
	}, `SELECT COALESCE(request, '') FROM fos.connect_api_log WHERE case_id = $1 AND api_name = $2 AND api_status = $3`,
		caseID, apiPBSResponse, apiRejected)
	if err != nil {
		return false, err
	}
	for _, req := range requests {
		if shown(jsonObject(req), "Kode") == *code {
			return true, nil
		}
	}
	return false, nil
}

// pefindoData: FR-PSAPI-005 rule 11, from the Search row. Report Date always
// shows "--" (UI-021). Result fields accept both namings the SoT gives for
// a data element (the sample's and FR-PSAPI-014's list).
func pefindoData(name string, l latestLog) APIDataProcess {
	req, resp := jsonObject(l.request), jsonObject(l.response)
	p := APIDataProcess{Name: name}
	p.Request = fields(req, "Type", "type", "Product ID", "product_id", "Inquiry Reason", "inquiry_reason", "Reference Code", "reference_code")
	p.Request = append(p.Request, APIDataField{Label: "Params", Value: ""})
	var param map[string]any
	if list, ok := req["params"].([]any); ok && len(list) > 0 {
		param, _ = list[0].(map[string]any)
	}
	for _, f := range fields(param, "ID Type", "id_type", "ID No", "id_no", "Name", "name", "Date Of Birth", "date_of_birth") {
		f.Indent = true
		p.Request = append(p.Request, f)
	}
	p.Request = append(p.Request, APIDataField{Label: "Report Date", Value: noValue, Indent: true})
	p.Response = fields(resp, "Code", "code", "API Status", "status", "Message", "message", "Inquiry ID", "inquiry_id")

	list, _ := resp["data"].([]any)
	type result struct {
		score  float64
		fields []APIDataField
	}
	var results []result
	either := func(m map[string]any, keys ...string) string {
		for _, k := range keys {
			if v := shown(m, k); v != noValue {
				return v
			}
		}
		return noValue
	}
	for _, it := range list {
		m, _ := it.(map[string]any)
		score, _ := strconv.ParseFloat(either(m, "similarity_score"), 64)
		results = append(results, result{score, []APIDataField{
			{Label: "Similarity Score", Value: either(m, "similarity_score")},
			{Label: "ID Pefindo", Value: either(m, "id_pefindo")},
			{Label: "ID No", Value: either(m, "id_no")},
			{Label: "Tipe Pencarian", Value: either(m, "tipe_pencarian")},
			{Label: "Nama Debitur", Value: either(m, "nama_debitur", "debtor_name")},
			{Label: "Tanggal Lahir", Value: either(m, "tanggal_lahir", "birth_date")},
			{Label: "NPWP", Value: either(m, "npwp")},
			{Label: "Alamat", Value: either(m, "alamat", "address")},
			{Label: "Nama Gadis Ibu Kandung", Value: either(m, "nama_gadis_ibu_kandung", "mother_maiden_name")},
			{Label: "Tempat Lahir", Value: either(m, "tempat_lahir", "birth_place")},
			{Label: "Jenis Kelamin", Value: either(m, "jenis_kelamin", "gender")},
			{Label: "Kode Kota", Value: either(m, "kode_kota")},
			{Label: "Kode Pos", Value: either(m, "kode_pos", "postal_code")},
			{Label: "Ketersediaan Data Kredit", Value: either(m, "ketersediaan_data_kredit")},
		}})
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].score > results[j].score })
	for _, r := range results {
		p.Results = append(p.Results, r.fields)
	}
	return p
}
