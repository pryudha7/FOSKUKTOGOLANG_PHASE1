package cases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Pefindo (FR-PSAPI-014 to FR-PSAPI-018) is one step per subject, Customer
// then Spouse, each a chain of APIs: Search, Generate Report, Download PDF,
// Get Report (DEC-096). The run's current_api holds the API in progress;
// each API has its own attempts and retry settings. The Reject is decided on
// Get Report (DEC-093).

const (
	stepPefindo       = "Pefindo"
	stepPefindoSpouse = "Pefindo Pasangan"

	// Pefindo PDF slots (DB-RULE-079).
	docPefindoCustomer = "PEFINDO_PDF_CUSTOMER"
	docPefindoSpouse   = "PEFINDO_PDF_SPOUSE"

	// pefindoCriterion is the Rejected List criterion (FR-PSAPI-017 Reject 5).
	pefindoCriterion = "Risk Grade di bawah D3"

	// pefindoEventID in the Download PDF and Get Report paths is replaced by
	// the Search's EventID.
	pefindoEventID = "{EventID}"
)

// Pefindo answer codes.
const (
	pefindoFound        = "01"
	pefindoTokenExpired = "06"
	pefindoNotFound     = "31"
	pefindoBigReport    = "36"
)

// mediaTypePDF is a Download PDF answer that is the file (FR-PSAPI-018).
const mediaTypePDF = "application/pdf"

// pefindoRejectedGrades reject on Get Report (FR-PSAPI-017 Reject 2).
var pefindoRejectedGrades = map[string]bool{"E1": true, "E2": true, "E3": true}

// pefindoSubject is what differs between the Customer's Pefindo and the
// Spouse's: the API names (same settings) and the PDF slot and name.
type pefindoSubject struct {
	code                               string
	search, generate, download, report outboundAPI
	pdfDoc, pdfSuffix                  string
}

func newPefindoSubject(code, name, pdfDoc, pdfSuffix string) pefindoSubject {
	api := func(suffix, settings, keyHeader string) outboundAPI {
		return outboundAPI{logName: name + suffix, settings: settings, base: "pefindo", keyHeader: keyHeader}
	}
	return pefindoSubject{
		code:     code,
		search:   api("", "pefindo", "x-gateway-apikey"), // only Search carries the key (FR-PSAPI-014, -016)
		generate: api("GenerateReport", "pefindogeneratereport", ""),
		download: api("DownloadPDF", "pefindodownloadpdf", ""),
		report:   api("GetReport", "pefindogetreport", ""),
		pdfDoc:   pdfDoc, pdfSuffix: pdfSuffix,
	}
}

var pefindoSubjects = map[string]pefindoSubject{
	subjectCustomer: newPefindoSubject(subjectCustomer, "Pefindo", docPefindoCustomer, ""),
	subjectSpouse:   newPefindoSubject(subjectSpouse, "PefindoPasangan", docPefindoSpouse, " - Spouse"),
}

// apiNamed is the subject's API with that api_name; "" is Search, the
// first. An unknown name (e.g. a renamed API) is a system failure, never a
// restart of the chain.
func (p pefindoSubject) apiNamed(name string) (outboundAPI, error) {
	if name == "" {
		return p.search, nil
	}
	for _, a := range []outboundAPI{p.search, p.generate, p.download, p.report} {
		if a.logName == name {
			return a, nil
		}
	}
	return outboundAPI{}, fmt.Errorf("the run is on an unknown Pefindo API %q", name)
}

func (e *Executor) pefindoCheck(p pefindoSubject) check {
	return check{
		apiName: p.search.logName, criterion: pefindoCriterion,
		try: func(ctx context.Context, r runRef) (attempt, error) { return e.tryPefindo(ctx, r, p) },
		retry: func(ctx context.Context, name string) (int, time.Duration, error) {
			api, err := p.apiNamed(name)
			if err != nil {
				return 0, 0, err
			}
			return e.retrySettings(api)(ctx, name)
		},
	}
}

