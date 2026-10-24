package cases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"fos/internal/auth"
)

// While a run waits for its SLIK results the Branch Officer has the
// "SLIKWQFlowAct" assignment (FR-PSAPI-013, UI-023), where "Check SLIK
// Status" asks SFS for each reference code's status (SLIKInquiry). It never
// changes SLIK data, the run or case_status (FR-PSAPI-013 rule 7).

// AssignmentWaitingSLIK is the waiting assignment's name (Gambar 44; agreed
// for TBD-048a).
const AssignmentWaitingSLIK = "SLIKWQFlowAct"

var slikInquiryAPI = outboundAPI{logName: "SLIKInquiry", settings: "slikinquiry", base: "slik", secret: "slik", keyHeader: "x-api-key"}

// slikInquiryCode is the reference code in api/slikinquiry/path.
const slikInquiryCode = "{KodeReff}"

// openWaitingSLIK gives the case's Branch Officer the waiting assignment, in
// the transaction that puts the run in WAITING_SLIK (FR-PSAPI-011 Process 1).
func openWaitingSLIK(ctx context.Context, tx pgx.Tx, caseID string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
		SELECT case_id, $2, $3, branch_officer, $4, $5 FROM fos.work WHERE case_id = $1`,
		caseID, AssignmentWaitingSLIK, string(Worklist), string(AssignmentNew), now)
	return err
}

// completeWaitingSLIK completes the case's waiting assignment, if open: the
// results are in, rejected, timed out or the case stopped (DEC-089).
func completeWaitingSLIK(ctx context.Context, tx pgx.Tx, caseID string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE fos.assignments SET assignment_status = $3, completed_at = $4
		WHERE case_id = $1 AND assignment_name = $2 AND assignment_status <> $3`,
		caseID, AssignmentWaitingSLIK, string(AssignmentCompleted), now)
	return err
}

// WaitingSLIKPage is UI-023: the whole minutes left before the SLA
// deadline when the page opens, never below 0 (agreed for TBD-048a), and
// whether "Check SLIK Status" may be pressed.
type WaitingSLIKPage struct {
	Editable   bool
	DueMinutes int
}

func (s *Service) waitingSLIKPage(ctx context.Context, caseID string, editable bool) (WaitingSLIKPage, error) {
	page := WaitingSLIKPage{Editable: editable}
	var deadline *time.Time
	err := s.db.QueryRow(ctx, `SELECT wait_deadline_at FROM fos.prescreening_runs WHERE run_id = `+latestRun, caseID).Scan(&deadline)
	if err != nil && !isNotFound(err) {
		return page, err
	}
	if deadline != nil {
		page.DueMinutes = max(0, int(deadline.Sub(s.now())/time.Minute))
	}
	return page, nil
}

// SLIKStatus is one subject's answer to Check SLIK Status: the message
// shown in its Message panel.
type SLIKStatus struct {
	Subject, Message string
}

// CheckSLIKStatus asks SLIKInquiry for the Customer's reference code, then
// the Spouse's when the case has one, one after the other; one failing does
// not stop the other (FR-PSAPI-013 rule 1). Only the holder of the open
// waiting assignment may ask.
func (s *Service) CheckSLIKStatus(ctx context.Context, sess auth.Session, caseID string, assignmentID int64) ([]SLIKStatus, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	a, err := ownOpenAssignment(ctx, tx, sess, caseID, assignmentID)
	_ = tx.Rollback(ctx) // the calls are made without holding the assignment
	if err != nil {
		return nil, err
	}
	if a.name != AssignmentWaitingSLIK {
		return nil, ErrUnsupported
	}
	type report struct{ subject, code string }
	reports, err := collect(ctx, s, func(r pgx.Rows) (report, error) {
		var rp report
		return rp, r.Scan(&rp.subject, &rp.code)
	}, `SELECT subject_type, reference_code FROM fos.slik_report WHERE case_id = $1
		ORDER BY CASE subject_type WHEN $2 THEN 0 ELSE 1 END`, caseID, subjectCustomer)
	if err != nil {
		return nil, err
	}
	out := []SLIKStatus{}
	for _, rp := range reports {
		message, err := s.runs.slikInquiry(ctx, caseID, rp.code)
		if err != nil {
			return nil, err
		}
		out = append(out, SLIKStatus{rp.subject, message})
	}
	return out, nil
}

// slikInquiry asks the status of one reference code, without retry
// (FR-PSAPI-013 rule 3), logs it (Success only on code 1100) and returns the
// message to show: the answer's message, else the logged Response. An error
// is a system failure (a missing setting).
func (e *Executor) slikInquiry(ctx context.Context, caseID, code string) (string, error) {
	cfg, err := e.apiConfig(ctx, slikInquiryAPI)
	if err != nil {
		return "", err
	}
	if !cfg.simulate && !strings.Contains(cfg.path, slikInquiryCode) {
		return "", fmt.Errorf("api/slikinquiry/path has no %s", slikInquiryCode)
	}
	log := &apiLog{name: slikInquiryAPI.logName, submittedAt: e.s.now(), method: http.MethodGet, attempt: 1, request: code}
	path := strings.Replace(cfg.path, slikInquiryCode, url.PathEscape(code), 1)
	if _, err := e.send(ctx, cfg, log, path, nil, map[string]string{"Accept": "application/json"}, evaluateSLIKInquiry); err != nil {
		return "", err
	}
	if cfg.simulate {
		log.status = evaluateSLIKInquiry(log.response).logStatus() // a simulated answer is judged too
	}
	if err := e.insertLog(ctx, caseID, log); err != nil {
		// The answer still shows; the missing row is reported (as for PBSResponse).
		slog.Error("API_LOG_WRITE_FAILED", "api_name", log.name, "case_id", caseID, "reference_code", code, "error", err)
	}
	var resp struct {
		Message *string `json:"message"`
	}
	if json.Unmarshal([]byte(log.response), &resp) == nil && resp.Message != nil {
		return *resp.Message, nil
	}
	return log.response, nil
}

// evaluateSLIKInquiry passes only code "1100"; nothing rejects (FR-PSAPI-013
// rule 2).
func evaluateSLIKInquiry(body string) outcome {
	if evaluateSLIKRequest(body) == outcomeSuccess {
		return outcomeSuccess
	}
	return outcomeFail
}

// insertLog appends one log row in its own transaction.
func (e *Executor) insertLog(ctx context.Context, caseID string, log *apiLog) error {
	tx, err := e.s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := log.insert(ctx, tx, caseID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
