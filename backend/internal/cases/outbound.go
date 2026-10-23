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

// Outbound Pre-Screening APIs share their configuration shape (§9.25,
// FR-PSAPI-002) and the way an attempt is called and logged (FR-PSAPI-004).

// maxResponseBytes bounds a response read into the log.
const maxResponseBytes = 10 << 20

// apiConfig is one API's configuration for an attempt.
type apiConfig struct {
	simulate         bool
	simulateResponse string
	base, path       string
	timeout          time.Duration
	keyHeader, key   string
	maskedHeaders    []string
}

// apiConfig reads api/<api>/* and the key named by secret/<api>/apikey_ref;
// a missing one is a system failure of the step (DB-RULE-084). keyHeader is
// masked in the log whatever api/log/masked_headers says (DEC-060).
func (e *Executor) apiConfig(ctx context.Context, api, keyHeader string) (apiConfig, error) {
	st := e.s.settings
	c := apiConfig{keyHeader: keyHeader}
	var err error
	if c.maskedHeaders, err = e.s.maskedHeaderNames(ctx); err != nil {
		return c, err
	}
	c.maskedHeaders = append(c.maskedHeaders, keyHeader)
	if c.simulate, err = st.Bool(ctx, "api/"+api+"/simulate"); err != nil {
		return c, err
	}
	if c.simulate {
		c.simulateResponse, err = st.String(ctx, "api/"+api+"/simulate_response")
		return c, err
	}
	if c.base, err = st.String(ctx, "api/"+api+"/baseurl"); err != nil {
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
	keyRef, err := st.String(ctx, "secret/"+api+"/apikey_ref")
	if err != nil {
		return c, err
	}
	if c.key = e.getenv(keyRef); c.key == "" {
		return c, fmt.Errorf("%s API key: environment variable %s is empty (DEC-060)", api, keyRef)
	}
	return c, nil
}

// retrySettings reads api/<api>/retry_max and retry_gap_ms (FR-PSAPI-002).
func (e *Executor) retrySettings(api string) func(context.Context) (int, time.Duration, error) {
	return func(ctx context.Context) (int, time.Duration, error) {
		max, err := e.s.settings.Int(ctx, "api/"+api+"/retry_max")
		if err != nil {
			return 0, 0, err
		}
		gap, err := e.s.settings.Int(ctx, "api/"+api+"/retry_gap_ms")
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
// reports whether an answer with a body arrived; a ctx error means the
// executor is stopping.
func (e *Executor) call(ctx context.Context, c apiConfig, log *apiLog, u *url.URL, body []byte, headers map[string]string) (bool, error) {
	log.url = u.String()
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(callCtx, log.method, log.url, rdr)
	if err != nil {
		return false, errors.New("request could not be built")
	}
	req.Header.Set(c.keyHeader, c.key)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	log.requestHeaders = masked(req.Header, c.maskedHeaders)

	start := time.Now()
	resp, err := e.http.Do(req)
	answered := false
	if err == nil {
		var raw []byte
		raw, err = io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		resp.Body.Close()
		code := resp.StatusCode
		log.httpStatus = &code
		log.responseHeaders = masked(resp.Header, c.maskedHeaders)
		switch {
		case err != nil:
		case len(raw) > maxResponseBytes:
			err = fmt.Errorf("response larger than %d bytes", maxResponseBytes)
		case len(bytes.TrimSpace(raw)) == 0:
			log.response = noBodyMessage(code)
		default:
			log.response, answered = string(raw), true
		}
	}
	log.duration = time.Since(start)
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		log.response = callError(err, u, c.timeout)
	}
	return answered, nil
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