// pefindoParty is the subject's identity as sent on Search and Generate
// Report (FR-PSAPI-014 Body).
type pefindoParty struct {
	business               bool
	idType, idNo, name     string
	birthDate, customerRef string
}

func (e *Executor) tryPefindo(ctx context.Context, r runRef, p pefindoSubject) (attempt, error) {
	customerType, err := caseCustomerType(ctx, e.s.db, r.caseID)
	if err != nil {
		return attempt{}, err
	}
	// A Business Entity has no Spouse (agreed for TBD-042c).
	if p.code == subjectSpouse && customerType != customerTypeIndividual {
		return attempt{outcome: outcomeSkip}, nil
	}
	customerRow := partySpouse
	if p.code == subjectCustomer {
		customerRow = customerParty(customerType)
	}
	var idNumber, npwp, name *string
	var birth *time.Time
	err = e.s.db.QueryRow(ctx, `SELECT id_number, npwp, full_name, birth_date FROM fos.customer WHERE case_id = $1 AND customer_type = $2`,
		r.caseID, customerRow).Scan(&idNumber, &npwp, &name, &birth)
	if isNotFound(err) && p.code == subjectSpouse {
		return attempt{outcome: outcomeSkip}, nil // no Spouse data: no Pefindo Pasangan
	}
	if err != nil && !isNotFound(err) {
		return attempt{}, err
	}
	var customerRef *string
	if err := e.s.db.QueryRow(ctx, `SELECT customer_code FROM fos.work WHERE case_id = $1`, r.caseID).Scan(&customerRef); err != nil {
		return attempt{}, err
	}
	party := pefindoParty{idType: "KTP", idNo: value(idNumber), name: value(name), customerRef: value(customerRef)}
	if customerType == customerTypeBusinessEntity {
		party.business, party.idType, party.idNo = true, "NPWP", value(npwp)
	} else if birth != nil {
		party.birthDate = birth.Format("2006-01-02")
	}

	api, err := p.apiNamed(r.api)
	if err != nil {
		return attempt{}, err
	}
	cfg, err := e.apiConfig(ctx, api)
	if err != nil {
		return attempt{}, err
	}
	cfg.maskedHeaders = append(cfg.maskedHeaders, "Authorization") // the bearer token (DB-RULE-081)
	log := &apiLog{name: api.logName, submittedAt: e.s.now(), method: http.MethodGet, attempt: r.attempt}
	// Search needs every field of its body (FR-PSAPI-014 Body).
	missing := ""
	switch {
	case party.idNo == "":
		missing = "the " + party.idType + " is empty"
	case party.customerRef == "":
		missing = "the customer code is empty"
	case party.name == "":
		missing = "the name is empty"
	case !party.business && party.birthDate == "":
		missing = "the birth date is empty"
	}
	if missing != "" {
		return failed(log, errors.New("Pefindo request data missing: "+missing)), nil
	}
	call := pefindoAttempt{cfg: cfg, log: log, run: r, subject: p, party: party}
	switch api {
	case p.search:
		return e.pefindoSearch(ctx, call)
	case p.generate:
		return e.pefindoGenerate(ctx, call)
	case p.download:
		return e.pefindoDownload(ctx, call)
	}
	return e.pefindoReport(ctx, call)
}

// pefindoAttempt is one attempt of a Pefindo API: its configuration and log
// row, the run, the subject and its identity.
type pefindoAttempt struct {
	cfg     apiConfig
	log     *apiLog
	run     runRef
	subject pefindoSubject
	party   pefindoParty
}

// pefindoAnswer is a Pefindo API's answer: its body, its code ("" when the
// body is not JSON with a code) and its media type.
type pefindoAnswer struct {
	body      []byte
	code      string
	mediaType string
}

