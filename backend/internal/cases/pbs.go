package cases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// PBSResponse (FR-PSAPI-009) is the service the RPA calls with one SLIK
// result: a kode reff and its ZIP.

const (
	apiPBSResponse     = "SLIKPBSResponse"
	apiTypeServiceRest = "Service Rest"
	// timeoutResponse marks a case whose SLIK wait ran out (FR-PSAPI-011).
	timeoutResponse = "Timeout SLIK API No Response"

	// slikCriterion is the SLIK Check Rejection criterion in the Rejected
	// List reason (FR-PSAPI-010 rule 4; agreed).
	slikCriterion = "Kolektibilitas 2 dengan maksimum tunggakan hari 15 (sebanyak 6x) ATAU Kolektabilitas 2 dengan maksimum tunggakan hari di atas 15 (sebanyak 1x)"
)

// PBSResult is the outcome of one call: the answer's Code and message, and
// what its Connect API Log row records.
type PBSResult struct {
	Code, Message string
	CaseID        string // empty when the kode reff is unknown
	LogStatus     string
	// Stored is the ZIP result's Case Document, when one was stored.
	Stored *uuid.UUID
}

// pbsAnswer is the body of an answer (FR-PSAPI-009 Response).
type pbsAnswer struct {
	Error     string `json:"Error"`
	Message   string `json:"message"`
	Code      string `json:"Code"`
	Timestamp string `json:"Timestamp"`
}

// Answer is the JSON the RPA receives, timestamped in Asia/Jakarta.
func (r PBSResult) Answer(now time.Time) []byte {
	errFlag := "true"
	if r.Code == "0" {
		errFlag = "false"
	}
	b, _ := json.Marshal(pbsAnswer{errFlag, r.Message, r.Code, now.In(jakarta).Format("02-01-2006 15:04")})
	return b
}

func unexpected(reason string) string { return "an unexpected error occurred. Reason: " + reason }

// ReceiveSLIK takes one SLIK result from the RPA account rpaUser. Steps 3
// to 5 run under a row lock on the kode reff's slik_report, so two calls for
// the same code are never processed twice (PROPOSED); the run resumes, or
// the case is rejected, in the same transaction.
func (s *Service) ReceiveSLIK(ctx context.Context, rpaUser, kode string, zipData []byte) PBSResult {
	res, runID, err := s.receiveSLIK(ctx, rpaUser, kode, zipData)
	if err != nil {
		slog.Error("SLIK result not processed", "kode", kode, "case_id", res.CaseID, "error", err)
		res.Code, res.Message, res.LogStatus, res.Stored = "1", unexpected("the SLIK result could not be processed"), apiFail, nil
	}
	if runID != 0 {
		s.runs.launch(runID)
	}
	return res
}

