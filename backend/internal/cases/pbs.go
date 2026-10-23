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
	apiPBSResponse        = "SLIKPBSResponse"
	runStoppedSLIKTimeout = "STOPPED_SLIK_TIMEOUT"
	apiTypeServiceRest    = "Service Rest"
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

// UnexpectedReason is the Code 1 message (FR-PSAPI-009 Response).
func UnexpectedReason(reason string) string { return "an unexpected error occurred. Reason: " + reason }

// ReceiveSLIK takes one SLIK result from the RPA account rpaUser. The
// result is stored and judged under row locks on the reference code's
// slik_report and the case's run, so two calls for one code are never
// processed twice and a run never misses the last result. What goes on
// afterwards (the resumed run) starts when the caller calls resume, after
// the answer is sent (FR-PSAPI-009 step 6).
func (s *Service) ReceiveSLIK(ctx context.Context, rpaUser, kode string, zipData []byte) (res PBSResult, resume func()) {
	res, runID, err := s.receiveSLIK(ctx, rpaUser, kode, zipData)
	if err != nil {
		slog.Error("SLIK result not processed", "case_id", res.CaseID, "error", err)
		res.Code, res.Message, res.LogStatus, res.Stored = "1", UnexpectedReason("the SLIK result could not be processed"), apiFail, nil
		runID = 0
	}
	return res, func() {
		if runID != 0 {
			s.runs.launch(runID)
		}
	}
}

