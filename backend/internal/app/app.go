// Package app wires the FOS HTTP API.
//
// Response format and HTTP status codes are TBD-013 in the SoT; until it is
// settled, refusals are JSON {code, message} for the SoT error codes (§12) and
// a bare status code otherwise.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"fos/internal/auth"
	"fos/internal/cases"
	"fos/internal/config"
	"fos/internal/gcs"
	"fos/internal/settings"
)

const sessionCookie = "fos_session"

// loginErrors gives each SoT login error its HTTP status and the message
// shown to the operator (§12).
var loginErrors = map[string]struct {
	status  int
	message string
}{
	"ERR-001": {http.StatusUnauthorized, "Informasi yang Anda masukkan tidak dikenali."},
	"ERR-002": {http.StatusServiceUnavailable, "Layanan autentikasi sedang tidak tersedia. Silakan coba beberapa saat lagi."},
	"ERR-003": {http.StatusServiceUnavailable, "Layanan sedang tidak tersedia. Silakan coba beberapa saat lagi."},
	"ERR-004": {http.StatusForbidden, "Anda tidak memiliki akses ke aplikasi mobile."},
}

// endpoint is one channel (FR-ACCESS-003). Its login paths are the
// configured LOGIN_PATH_* under loginPrefix: none on desktop, and
// ENDPOINT_PATH_MOBILE on mobile (agreed for TBD-017).
type endpoint struct {
	id          auth.Endpoint
	path        string
	loginPrefix string
}

type server struct {
	cfg   config.Config
	auth  *auth.Service
	cases *cases.Service
}

// Option adjusts how the app is built.
type Option func(*options)

type options struct{ now func() time.Time }

// WithClock replaces the wall clock, e.g. with a test clock.
func WithClock(now func() time.Time) Option { return func(o *options) { o.now = now } }

// App is the HTTP API and the background Pre-Screening Check executor.
type App struct {
	http.Handler
	cases *cases.Service
}

// Close stops the background executor and waits for it.
func (a *App) Close() { a.cases.Close() }

func New(cfg config.Config, pool *pgxpool.Pool, opts ...Option) *App {
	o := options{now: func() time.Time { return time.Now().UTC() }}
	for _, f := range opts {
		f(&o)
	}
	s := &server{cfg: cfg, auth: auth.NewService(pool, cfg.LDAP, o.now), cases: cases.NewService(pool, o.now, gcs.New(cfg.GCS, o.now), settings.New(pool, o.now))}
	s.cases.StartRuns()
	mux := http.NewServeMux()
	for _, ep := range []endpoint{
		{id: auth.EndpointDesktop, path: cfg.EndpointPathDesktop},
		{id: auth.EndpointMobile, path: cfg.EndpointPathMobile, loginPrefix: cfg.EndpointPathMobile},
	} {
		mux.HandleFunc("POST "+ep.loginPrefix+cfg.LoginPathLDAP, s.login(ep, s.auth.LoginLDAP))   // API-001
		mux.HandleFunc("POST "+ep.loginPrefix+cfg.LoginPathDB, s.login(ep, s.auth.LoginDatabase)) // API-002
		mux.HandleFunc("GET "+ep.path+"/api/me", s.me(ep))
		mux.HandleFunc("POST "+ep.path+"/api/logout", s.logout(ep))
	}
	// Case functions are desktop-only; mobile functions are TBD-017.
	desktop := endpoint{id: auth.EndpointDesktop, path: cfg.EndpointPathDesktop}
	mux.HandleFunc("POST "+desktop.path+"/api/cases", s.createCase(desktop))
	mux.HandleFunc("GET "+desktop.path+"/api/cases/{caseId}", s.caseFrame(desktop))
	mux.HandleFunc("GET "+desktop.path+"/api/cases/{caseId}/assignments/{assignmentId}", s.openAssignment(desktop))
	mux.HandleFunc("POST "+desktop.path+"/api/cases/{caseId}/assignments/{assignmentId}/submit", s.submitAssignment(desktop))
	mux.HandleFunc("POST "+desktop.path+"/api/cases/{caseId}/assignments/{assignmentId}/back", s.backAssignment(desktop))
	mux.HandleFunc("POST "+desktop.path+"/api/cases/{caseId}/assignments/{assignmentId}/create", s.createUnderlying(desktop))
	mux.HandleFunc("POST "+desktop.path+"/api/cases/{caseId}/assignments/{assignmentId}/review", s.submitReview(desktop))
	mux.HandleFunc("POST "+desktop.path+"/api/cases/{caseId}/assignments/{assignmentId}/retry", s.retryReview(desktop))
	mux.HandleFunc("POST "+desktop.path+"/api/cases/{caseId}/assignments/{assignmentId}/documents", s.uploadDocument(desktop))
	mux.HandleFunc("GET "+desktop.path+"/api/cases/{caseId}/documents/{fileId}", s.downloadDocument(desktop))
	// External services (FR-PSAPI-009) live at the root, outside the endpoints.
	mux.HandleFunc(serviceNamespace+"v1/PBSResponse", s.pbsResponse)
	mux.HandleFunc(serviceNamespace, s.serviceUnknown)
	return &App{Handler: s.requireHTTPS(mux), cases: s.cases}
}

