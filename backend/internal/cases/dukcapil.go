package cases

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Dukcapil check (FR-PSAPI-003): the Perorangan customer's Data Pribadi,
// never the spouse or a Badan Usaha.

const apiDukcapil = "Dukcapil"

// dukcapilRequest is the body, in the SoT's field order.
type dukcapilRequest struct {
	NIK          string `json:"nik"`
	TanggalLahir string `json:"tanggalLahir"` // dd-MM-yyyy
	NamaLengkap  string `json:"namaLengkap"`
}

func (e *Executor) dukcapilCheck() check {
	return check{
		try: e.tryDukcapil,
		retry: func(ctx context.Context) (int, time.Duration, error) {
			max, err := e.s.settings.Int(ctx, "api/dukcapil/retry_max")
			if err != nil {
				return 0, 0, err
			}
			gap, err := e.s.settings.Int(ctx, "api/dukcapil/retry_gap_ms")
			return int(max), time.Duration(gap) * time.Millisecond, err
		},
	}
}

func (e *Executor) tryDukcapil(ctx context.Context, caseID string, attemptNumber int) (attempt, error) {
	s := e.s
	var (
		customerType *string
		nik, name    *string
		birthDate    *time.Time
	)
	if err := s.db.QueryRow(ctx, `SELECT customer_type FROM fos.work WHERE case_id = $1`, caseID).Scan(&customerType); err != nil {
		return attempt{}, err
	}
	if value(customerType) != customerTypeIndividual {
		return attempt{outcome: outcomeSkip}, nil
	}
	// FR-PSAPI-002 rule 6: the request is built from the case data each time.
	if err := s.db.QueryRow(ctx, `
		SELECT id_number, full_name, birth_date FROM fos.customer WHERE case_id = $1 AND customer_type = $2`,
		caseID, partyCustomer).Scan(&nik, &name, &birthDate); err != nil {
		return attempt{}, fmt.Errorf("Dukcapil request data: %w", err)
	}
	if birthDate == nil {
		return attempt{}, errors.New("Dukcapil request data: no birth date")
	}
	body, _ := json.Marshal(dukcapilRequest{value(nik), birthDate.Format("02-01-2006"), value(name)})
	secret, err := s.maskedHeaderNames(ctx)
	if err != nil {
		return attempt{}, err
	}
	log := &apiLog{name: apiDukcapil, submittedAt: s.now(), request: string(body), method: http.MethodPost, attempt: attemptNumber}

	simulate, err := s.settings.Bool(ctx, "api/dukcapil/simulate")
	if err != nil {
		return attempt{}, err
	}
	if simulate {
		// No call; the configured response is logged as Success (FR-PSAPI-003).
		resp, err := s.settings.String(ctx, "api/dukcapil/simulate_response")
		if err != nil {
			return attempt{}, err
		}
		log.status, log.response, log.method = apiSuccess, resp, ""
		return attempt{outcomeSuccess, log}, nil
	}

	base, err := s.settings.String(ctx, "api/dukcapil/baseurl")
	if err != nil {
		return attempt{}, err
	}
	path, err := s.settings.String(ctx, "api/dukcapil/path")
	if err != nil {
		return attempt{}, err
	}
	timeoutMS, err := s.settings.Int(ctx, "api/dukcapil/timeout_ms")
	if err != nil {
		return attempt{}, err
	}
	keyRef, err := s.settings.String(ctx, "secret/dukcapil/apikey_ref")
	if err != nil {
		return attempt{}, err
	}
	key := e.getenv(keyRef)
	if key == "" {
		return attempt{}, fmt.Errorf("Dukcapil API key: environment variable %s is empty (DEC-060)", keyRef)
	}

	log.url = strings.TrimSuffix(base, "/") + path
	timeout := time.Duration(timeoutMS) * time.Millisecond
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, log.url, bytes.NewReader(body))
	if err != nil {
		return attempt{}, fmt.Errorf("Dukcapil URL: %w", err)
	}
	req.Header.Set("x-Gateway-APIKey", key)
	req.Header.Set("Content-Type", "application/json")
	log.requestHeaders = masked(req.Header, secret)

	start := time.Now()
	resp, err := e.http.Do(req)
	if err == nil {
		var raw []byte
		raw, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		code := resp.StatusCode
		log.httpStatus = &code
		log.responseHeaders = masked(resp.Header, secret)
		log.response = string(raw)
		if err == nil && len(bytes.TrimSpace(raw)) == 0 {
			log.response = noBodyMessage(code)
		}
	}
	log.duration = time.Since(start)
	if ctx.Err() != nil {
		return attempt{}, ctx.Err()
	}
	if err != nil {
		log.response = callError(err, req.URL, timeout)
		log.status = apiFail
		return attempt{outcomeFail, log}, nil
	}
	o := evaluateDukcapil(log.response)
	log.status = map[outcome]string{outcomeSuccess: apiSuccess, outcomeRejected: apiRejected, outcomeFail: apiFail}[o]
	return attempt{o, log}, nil
}

// evaluateDukcapil is FR-PSAPI-003 "Evaluasi hasil", judged on the body only.
func evaluateDukcapil(body string) outcome {
	var fields map[string]any
	if json.Unmarshal([]byte(body), &fields) != nil {
		return outcomeFail
	}
	if code, ok := fields["RESPONSE_CODE"]; ok {
		if code == "13" { // data not found: passes (PROPOSED)
			return outcomeSuccess
		}
		return outcomeFail
	}
	for _, k := range []string{"NIK", "NAMA_LGKP"} {
		if _, ok := fields[k].(string); !ok {
			return outcomeFail
		}
	}
	birth, _ := fields["TGL_LHR"].(string)
	switch {
	case strings.HasPrefix(birth, "Tidak Sesuai"):
		return outcomeRejected
	case strings.HasPrefix(birth, "Sesuai"):
		return outcomeSuccess
	}
	return outcomeFail
}

// noBodyMessage is the Response of an answer without a body (FR-PSAPI-004
// rule 3; agreed for other codes).
func noBodyMessage(code int) string {
	if code == http.StatusUnauthorized {
		return "the server responded with an HTTP 401 code, indicating an authorization problem."
	}
	return fmt.Sprintf("the server responded with an HTTP %d code.", code)
}

// callError is the Response of a call that got no answer, without the URL's
// credentials (agreed for ticket 09).
func callError(err error, u *url.URL, timeout time.Duration) string {
	var ne net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
		port := u.Port()
		if port == "" {
			port = map[string]string{"https": "443", "http": "80"}[u.Scheme]
		}
		return fmt.Sprintf("Connect to %s:%s timed out after %d ms", u.Hostname(), port, timeout.Milliseconds())
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err.Error()
	}
	return err.Error()
}
