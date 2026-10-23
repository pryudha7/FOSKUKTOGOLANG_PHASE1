package cases

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Connect API Log statuses and type (FR-PSAPI-004, §9.14).
const (
	apiSuccess  = "Success"
	apiRejected = "Rejected"
	apiFail     = "Fail"

	apiTypeConnectRest = "ConnectRest"
)

// apiLog is one Connect API Log row for an outbound attempt.
type apiLog struct {
	name, status      string
	submittedAt       time.Time
	request, response string
	method, url       string
	httpStatus        *int
	requestHeaders    http.Header
	responseHeaders   http.Header
	duration          time.Duration
	attempt           int
	// Pefindo Search (FR-PSAPI-014, §9.14).
	eventID, pefindoID, inquiryID string
}

// masked replaces the values of secret headers with "***" (DB-RULE-066);
// names match case-insensitively.
func masked(h http.Header, secret []string) map[string][]string {
	if h == nil {
		return nil
	}
	out := map[string][]string{}
	for k, v := range h {
		hide := false
		for _, s := range secret {
			if strings.EqualFold(k, s) {
				hide = true
			}
		}
		if hide {
			stars := make([]string, len(v))
			for i := range stars {
				stars[i] = "***"
			}
			v = stars
		}
		out[k] = v
	}
	return out
}

// maskedHeaderNames reads api/log/masked_headers (comma-separated).
func (s *Service) maskedHeaderNames(ctx context.Context) ([]string, error) {
	v, err := s.settings.String(ctx, "api/log/masked_headers")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, n := range strings.Split(v, ",") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out, nil
}

// insert appends the row (DB-RULE-041); caseID "" is a row without a case
// (PefindoToken). Headers must already be masked.
func (l *apiLog) insert(ctx context.Context, tx pgx.Tx, caseID string) error {
	toJSON := func(h http.Header) []byte {
		if h == nil {
			return nil
		}
		b, _ := json.Marshal(map[string][]string(h))
		return b
	}
	empty := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO fos.connect_api_log (log_id, case_id, api_type, api_name, submitted_at, api_status, request, response,
			http_method, request_url, http_status_code, request_headers, response_headers, duration_milliseconds, attempt_number,
			event_id, pefindo_id, inquiry_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
		uuid.New(), empty(caseID), apiTypeConnectRest, l.name, l.submittedAt, l.status, empty(l.request), l.response,
		empty(l.method), empty(l.url), l.httpStatus, toJSON(l.requestHeaders), toJSON(l.responseHeaders),
		int(l.duration/time.Millisecond), l.attempt, empty(l.eventID), empty(l.pefindoID), empty(l.inquiryID))
	return err
}
