package cases

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The outbound APIs of the Pre-Screening Check share their configuration
// shape (§9.25, FR-PSAPI-002) and the way an attempt is called and logged
// (FR-PSAPI-004).

// outboundAPI names one API: its Connect API Log api_name, its settings key
// (api/<settings>/..., secret/<settings>/apikey_ref), the settings keys of
// its base URL and of its key when another API's (base, secret; "" for its
// own) and the header that carries its key ("" for an API without one).
type outboundAPI struct {
	logName, settings, base, secret, keyHeader string
}

// maxResponseBytes bounds a response read into the log.
const maxResponseBytes = 10 << 20

// apiConfig is one API's configuration for an attempt. unbounded reads a
// response whole (the Pefindo PDF, agreed for ticket 13); otherwise a
// response is at most maxResponseBytes.
type apiConfig struct {
	simulate         bool
	simulateResponse string
	base, path       string
	timeout          time.Duration
	keyHeader, key   string
	maskedHeaders    []string
	unbounded        bool
}

// apiConfig reads api/<api>/* and the key named by secret/<api>/apikey_ref;
// a missing one is a system failure of the step (DB-RULE-084). The key
// header is masked in the log whatever api/log/masked_headers says (DEC-060).
func (e *Executor) apiConfig(ctx context.Context, a outboundAPI) (apiConfig, error) {
	st := e.s.settings
	api, keyHeader := a.settings, a.keyHeader
	c := apiConfig{keyHeader: keyHeader}
	var err error
	if c.maskedHeaders, err = e.s.maskedHeaderNames(ctx); err != nil {
		return c, err
	}
	if keyHeader != "" {
		c.maskedHeaders = append(c.maskedHeaders, keyHeader)
	}
	if c.simulate, err = st.Bool(ctx, "api/"+api+"/simulate"); err != nil {
		return c, err
	}
	if c.simulate {
		c.simulateResponse, err = st.String(ctx, "api/"+api+"/simulate_response")
		return c, err
	}
	base := a.base
	if base == "" {
		base = api
	}
	if c.base, err = st.String(ctx, "api/"+base+"/baseurl"); err != nil {
		return c, err
	}
	if c.path, err = st.String(ctx, "api/"+api+"/path"); err != nil {
		return c, err
	}
	timeoutMS, err := st.Int(ctx, "api/"+api+"/timeout_ms")
	if err != nil {
		return c, err
	}
	c.timeout = time.Duration(timeoutMS) * time.Millisecond
	if keyHeader == "" {
		return c, nil
	}
	secret := a.secret
	if secret == "" {
		secret = api
	}
	keyRef, err := st.String(ctx, "secret/"+secret+"/apikey_ref")
	if err != nil {
		return c, err
	}
	if c.key = e.getenv(keyRef); c.key == "" {
		return c, fmt.Errorf("%s API key: environment variable %s is empty (DEC-060)", api, keyRef)
	}
	return c, nil
}

// retrySettings reads api/<api>/retry_max and retry_gap_ms (FR-PSAPI-002).
func (e *Executor) retrySettings(a outboundAPI) func(context.Context, string) (int, time.Duration, error) {
	return func(ctx context.Context, _ string) (int, time.Duration, error) {
		max, err := e.s.settings.Int(ctx, "api/"+a.settings+"/retry_max")
		if err != nil {
			return 0, 0, err
		}
		gap, err := e.s.settings.Int(ctx, "api/"+a.settings+"/retry_gap_ms")
		return int(max), time.Duration(gap) * time.Millisecond, err
	}
}

// endpoint is base plus path, without credentials (§9.14).
func (c apiConfig) endpoint(path string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSuffix(c.base, "/") + path)
	if err != nil || u.Host == "" {
		return nil, errors.New("base URL or path is not a URL")
	}
	u.User = nil
	return u, nil
}

// call sends one attempt and fills log with the URL, masked headers, HTTP
// status, response (or the message of a missing one) and duration. It
// returns the body of an answer that has one (nil otherwise); a ctx error
// means the executor is stopping.
func (e *Executor) call(ctx context.Context, c apiConfig, log *apiLog, u *url.URL, body []byte, headers map[string]string) ([]byte, error) {
	log.url = u.String()
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(callCtx, log.method, log.url, rdr)
	if err != nil {
		return nil, errors.New("request could not be built")
	}
	if c.keyHeader != "" {
		req.Header.Set(c.keyHeader, c.key)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	log.requestHeaders = masked(req.Header, c.maskedHeaders)

	start := time.Now()
	resp, err := e.http.Do(req)
	var answer []byte
	if err == nil {
		var raw []byte
		if c.unbounded {
			raw, err = io.ReadAll(resp.Body)
		} else {
			raw, err = io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		}
		resp.Body.Close()
		code := resp.StatusCode
		log.httpStatus = &code
		log.responseHeaders = masked(resp.Header, c.maskedHeaders)
		switch {
		case err != nil:
		case !c.unbounded && len(raw) > maxResponseBytes:
			err = fmt.Errorf("response larger than %d bytes", maxResponseBytes)
		case len(bytes.TrimSpace(raw)) == 0:
			log.response = noBodyMessage(code)
		default:
			log.response, answer = string(raw), raw
		}
	}
	log.duration = time.Since(start)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		log.response = callError(err, u, c.timeout)
	}
	return answer, nil
}

// send completes an attempt whose log holds the request: a simulation is
// logged as Success with the configured response (FR-PSAPI-003); otherwise
// path is called and a body is judged by evaluate, anything else is a Fail.
func (e *Executor) send(ctx context.Context, c apiConfig, log *apiLog, path string, body []byte, headers map[string]string, evaluate func(string) outcome) (attempt, error) {
	if c.simulate {
		log.status, log.response, log.method = apiSuccess, c.simulateResponse, ""
		return attempt{outcome: outcomeSuccess, log: log}, nil
	}
	u, err := c.endpoint(path)
	if err != nil {
		return attempt{}, fmt.Errorf("%s: %w", log.name, err)
	}
	answer, err := e.call(ctx, c, log, u, body, headers)
	if err != nil {
		return attempt{}, err
	}
	o := outcomeFail
	if answer != nil {
		o = evaluate(log.response)
	}
	log.status = o.logStatus()
	return attempt{outcome: o, log: log}, nil
}

// failed is an attempt that failed before any call, e.g. on missing
// request data (agreed for ticket 09): logged as Fail and retried.
func failed(log *apiLog, cause error) attempt {
	log.status, log.response, log.method = apiFail, cause.Error(), ""
	return attempt{outcome: outcomeFail, log: log}
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