// loginPath is the login page a session on ep came from.
func (s *server) loginPath(ep endpoint, m auth.Method) string {
	if m == auth.MethodLDAP {
		return ep.loginPrefix + s.cfg.LoginPathLDAP
	}
	return ep.loginPrefix + s.cfg.LoginPathDB
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeOperator(w http.ResponseWriter, op auth.Operator) {
	writeJSON(w, http.StatusOK, struct {
		UserID   string `json:"userId"`
		FullName string `json:"fullName"`
	}{op.UserID, op.FullName})
}

// login handles a credential submit to a login path: username and password,
// no other field (API-001, API-002).
func (s *server) login(ep endpoint, authenticate func(context.Context, auth.Attempt) (auth.Operator, string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		op, token, err := authenticate(r.Context(), auth.Attempt{
			Username:  req.Username,
			Password:  req.Password,
			ClientIP:  s.clientIP(r),
			UserAgent: r.UserAgent(),
			Endpoint:  ep.id,
		})
		var refusal *auth.LoginError
		if errors.As(err, &refusal) {
			e := loginErrors[refusal.Code]
			writeJSON(w, e.status, struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{refusal.Code, e.message})
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookie,
			Value:    token,
			Path:     ep.path,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})
		writeOperator(w, op)
	}
}

type namedJSON struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func toNamedJSON(n *auth.Named) *namedJSON {
	if n == nil {
		return nil
	}
	return &namedJSON{n.Code, n.Name}
}

// idleJSON states the FR-ACCESS-005 timings, so the UI holds no copy of them.
type idleJSON struct {
	WarnAfterSeconds   int `json:"warnAfterSeconds"`
	LogoffAfterSeconds int `json:"logoffAfterSeconds"`
}

// me returns the session's operator and access context. Without a session of
// this endpoint the request is unauthenticated (FR-ACCESS-004).
func (s *server) me(ep endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.session(w, r, ep)
		if !ok {
			return
		}
		var idle *idleJSON
		if sess.IdleTimeoutApplies {
			idle = &idleJSON{int(auth.IdleWarning.Seconds()), int(auth.IdleTimeout.Seconds())}
		}
		writeJSON(w, http.StatusOK, struct {
			UserID      string         `json:"userId"`
			FullName    string         `json:"fullName"`
			Role        *namedJSON     `json:"role"`
			AccessGroup *namedJSON     `json:"accessGroup"`
			WorkGroup   *namedJSON     `json:"workGroup"`
			WorkQueue   *namedJSON     `json:"workQueue"`
			Idle        *idleJSON      `json:"idle"`
			LoginPath   string         `json:"loginPath"`
			CaseTypes   []caseTypeJSON `json:"caseTypes"`
		}{
			sess.UserID, sess.FullName, toNamedJSON(&sess.Role),
			toNamedJSON(sess.AccessGroup), toNamedJSON(sess.WorkGroup), toNamedJSON(sess.WorkQueue),
			idle, s.loginPath(ep, sess.Method), toCaseTypesJSON(cases.CreatableCaseTypes(sess)),
		})
	}
}

// logout ends the session of this endpoint and clears its cookie
// (FR-ACCESS-005 step 4; FR-NAV-002 reuses it).
func (s *server) logout(ep endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil {
			s.auth.Logout(r.Context(), c.Value, ep.id)
		}
		http.SetCookie(w, &http.Cookie{
			Name: sessionCookie, Value: "", Path: ep.path, MaxAge: -1,
			HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

// requireHTTPS refuses, without processing, any request that did not arrive
// over TLS directly or via a trusted reverse proxy (FR-LOGIN-004 rule 3).
func (s *server) requireHTTPS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secure := r.TLS != nil ||
			(s.fromTrustedProxy(r) && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"))
		if !secure {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) isTrustedProxy(addr netip.Addr) bool {
	for _, p := range s.cfg.TrustedProxyCIDRs {
		if p.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}

func remoteAddr(r *http.Request) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	return addr, err == nil
}

func (s *server) fromTrustedProxy(r *http.Request) bool {
	addr, ok := remoteAddr(r)
	return ok && s.isTrustedProxy(addr)
}

// clientIP follows DB-RULE-068: the connection address, or behind a trusted
// proxy the left-most X-Forwarded-For address that is not a trusted proxy.
func (s *server) clientIP(r *http.Request) string {
	addr, ok := remoteAddr(r)
	if !ok {
		return ""
	}
	if s.isTrustedProxy(addr) {
		for _, part := range strings.Split(r.Header.Get("X-Forwarded-For"), ",") {
			fwd, err := netip.ParseAddr(strings.TrimSpace(part))
			if err == nil && !s.isTrustedProxy(fwd) {
				return fwd.String()
			}
		}
	}
	return addr.String()
}
