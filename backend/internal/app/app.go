// Package app wires the FOS HTTP API.
package app

import (
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

// Error messages shown to the operator (SoT §12).
const (
	msgERR001 = "Informasi yang Anda masukkan tidak dikenali."
	msgERR003 = "Layanan sedang tidak tersedia. Silakan coba beberapa saat lagi."
)

type server struct {
	cfg  config.Config
	auth *auth.Service
}

func New(cfg config.Config, pool *pgxpool.Pool) http.Handler {
	s := &server{cfg: cfg, auth: auth.NewService(pool)}
	api := cfg.EndpointPathDesktop + "/api"
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+api+"/auth/login", s.login)
	mux.HandleFunc("GET "+api+"/me", s.me)
	return s.requireHTTPS(mux)
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type operatorJSON struct {
	UserID   string `json:"userId"`
	FullName string `json:"fullName"`
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Method   string `json:"method"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Method != string(auth.MethodDatabase) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "BAD_REQUEST", Message: "Permintaan tidak valid."})
		return
	}
	op, token, err := s.auth.LoginDatabase(r.Context(), auth.Attempt{
		Username:  req.Username,
		Password:  req.Password,
		ClientIP:  s.clientIP(r),
		UserAgent: r.UserAgent(),
		Endpoint:  auth.EndpointDesktop,
	})
	switch {
	case errors.Is(err, auth.ErrInvalidCredential):
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "ERR-001", Message: msgERR001})
		return
	case err != nil:
		writeJSON(w, http.StatusServiceUnavailable, apiError{Code: "ERR-003", Message: msgERR003})
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
	writeJSON(w, http.StatusOK, operatorJSON{UserID: op.UserID, FullName: op.FullName})
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(sessionCookie)
	if err == nil {
		if op, ok := s.auth.SessionOperator(r.Context(), c.Value, auth.EndpointDesktop); ok {
			writeJSON(w, http.StatusOK, operatorJSON{UserID: op.UserID, FullName: op.FullName})
			return
		}
	}
	writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "Sesi tidak ditemukan."})
}

// requireHTTPS refuses any request that did not arrive over TLS, either
// directly or via a trusted reverse proxy (FR-LOGIN-004 rule 3).
func (s *server) requireHTTPS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secure := r.TLS != nil ||
			(s.fromTrustedProxy(r) && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"))
		if !secure {
			writeJSON(w, http.StatusForbidden, apiError{Code: "HTTPS_REQUIRED", Message: "Koneksi harus melalui HTTPS."})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) trusted(addr netip.Addr) bool {
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
	return ok && s.trusted(addr)
}

// clientIP follows DB-RULE-068: the connection address, or behind a trusted
// proxy the left-most X-Forwarded-For address that is not a trusted proxy.
func (s *server) clientIP(r *http.Request) string {
	addr, ok := remoteAddr(r)
	if !ok {
		return ""
	}
	if s.trusted(addr) {
		for _, part := range strings.Split(r.Header.Get("X-Forwarded-For"), ",") {
			fwd, err := netip.ParseAddr(strings.TrimSpace(part))
			if err == nil && !s.trusted(fwd) {
				return fwd.String()
			}
		}
	}
	return addr.String()
}