func (s *Service) receiveSLIK(ctx context.Context, rpaUser, kode string, zipData []byte) (PBSResult, int64, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return PBSResult{}, 0, err
	}
	defer tx.Rollback(ctx)

	var (
		reportID                int64
		caseID, subject, status string
	)
	err = tx.QueryRow(ctx, `SELECT slik_report_id, case_id, subject_type, status FROM fos.slik_report WHERE reference_code = $1 FOR UPDATE`, kode).
		Scan(&reportID, &caseID, &subject, &status)
	if isNotFound(err) {
		return PBSResult{Code: "2", Message: "Case not found", LogStatus: apiFail}, 0, nil
	}
	if err != nil {
		return PBSResult{}, 0, err
	}
	res := PBSResult{CaseID: caseID}
	var timedOut bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM fos.connect_api_log WHERE case_id = $1 AND api_name = $2 AND response = $3)`,
		caseID, apiPBSResponse, timeoutResponse).Scan(&timedOut); err != nil {
		return res, 0, err
	}
	switch {
	case timedOut: // FR-PSAPI-011 rule 3; reason text TBD-044 (ticket 12)
		res.Code, res.Message, res.LogStatus = "1", unexpected("the SLIK waiting time for case "+caseID+" has passed"), apiFail
		return res, 0, nil
	case status == slikReceived: // FR-PSAPI-009 step 7 (agreed text)
		res.Code, res.Message, res.LogStatus = "1", unexpected("SLIK result for "+kode+" has already been received"), apiFail
		return res, 0, nil
	}

	file, err := pickSLIKFile(zipData)
	if err != nil {
		// No usable file: the report keeps waiting and the RPA may resend.
		res.Code, res.Message, res.LogStatus = "1", unexpected(err.Error()), apiFail
		return res, 0, nil
	}

	docType, zipName := docZIPCustomer, kode+".zip"
	if subject == subjectSpouse {
		docType, zipName = docZIPSpouse, kode+" - Spouse.zip"
	}
	doc := generatedDoc{category: categoryResponse, docType: docType, fileName: zipName, contentType: "application/zip", data: zipData, createdBy: rpaUser}
	sd, err := s.putGenerated(ctx, caseID, doc)
	if err != nil {
		return res, 0, fmt.Errorf("storing the SLIK ZIP: %w", err)
	}
	committed := false
	defer func() { s.settleGenerated(ctx, sd, committed) }()
	if err := s.slotGenerated(ctx, tx, caseID, doc, &sd); err != nil {
		return res, 0, err
	}

	now := s.now()
	worst, err := storeSLIK(ctx, tx, reportID, file, now)
	if err != nil {
		return res, 0, err
	}
	// DB-RULE-058: RECEIVED with the data in one transaction; the PDF comes
	// with ticket 34 (agreed).
	if _, err := tx.Exec(ctx, `UPDATE fos.slik_report SET status = $2, received_at = $3, zip_document_id = $4, updated_at = $3 WHERE slik_report_id = $1`,
		reportID, slikReceived, now, sd.fileID); err != nil {
		return res, 0, err
	}
	if subject == subjectCustomer {
		if _, err := tx.Exec(ctx, `UPDATE fos.work SET slik_worst_collect = $2 WHERE case_id = $1`, caseID, worst); err != nil {
			return res, 0, err
		}
	}

	rejects, err := slikRejects(ctx, tx, reportID)
	if err != nil {
		return res, 0, err
	}
	var runID int64
	res.Code, res.LogStatus = "0", apiSuccess
	res.Message = "Case " + caseID + " successfully resume to Pipeline Data Entry"
	switch {
	case rejects:
		// FR-PSAPI-010 rule 4: at once, even with one result in.
		api := slikAPI.logName
		if subject == subjectSpouse {
			api = slikSpouseAPI.logName
		}
		if err := s.runs.rejectCase(ctx, tx, caseID, rejectionReason(api, slikCriterion)); err != nil {
			return res, 0, err
		}
		if _, err := tx.Exec(ctx, `UPDATE fos.prescreening_runs SET state = $2, updated_at = $3 WHERE case_id = $1 AND state IN ($4, $5)`,
			caseID, runStoppedRejected, now, runRunning, runWaitingSLIK); err != nil {
			return res, 0, err
		}
		res.LogStatus = apiRejected
	default:
		var pending bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM fos.slik_report WHERE case_id = $1 AND status <> $2)`, caseID, slikReceived).Scan(&pending); err != nil {
			return res, 0, err
		}
		if pending {
			res.Message = "SLIK result for " + kode + " received; waiting for the other result" // agreed for TBD-044
			break
		}
		// Every result is in: the waiting run goes on (Pefindo, ...).
		err := tx.QueryRow(ctx, `
			UPDATE fos.prescreening_runs SET state = $2, owner_id = $3, heartbeat_at = $4, wait_deadline_at = NULL, updated_at = $4
			WHERE case_id = $1 AND state = $5 RETURNING run_id`, caseID, runRunning, s.runs.owner, now, runWaitingSLIK).Scan(&runID)
		if err != nil && !isNotFound(err) {
			return res, 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return res, 0, err
	}
	committed = true
	res.Stored = &sd.fileID
	return res, runID, nil
}

// InboundLog is one Connect API Log row of a call into FOS (FR-PSAPI-004
// rule 8, DB-RULE-067).
type InboundLog struct {
	CaseID, APIName, Status        string
	Request, Response              string
	Method, URL                    string
	HTTPStatus                     int
	RequestHeaders, ResponseHeader http.Header
	ClientIP, AuthUsername         string
	SubmittedAt                    time.Time
	Duration                       time.Duration
}

// LogInbound appends the row, retrying three times a second apart; a row
// that still cannot be written goes to the application log with the tag
// API_LOG_WRITE_FAILED (FR-PSAPI-009 Log API, DEC-078). Secret headers are
// masked (DB-RULE-066).
func (s *Service) LogInbound(ctx context.Context, l InboundLog) {
	secret, err := s.maskedHeaderNames(ctx)
	if err != nil {
		secret = nil
	}
	secret = append(secret, "Authorization")
	toJSON := func(h http.Header) []byte {
		if h == nil {
			return nil
		}
		b, _ := json.Marshal(masked(h, secret))
		return b
	}
	nullable := func(v string) *string {
		if v == "" {
			return nil
		}
		return &v
	}
	insert := func() error {
		_, err := s.db.Exec(context.WithoutCancel(ctx), `
			INSERT INTO fos.connect_api_log (log_id, case_id, api_type, api_name, submitted_at, api_status, request, response,
				http_method, request_url, http_status_code, request_headers, response_headers, client_ip, duration_milliseconds, auth_username)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
			uuid.New(), nullable(l.CaseID), apiTypeServiceRest, nullable(l.APIName), l.SubmittedAt, l.Status, nullable(l.Request), l.Response,
			l.Method, l.URL, l.HTTPStatus, toJSON(l.RequestHeaders), toJSON(l.ResponseHeader), nullable(l.ClientIP),
			int(l.Duration/time.Millisecond), nullable(l.AuthUsername))
		return err
	}
	err = insert()
	for i := 0; err != nil && i < 3; i++ {
		time.Sleep(time.Second)
		err = insert()
	}
	if err != nil {
		slog.Error("API_LOG_WRITE_FAILED", "api_name", l.APIName, "case_id", l.CaseID, "status", l.Status,
			"request", l.Request, "response", l.Response, "error", err)
	}
}