// callPefindo sends one attempt with the bearer token, or logs the
// configured response of a simulation (which needs no token). A code 06
// renews the token before the attempt is retried (FR-PSAPI-015). nil means
// no answer: a failed call, or no token to call with.
func (e *Executor) callPefindo(ctx context.Context, cfg apiConfig, log *apiLog, path string, body []byte) (*pefindoAnswer, error) {
	if cfg.simulate {
		log.response, log.method = cfg.simulateResponse, ""
		return &pefindoAnswer{body: []byte(cfg.simulateResponse), code: pefindoCode([]byte(cfg.simulateResponse)), mediaType: "application/json"}, nil
	}
	u, err := cfg.endpoint(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", log.name, err)
	}
	token, err := e.pefindoToken(ctx)
	if err != nil {
		return nil, err
	}
	if token.value == "" {
		// The token request failed and is not repeated; the attempt fails
		// without a call and is retried (agreed for ticket 13).
		log.response, log.method = "the Pefindo token could not be obtained", ""
		return nil, nil
	}
	raw, err := e.call(ctx, cfg, log, u, body, map[string]string{"Authorization": "Bearer " + token.value, "Content-Type": "application/json"})
	if err != nil || raw == nil {
		return nil, err
	}
	a := &pefindoAnswer{body: raw, code: pefindoCode(raw)}
	if ct := log.responseHeaders["Content-Type"]; len(ct) > 0 {
		a.mediaType, _, _ = mime.ParseMediaType(ct[0])
	}
	if a.code == pefindoTokenExpired {
		if _, err := e.renewPefindoToken(ctx, token.nonce); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// pefindoCode is the code of a JSON answer, or "".
func pefindoCode(body []byte) string {
	var resp struct {
		Code json.RawMessage `json:"code"`
	}
	if json.Unmarshal(body, &resp) != nil {
		return ""
	}
	return jsonText(resp.Code)
}

// jsonText is a JSON string's text or a number's digits; "" otherwise.
func jsonText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return ""
}

// allNines is a code made only of 9s, at least two (FR-PSAPI-017 rule 2).
func allNines(code string) bool {
	return len(code) >= 2 && strings.Trim(code, "9") == ""
}

// judged ends an attempt with an outcome; a code 06 is retried at once.
func judged(log *apiLog, a *pefindoAnswer, o outcome) attempt {
	log.status = o.logStatus()
	return attempt{outcome: o, log: log, immediate: a != nil && a.code == pefindoTokenExpired && o == outcomeFail}
}

func (e *Executor) pefindoSearch(ctx context.Context, c pefindoAttempt) (attempt, error) {
	log, party := c.log, c.party
	productID, err := e.s.settings.String(ctx, "api/pefindo/product_id")
	if err != nil {
		return attempt{}, err
	}
	type param struct {
		DateOfBirth string `json:"date_of_birth,omitempty"`
		IDNo        string `json:"id_no"`
		IDType      string `json:"id_type"`
		Name        string `json:"name"`
	}
	kind := "PERSONAL"
	if party.business {
		kind = "CORPORATE"
	}
	body, _ := json.Marshal(struct {
		InquiryReason string  `json:"inquiry_reason"`
		ProductID     string  `json:"product_id"`
		ReferenceCode string  `json:"reference_code"`
		Type          string  `json:"type"`
		Params        []param `json:"params"`
	}{"12", productID, party.customerRef, kind, []param{{party.birthDate, party.idNo, party.idType, party.name}}})
	log.method, log.request = http.MethodPost, string(body)
	a, err := e.callPefindo(ctx, c.cfg, log, c.cfg.path, body)
	if err != nil || a == nil {
		return judged(log, a, outcomeFail), err
	}
	switch a.code {
	case pefindoNotFound:
		return judged(log, a, outcomeSuccess), nil // no further Pefindo API for the subject
	case pefindoFound:
	default:
		return judged(log, a, outcomeFail), nil
	}
	var resp struct {
		Data []struct {
			Similarity json.RawMessage `json:"similarity_score"`
			IDPefindo  json.RawMessage `json:"id_pefindo"`
		} `json:"data"`
		InquiryID json.RawMessage `json:"inquiry_id"`
	}
	if json.Unmarshal(a.body, &resp) != nil {
		return judged(log, a, outcomeFail), nil
	}
	// The highest similarity_score, a number or a string (agreed for ticket 13).
	best, bestScore := -1, 0.0
	for i, d := range resp.Data {
		score, err := strconv.ParseFloat(jsonText(d.Similarity), 64)
		if err != nil || jsonText(d.IDPefindo) == "" {
			continue
		}
		if best < 0 || score > bestScore {
			best, bestScore = i, score
		}
	}
	if best < 0 || jsonText(resp.InquiryID) == "" {
		return judged(log, a, outcomeFail), nil // not the documented format
	}
	// The EventID is made only for code 01 (FR-PSAPI-014 Data yang disimpan).
	log.eventID, log.pefindoID, log.inquiryID = uuid.NewString(), jsonText(resp.Data[best].IDPefindo), jsonText(resp.InquiryID)
	at := judged(log, a, outcomeSuccess)
	at.nextAPI = c.subject.generate.logName
	return at, nil
}

// pefindoSearchResult is the subject's latest successful Search with an
// EventID (FR-PSAPI-016 Body).
type pefindoSearchResult struct{ eventID, pefindoID, inquiryID string }

func (e *Executor) lastPefindoSearch(ctx context.Context, caseID string, p pefindoSubject) (pefindoSearchResult, error) {
	var s pefindoSearchResult
	err := e.s.db.QueryRow(ctx, `
		SELECT event_id, pefindo_id, inquiry_id FROM fos.connect_api_log
		WHERE case_id = $1 AND api_name = $2 AND api_status = $3 AND event_id IS NOT NULL
		ORDER BY submitted_at DESC LIMIT 1`, caseID, p.search.logName, apiSuccess).Scan(&s.eventID, &s.pefindoID, &s.inquiryID)
	if err != nil {
		return s, fmt.Errorf("%s: no successful Search to go on from: %w", p.search.logName, err)
	}
	return s, nil
}

func (e *Executor) pefindoGenerate(ctx context.Context, c pefindoAttempt) (attempt, error) {
	log, party := c.log, c.party
	search, err := e.lastPefindoSearch(ctx, c.run.caseID, c.subject)
	if err != nil {
		return attempt{}, err
	}
	language, err := e.s.settings.String(ctx, "api/pefindogeneratereport/language")
	if err != nil {
		return attempt{}, err
	}
	type id struct {
		IDNo      string `json:"id_no"`
		IDPefindo string `json:"id_pefindo"`
		IDType    string `json:"id_type"`
	}
	body, _ := json.Marshal(struct {
		EventID   string `json:"event_id"`
		InquiryID string `json:"inquiry_id"`
		Language  string `json:"language"`
		IDs       []id   `json:"ids"`
	}{search.eventID, search.inquiryID, language, []id{{party.idNo, search.pefindoID, party.idType}}})
	log.method, log.request = http.MethodPost, string(body)
	a, err := e.callPefindo(ctx, c.cfg, log, c.cfg.path, body)
	if err != nil || a == nil {
		return judged(log, a, outcomeFail), err
	}
	if a.code != pefindoFound && a.code != pefindoBigReport {
		return judged(log, a, outcomeFail), nil
	}
	at := judged(log, a, outcomeSuccess)
	at.nextAPI = c.subject.download.logName
	return at, nil
}

// callWithEvent calls a GET API whose path carries the subject's Search
// EventID (Download PDF, Get Report).
func (e *Executor) callWithEvent(ctx context.Context, c pefindoAttempt, api outboundAPI) (*pefindoAnswer, error) {
	search, err := e.lastPefindoSearch(ctx, c.run.caseID, c.subject)
	if err != nil {
		return nil, err
	}
	path := c.cfg.path
	if !c.cfg.simulate {
		if !strings.Contains(path, pefindoEventID) {
			return nil, fmt.Errorf("api/%s/path has no %s", api.settings, pefindoEventID)
		}
		path = strings.Replace(path, pefindoEventID, url.PathEscape(search.eventID), 1)
	}
	return e.callPefindo(ctx, c.cfg, c.log, path, nil)
}

func (e *Executor) pefindoDownload(ctx context.Context, c pefindoAttempt) (attempt, error) {
	log := c.log
	// The wait after Generate Report (FR-PSAPI-016) comes before the first
	// attempt, so a run resumed by another executor waits it too.
	if c.run.attempt == 1 {
		gapMS, err := e.s.settings.Int(ctx, "process/pefindo/generate_to_download_gap_ms")
		if err != nil {
			return attempt{}, err
		}
		if !e.wait(ctx, time.Duration(gapMS)*time.Millisecond) {
			return attempt{}, ctx.Err()
		}
	}
	c.cfg.unbounded = true // no size limit on the PDF (agreed for ticket 13)
	a, err := e.callWithEvent(ctx, c, c.subject.download)
	if err != nil || a == nil {
		return judged(log, a, outcomeFail), err
	}
	if a.mediaType == mediaTypePDF {
		// The log keeps a description, never the file (DB-RULE-086).
		log.response = fmt.Sprintf("[%s, %d bytes]", mediaTypePDF, len(a.body))
		by, err := lastReviewer(ctx, e.s.db, c.run.caseID)
		if err != nil {
			return attempt{}, err
		}
		_, err = e.s.storeGenerated(ctx, c.run.caseID, generatedDoc{
			category: categoryResponse, docType: c.subject.pdfDoc, fileName: "Pefindo Report - " + c.party.customerRef + c.subject.pdfSuffix + ".pdf",
			contentType: mediaTypePDF, data: a.body, createdBy: by,
		})
		if errors.Is(err, ErrStorageFailed) {
			return attempt{}, err // GCS not configured: a system failure (as for SLIK)
		}
		if err != nil {
			log.response += "; the PDF could not be stored: " + err.Error()
			return judged(log, a, outcomeFail), nil
		}
		at := judged(log, a, outcomeSuccess)
		at.nextAPI = c.subject.report.logName
		return at, nil
	}
	if a.code == pefindoNotFound || a.code == pefindoBigReport || allNines(a.code) {
		at := judged(log, a, outcomeSuccess) // no PDF to store
		at.nextAPI = c.subject.report.logName
		return at, nil
	}
	return judged(log, a, outcomeFail), nil // 32, 06, other codes, neither PDF nor JSON
}

// pefindoScore is one scoring element of Get Report.
type pefindoScore struct {
	IDPefindo        json.RawMessage `json:"id_pefindo"`
	Period           string          `json:"period"`
	Score            json.RawMessage `json:"score"`
	RiskGrade        string          `json:"risk_grade"`
	RiskGradeDesc    string          `json:"risk_grade_desc"`
	RiskGradeDescAlt string          `json:"risk_grade_description"`
	RiskDesc         string          `json:"risk_desc"`
	RiskDescAlt      string          `json:"risk_description"`
	POD              json.RawMessage `json:"pod"`
	ReasonCode       []string        `json:"reason_code"`
	ReasonDesc       []string        `json:"reason_desc"`
	ReasonDescAlt    []string        `json:"reason_description"`

	period time.Time
}

func (e *Executor) pefindoReport(ctx context.Context, c pefindoAttempt) (attempt, error) {
	log := c.log
	a, err := e.callWithEvent(ctx, c, c.subject.report)
	if err != nil || a == nil {
		return judged(log, a, outcomeFail), err
	}
	switch {
	case a.code == pefindoNotFound:
		return judged(log, a, outcomeSuccess), nil // no data: nothing stored, no Reject
	case a.code == pefindoFound || a.code == pefindoBigReport || allNines(a.code):
	default:
		return judged(log, a, outcomeFail), nil // 32, 06, other codes
	}
	var resp struct {
		Report  json.RawMessage `json:"report"`
		Scoring []pefindoScore  `json:"scoring"`
	}
	if json.Unmarshal(a.body, &resp) != nil {
		return judged(log, a, outcomeFail), nil
	}
	hasReport := len(resp.Report) > 0 && string(resp.Report) != "null"
	// Code 9s is stored and judged only with a report and scoring; 01 and 36
	// are judged on any scoring and stored only with a report (FR-PSAPI-017
	// rule 2, Data yang disimpan).
	if allNines(a.code) && (!hasReport || len(resp.Scoring) == 0) || !hasReport && len(resp.Scoring) == 0 {
		return judged(log, a, outcomeSuccess), nil
	}
	var latest *pefindoScore
	for i := range resp.Scoring {
		s := &resp.Scoring[i]
		if len(s.Period) < 10 {
			return judged(log, a, outcomeFail), nil // a scoring element without its period is not the documented format
		}
		if s.period, err = time.Parse("2006-01-02", s.Period[:10]); err != nil {
			return judged(log, a, outcomeFail), nil
		}
		if latest == nil || s.period.After(latest.period) {
			latest = s // the latest period, not the last element (DB-RULE-076)
		}
	}
	o := outcomeSuccess
	if latest != nil {
		if pefindoRejectedGrades[strings.ToUpper(strings.ReplaceAll(latest.RiskGrade, " ", ""))] {
			o = outcomeRejected
		}
	}
	at := judged(log, a, o)
	if !hasReport {
		return at, nil
	}
	search, err := e.lastPefindoSearch(ctx, c.run.caseID, c.subject)
	if err != nil {
		return attempt{}, err
	}
	report := reportToStore{subject: c.subject.code, search: search, code: a.code, report: resp.Report, scoring: resp.Scoring, latest: latest, receivedAt: e.s.now()}
	at.apply = func(ctx context.Context, tx pgx.Tx) error { return e.storePefindoReport(ctx, tx, c.run.caseID, report) }
	at.failIfNotApplied = true // DB-RULE-075
	return at, nil
}

// reportToStore is a Get Report result to store (§9.22, §9.23, §9.40).
type reportToStore struct {
	subject    string
	search     pefindoSearchResult
	code       string
	report     json.RawMessage
	scoring    []pefindoScore
	latest     *pefindoScore
	receivedAt time.Time
}

// storePefindoReport replaces the subject's report and its scoring in tx,
// before the Reject is decided (DB-RULE-074, DB-RULE-075).
func (e *Executor) storePefindoReport(ctx context.Context, tx pgx.Tx, caseID string, sr reportToStore) error {
	for _, q := range []string{
		`DELETE FROM fos.pefindo_scoring_reason WHERE pefindo_scoring_id IN (SELECT s.pefindo_scoring_id FROM fos.pefindo_scoring s
			JOIN fos.pefindo_report r USING (pefindo_report_id) WHERE r.case_id = $1 AND r.subject_type = $2)`,
		`DELETE FROM fos.pefindo_scoring WHERE pefindo_report_id IN (SELECT pefindo_report_id FROM fos.pefindo_report WHERE case_id = $1 AND subject_type = $2)`,
		`DELETE FROM fos.pefindo_report WHERE case_id = $1 AND subject_type = $2`,
	} {
		if _, err := tx.Exec(ctx, q, caseID, sr.subject); err != nil {
			return err
		}
	}
	var report struct {
		Header struct {
			Username    json.RawMessage `json:"username"`
			IDReport    json.RawMessage `json:"id_report"`
			InquiryDate json.RawMessage `json:"inquiry_date"`
		} `json:"header"`
		Management         json.RawMessage `json:"pengurus"`
		OtherData          json.RawMessage `json:"other_data"`
		OtherDataSource    json.RawMessage `json:"otherDataSource"`
		DebtorHistory      json.RawMessage `json:"summary_riwayat_debitur"`
		CurrentContract    json.RawMessage `json:"CurrentContract"`
		InquiriesAnalysis  json.RawMessage `json:"InquiriesAnalysis"`
		PastDueInformation json.RawMessage `json:"PastDueInformation"`
		Repayment          json.RawMessage `json:"RepaymentInformation"`
	}
	_ = json.Unmarshal(sr.report, &report) // a report not shaped as expected keeps only report_json
	var latestPeriod *time.Time
	var latestGrade *string
	var latestScore *int32
	if sr.latest != nil {
		latestPeriod, latestGrade, latestScore = &sr.latest.period, nullText(sr.latest.RiskGrade), jsonInt(sr.latest.Score)
	}
	var id int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO fos.pefindo_report (case_id, subject_type, event_id, inquiry_id, id_pefindo, report_code, report_json,
			latest_period, latest_risk_grade, latest_score, header_username, header_id_report, header_inquiry_date,
			management_json, other_data_json, other_data_source_json, debtor_history_summary_json, current_contract_json,
			inquiries_analysis_json, past_due_information_json, repayment_information_json, received_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $22, $22)
		RETURNING pefindo_report_id`,
		caseID, sr.subject, sr.search.eventID, sr.search.inquiryID, sr.search.pefindoID, sr.code, jsonOrNil(sr.report),
		latestPeriod, latestGrade, latestScore, nullText(jsonText(report.Header.Username)), nullText(jsonText(report.Header.IDReport)),
		reportTime(jsonText(report.Header.InquiryDate)), jsonOrNil(report.Management), jsonOrNil(report.OtherData),
		jsonOrNil(report.OtherDataSource), jsonOrNil(report.DebtorHistory), jsonOrNil(report.CurrentContract),
		jsonOrNil(report.InquiriesAnalysis), jsonOrNil(report.PastDueInformation), jsonOrNil(report.Repayment), sr.receivedAt).Scan(&id); err != nil {
		return err
	}
	for _, s := range sr.scoring {
		var scoringID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO fos.pefindo_scoring (pefindo_report_id, id_pefindo, period, score, risk_grade, risk_grade_description,
				risk_description, probability_of_default)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8::numeric) RETURNING pefindo_scoring_id`,
			id, nullText(jsonText(s.IDPefindo)), s.period, jsonInt(s.Score), nullText(s.RiskGrade),
			nullText(coalesce(s.RiskGradeDesc, s.RiskGradeDescAlt)), nullText(coalesce(s.RiskDesc, s.RiskDescAlt)), jsonDecimal(s.POD)).Scan(&scoringID); err != nil {
			return err
		}
		descs := s.ReasonDesc
		if descs == nil {
			descs = s.ReasonDescAlt
		}
		// Pairs by array position; the shorter side is NULL (DB-RULE-115).
		for i := 0; i < max(len(s.ReasonCode), len(descs)); i++ {
			var code, desc *string
			if i < len(s.ReasonCode) {
				code = &s.ReasonCode[i]
			}
			if i < len(descs) {
				desc = &descs[i]
			}
			if _, err := tx.Exec(ctx, `INSERT INTO fos.pefindo_scoring_reason (pefindo_scoring_id, sequence_number, reason_code, reason_description)
				VALUES ($1, $2, $3, $4)`, scoringID, i+1, code, desc); err != nil {
				return err
			}
		}
	}
	return nil
}

