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

	"github.com/jackc/pgx/v5/pgxpool"

	"fos/internal/auth"
	"fos/internal/config"
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
}

type server struct {
	cfg  config.Config
	auth *auth.Service
}

func New(cfg config.Config, pool *pgxpool.Pool) http.Handler {
	s := &server{cfg: cfg, auth: auth.NewService(pool, cfg.LDAP)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+cfg.LoginPathLDAP, s.login(s.auth.LoginLDAP))   // API-001
	mux.HandleFunc("POST "+cfg.LoginPathDB, s.login(s.auth.LoginDatabase)) // API-002
	mux.HandleFunc("GET "+cfg.EndpointPathDesktop+"/api/me", s.me)
	return s.requireHTTPS(mux)
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
func (s *server) login(authenticate func(context.Context, auth.Attempt) (auth.Operator, string, error)) http.HandlerFunc {
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
			Endpoint:  auth.EndpointDesktop,
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
			Path:     s.cfg.EndpointPathDesktop,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})
		writeOperator(w, op)
	}
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if op, ok := s.auth.SessionOperator(r.Context(), c.Value, auth.EndpointDesktop); ok {
			writeOperator(w, op)
			return
		}
	}
	w.WriteHeader(http.StatusUnauthorized)
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
