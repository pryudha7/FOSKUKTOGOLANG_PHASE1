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

// The Dukcapil check (FR-PSAPI-003) checks an Individual Customer's
// personal data, never the Spouse's or a Business Entity's.

const (
	apiDukcapil = "Dukcapil"
	// dukcapilKeyHeader carries the API key; it is masked in the log whatever
	// api/log/masked_headers says (DEC-060).
	dukcapilKeyHeader = "x-Gateway-APIKey"
	// maxResponseBytes bounds a response read into the log.
	maxResponseBytes = 10 << 20
)

// dukcapilRequest is the body, in the SoT's field order.
type dukcapilRequest struct {
	NIK          string `json:"nik"`
	TanggalLahir string `json:"tanggalLahir"` // dd-MM-yyyy
	NamaLengkap  string `json:"namaLengkap"`
}

// dukcapilConfig is the call's configuration (§9.25, DEC-060).
type dukcapilConfig struct {
	simulate         bool
	simulateResponse string
	url              string
	timeout          time.Duration
	key              string
	maskedHeaders    []string
}

func (e *Executor) dukcapilCheck() check {
	return check{
		apiName: apiDukcapil,
		try:     e.tryDukcapil,
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

// dukcapilConfig reads the settings and the API key; a missing one is a
// system failure of the step (DB-RULE-084).
func (e *Executor) dukcapilConfig(ctx context.Context) (dukcapilConfig, error) {
	st := e.s.settings
	var c dukcapilConfig
	var err error
	if c.maskedHeaders, err = e.s.maskedHeaderNames(ctx); err != nil {
		return c, err
	}
	c.maskedHeaders = append(c.maskedHeaders, dukcapilKeyHeader)
	if c.simulate, err = st.Bool(ctx, "api/dukcapil/simulate"); err != nil {
		return c, err
	}
	if c.simulate {
		c.simulateResponse, err = st.String(ctx, "api/dukcapil/simulate_response")
		return c, err
	}
	base, err := st.String(ctx, "api/dukcapil/baseurl")
	if err != nil {
		return c, err
	}
	path, err := st.String(ctx, "api/dukcapil/path")
	if err != nil {
		return c, err
	}
	timeoutMS, err := st.Int(ctx, "api/dukcapil/timeout_ms")
	if err != nil {
		return c, err
	}
	keyRef, err := st.String(ctx, "secret/dukcapil/apikey_ref")
	if err != nil {
		return c, err
	}
	if c.key = e.getenv(keyRef); c.key == "" {
		return c, fmt.Errorf("Dukcapil API key: environment variable %s is empty (DEC-060)", keyRef)
	}
	u, err := url.Parse(strings.TrimSuffix(base, "/") + path)
	if err != nil || u.Host == "" {
		return c, errors.New("api/dukcapil/baseurl or api/dukcapil/path is not a URL")
	}
	u.User = nil // never logged with credentials (§9.14)
	c.url = u.String()
	c.timeout = time.Duration(timeoutMS) * time.Millisecond
	return c, nil
}

func (e *Executor) tryDukcapil(ctx context.Context, caseID string, attemptNumber int) (attempt, error) {
	var customerType *string
	if err := e.s.db.QueryRow(ctx, `SELECT customer_type FROM fos.work WHERE case_id = $1`, caseID).Scan(&customerType); err != nil {
		return attempt{}, err
	}
	if value(customerType) != customerTypeIndividual {
		return attempt{outcome: outcomeSkip}, nil
	}
	cfg, err := e.dukcapilConfig(ctx)
	if err != nil {
		return attempt{}, err
	}
	log := &apiLog{name: apiDukcapil, submittedAt: e.s.now(), method: http.MethodPost, attempt: attemptNumber}

	// FR-PSAPI-002 rule 6: the request is built from the case data each time;
	// missing data is a Fail of the attempt (FR-PSAPI-003, agreed).
	body, err := e.dukcapilBody(ctx, caseID)
	if err != nil {
		log.status, log.response, log.method = apiFail, err.Error(), ""
		return attempt{outcomeFail, log}, nil
	}
	log.request = string(body)

	if cfg.simulate {
		// No call; the configured response is logged as Success (FR-PSAPI-003).
		log.status, log.response, log.method = apiSuccess, cfg.simulateResponse, ""
		return attempt{outcomeSuccess, log}, nil
	}

	log.url = cfg.url
	callCtx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, cfg.url, bytes.NewReader(body))
	if err != nil {
		return attempt{}, errors.New("Dukcapil request could not be built")
	}
	req.Header.Set(dukcapilKeyHeader, cfg.key)
	req.Header.Set("Content-Type", "application/json")
	log.requestHeaders = masked(req.Header, cfg.maskedHeaders)

	start := time.Now()
	resp, err := e.http.Do(req)
	if err == nil {
		var raw []byte
		raw, err = io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		resp.Body.Close()
		code := resp.StatusCode
		log.httpStatus = &code
		log.responseHeaders = masked(resp.Header, cfg.maskedHeaders)
		switch {
		case err != nil:
		case len(raw) > maxResponseBytes:
			err = fmt.Errorf("response larger than %d bytes", maxResponseBytes)
		case len(bytes.TrimSpace(raw)) == 0:
			log.response = noBodyMessage(code)
		default:
			log.response = string(raw)
		}
	}
	log.duration = time.Since(start)
	if ctx.Err() != nil {
		return attempt{}, ctx.Err()
	}
	if err != nil {
		log.response = callError(err, req.URL, cfg.timeout)
		log.status = apiFail
		return attempt{outcomeFail, log}, nil
	}
	o := evaluateDukcapil(log.response)
	log.status = o.logStatus()
	return attempt{o, log}, nil
}

// dukcapilBody is the request for the case's Individual Customer.
func (e *Executor) dukcapilBody(ctx context.Context, caseID string) ([]byte, error) {
	var (
		nik, name *string
		birthDate *time.Time
	)
	err := e.s.db.QueryRow(ctx, `
		SELECT id_number, full_name, birth_date FROM fos.customer WHERE case_id = $1 AND customer_type = $2`,
		caseID, partyCustomer).Scan(&nik, &name, &birthDate)
	if isNotFound(err) || (err == nil && (value(nik) == "" || value(name) == "" || birthDate == nil)) {
		return nil, errors.New("Dukcapil request data missing: the customer's NIK, Nama or Tanggal Lahir is empty")
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(dukcapilRequest{value(nik), birthDate.Format("02-01-2006"), value(name)})
}

// evaluateDukcapil is FR-PSAPI-003 "Evaluasi hasil", judged on the body only.
func evaluateDukcapil(body string) outcome {
	var fields map[string]any
	if json.Unmarshal([]byte(body), &fields) != nil {
		return outcomeFail
	}
	if code, ok := fields["RESPONSE_CODE"]; ok {
		// Data not found passes (PROPOSED); the code may come as text or number.
		if fmt.Sprint(code) == "13" {
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