// coalesce is a, or b when a is empty.
func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func nullText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// jsonOrNil is a JSON value to store as JSONB, or NULL for null or absent.
func jsonOrNil(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return string(raw)
}

// jsonInt reads an INTEGER sent as a number or a string; NULL when it is
// not a whole number within INTEGER (agreed for ticket 13).
func jsonInt(raw json.RawMessage) *int32 {
	t := jsonText(raw)
	n, err := strconv.ParseInt(t, 10, 32)
	if err != nil {
		// A whole number written with a fraction, e.g. 600.0.
		f, ferr := strconv.ParseFloat(t, 64)
		if ferr != nil || f != math.Trunc(f) || f < math.MinInt32 || f > math.MaxInt32 {
			return nil
		}
		n = int64(f)
	}
	v := int32(n)
	return &v
}

// jsonDecimal reads a decimal sent as a number or a string; NULL otherwise.
func jsonDecimal(raw json.RawMessage) *string {
	t := jsonText(raw)
	if _, err := strconv.ParseFloat(t, 64); err != nil {
		return nil
	}
	return &t
}

// reportTime reads a report timestamp: RFC3339, or yyyy-MM-dd HH:mm:ss in
// Asia/Jakarta; stored UTC, NULL when unreadable (agreed for ticket 13).
func reportTime(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		if t, err = time.ParseInLocation("2006-01-02 15:04:05", s, jakarta); err != nil {
			return nil
		}
	}
	t = t.UTC()
	return &t
}