// pendingSLIK reports whether a SLIK result of the case is still awaited.
func pendingSLIK(ctx context.Context, q rowQuerier, caseID string) (bool, error) {
	var pending bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM fos.slik_report WHERE case_id = $1 AND status <> $2)`, caseID, slikReceived).Scan(&pending)
	return pending, err
}

func (s *Service) receiveSLIK(ctx context.Context, rpaUser, kode string, zipData []byte) (PBSResult, int64, error) {
	var caseID, subjectCode, status string
	err := s.db.QueryRow(ctx, `SELECT case_id, subject_type, status FROM fos.slik_report WHERE reference_code = $1`, kode).
		Scan(&caseID, &subjectCode, &status)
	if isNotFound(err) {
		return PBSResult{Code: "2", Message: "Case not found", LogStatus: apiFail}, 0, nil
	}
	if err != nil {
		return PBSResult{}, 0, err
	}
	res := PBSResult{CaseID: caseID}
	refused := func(reason string) (PBSResult, int64, error) {
		res.Code, res.Message, res.LogStatus = "1", UnexpectedReason(reason), apiFail
		return res, 0, nil
	}
	if r, refusedNow := s.refuseSLIK(ctx, s.db, caseID, kode, status); refusedNow {
		return refused(r)
	}
	file, err := pickSLIKFile(zipData)
	if err != nil {
		// No usable file: the report keeps waiting and the RPA may resend.
		return refused(err.Error())
	}

	// The ZIP goes to GCS before the transaction, outside the locks
	// (DB-RULE-025); it is removed again unless the transaction commits.
	subject := slikSubjects[subjectCode]
	doc := generatedDoc{category: categoryResponse, docType: subject.zipDoc, fileName: subject.zipName(kode),
		contentType: "application/zip", data: zipData, createdBy: rpaUser}
	sd, err := s.putGenerated(ctx, caseID, doc)
	if err != nil {
		return res, 0, fmt.Errorf("storing the SLIK ZIP: %w", err)
	}
	committed := false
	defer func() { s.settleGenerated(context.WithoutCancel(ctx), sd, committed) }()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return res, 0, err
	}
	defer tx.Rollback(ctx)
	var reportID int64
	if err := tx.QueryRow(ctx, `SELECT slik_report_id, status FROM fos.slik_report WHERE reference_code = $1 FOR UPDATE`, kode).
		Scan(&reportID, &status); err != nil {
		return res, 0, err
	}
	// The run is locked too: the executor's advance after SLIK Pasangan and
	// this call then see each other's results (no run left waiting).
	var runID int64
	var runState string
	err = tx.QueryRow(ctx, `SELECT run_id, state FROM fos.prescreening_runs WHERE run_id = `+latestRun+` FOR UPDATE`, caseID).Scan(&runID, &runState)
	if err != nil && !isNotFound(err) {
		return res, 0, err
	}
	// A call that waited for the lock finds the result already in.
	if r, refusedNow := s.refuseSLIK(ctx, tx, caseID, kode, status); refusedNow {
		return refused(r)
	}

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
	if subjectCode == subjectCustomer {
		if _, err := tx.Exec(ctx, `UPDATE fos.work SET slik_worst_collect = $2 WHERE case_id = $1`, caseID, worst); err != nil {
			return res, 0, err
		}
	}

	res.Code, res.LogStatus = "0", apiSuccess
	res.Message = "Case " + caseID + " successfully resume to Pipeline Data Entry"
	resumeID := int64(0)
	if runState != runRunning && runState != runWaitingSLIK {
		// The case no longer waits (rejected, failed, ...): the result is kept
		// but changes nothing (agreed; SoT silent).
		res.Message = "SLIK result for " + kode + " received; case " + caseID + " is no longer waiting for SLIK"
	} else {
		rejects, err := slikRejects(ctx, tx, reportID)
		if err != nil {
			return res, 0, err
		}
		pending, err := pendingSLIK(ctx, tx, caseID)
		if err != nil {
			return res, 0, err
		}
		switch {
		case rejects:
			// FR-PSAPI-010 rule 4: at once, even with one result in.
			if err := s.runs.rejectCase(ctx, tx, caseID, rejectionReason(subject.api.logName, slikCriterion)); err != nil {
				return res, 0, err
			}
			if _, err := tx.Exec(ctx, `UPDATE fos.prescreening_runs SET state = $2, updated_at = $3 WHERE run_id = $1`, runID, runStoppedRejected, now); err != nil {
				return res, 0, err
			}
			res.LogStatus = apiRejected
		case pending:
			res.Message = "SLIK result for " + kode + " received; waiting for the other result" // agreed for TBD-044
		case runState == runWaitingSLIK:
			// Every result is in: the waiting run goes on (Pefindo, ...).
			if _, err := tx.Exec(ctx, `
				UPDATE fos.prescreening_runs SET state = $2, owner_id = $3, heartbeat_at = $4, wait_deadline_at = NULL, updated_at = $4
				WHERE run_id = $1`, runID, runRunning, s.runs.owner, now); err != nil {
				return res, 0, err
			}
			resumeID = runID
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return res, 0, err
	}
	committed = true
	res.Stored = &sd.fileID
	return res, resumeID, nil
}

// refuseSLIK is the reason a result for the reference code is refused with
// Code 1, if it is: the case's SLIK wait ran out (FR-PSAPI-011 rule 3) or
// its SLIK was bypassed (as after the timeout, agreed for ticket 12), the
// result is in already (FR-PSAPI-009 step 7), or it was never requested.
// Texts are interim (TBD-044, agreed).
func (s *Service) refuseSLIK(ctx context.Context, q rowQuerier, caseID, kode, status string) (string, bool) {
	var timedOut bool
	if err := q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM fos.connect_api_log WHERE case_id = $1 AND api_name = $2 AND response = $3)
		    OR EXISTS (SELECT 1 FROM fos.prescreening_runs WHERE case_id = $1 AND state IN ($4, $5))`,
		caseID, apiPBSResponse, timeoutResponse, runStoppedSLIKTimeout, runStoppedSLIKBypass).Scan(&timedOut); err != nil {
		return "the SLIK result could not be processed", true
	}
	switch {
	case timedOut:
		return "the SLIK waiting time for case " + caseID + " has passed", true
	case status == slikReceived:
		return "SLIK result for " + kode + " has already been received", true
	case status == slikPendingRequest:
		return "SLIK for " + kode + " has not been requested", true
	}
	return "", false
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
