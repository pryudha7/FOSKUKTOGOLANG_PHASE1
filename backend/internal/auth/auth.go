// Package auth authenticates operators (FR-LOGIN-001 to FR-LOGIN-006) and
// manages their sessions.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"fos/internal/config"
)

// Method is the login path used, recorded as login_log.auth_method (ENV-003).
type Method string

const (
	MethodDatabase Method = "DATABASE"
	MethodLDAP     Method = "LDAP"
)

// Endpoint is the channel a session belongs to (FR-ACCESS-004).
type Endpoint string

const EndpointDesktop Endpoint = "DESKTOP"

// LoginError is a login refusal: the SoT error code shown to the operator
// (§12) and the internal login_log fail_reason (§9.21).
type LoginError struct {
	Code       string
	FailReason string
}

func (e *LoginError) Error() string { return "login refused: " + e.Code }

var (
	// ErrInvalidCredential is ERR-001 for a wrong username or password or an
	// unknown user.
	ErrInvalidCredential = &LoginError{Code: "ERR-001", FailReason: "INVALID_CREDENTIAL"}
	// ErrNotRegistered is ERR-001 for a directory user who is not a FOS
	// operator. The operator sees the same message as ErrInvalidCredential.
	ErrNotRegistered = &LoginError{Code: "ERR-001", FailReason: "NOT_REGISTERED"}
	// ErrLDAPUnavailable is ERR-002: LDAP unreachable, service bind failed,
	// or LDAP configuration incomplete.
	ErrLDAPUnavailable = &LoginError{Code: "ERR-002", FailReason: "LDAP_UNAVAILABLE"}
	// ErrDatabaseUnavailable is ERR-003. The SoT defines no fail_reason for it.
	ErrDatabaseUnavailable = &LoginError{Code: "ERR-003"}
)

// Operator is the authenticated operator.
type Operator struct {
	UserID   string
	FullName string
}

// Attempt is one login attempt as received by the server.
type Attempt struct {
	Username  string
	Password  string
	ClientIP  string
	UserAgent string
	Endpoint  Endpoint
}

type Service struct {
	db   *pgxpool.Pool
	ldap config.LDAP
	now  func() time.Time
}

func NewService(db *pgxpool.Pool, ldap config.LDAP) *Service {
	return &Service{db: db, ldap: ldap, now: func() time.Time { return time.Now().UTC() }}
}

// dummyHash keeps the cost of an unknown-user attempt close to a wrong
// password, so response time does not reveal which usernames exist.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

// LoginDatabase authenticates against the FOS operator database
// (FR-LOGIN-003) and, on success, opens a session whose token is returned.
// Every attempt is written to login_log (DB-RULE-071).
func (s *Service) LoginDatabase(ctx context.Context, a Attempt) (Operator, string, error) {
	op, hash, err := s.findOperator(ctx, a.Username)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(a.Password))
		return s.refuse(ctx, MethodDatabase, a, "", ErrInvalidCredential)
	case err != nil:
		slog.Error("operator lookup failed", "error", err)
		return s.refuse(ctx, MethodDatabase, a, "", ErrDatabaseUnavailable)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(a.Password)) != nil {
		return s.refuse(ctx, MethodDatabase, a, op.UserID, ErrInvalidCredential)
	}
	return s.admit(ctx, MethodDatabase, a, op)
}

// LoginLDAP authenticates against the directory, then requires the user to
// be a registered FOS operator (FR-LOGIN-002).
func (s *Service) LoginLDAP(ctx context.Context, a Attempt) (Operator, string, error) {
	if !s.ldap.Complete() {
		slog.Error("LDAP login refused: LDAP configuration incomplete (ENV-008)")
		return s.refuse(ctx, MethodLDAP, a, "", ErrLDAPUnavailable)
	}
	if err := authenticateLDAP(s.ldap, a.Username, a.Password); err != nil {
		var refusal *LoginError
		if !errors.As(err, &refusal) {
			refusal = ErrLDAPUnavailable
		}
		return s.refuse(ctx, MethodLDAP, a, "", refusal)
	}
	op, _, err := s.findOperator(ctx, a.Username)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return s.refuse(ctx, MethodLDAP, a, "", ErrNotRegistered)
	case err != nil:
		slog.Error("operator lookup failed", "error", err)
		return s.refuse(ctx, MethodLDAP, a, "", ErrDatabaseUnavailable)
	}
	return s.admit(ctx, MethodLDAP, a, op)
}

// findOperator matches the username case-insensitively (TBD-012, agreed).
func (s *Service) findOperator(ctx context.Context, username string) (Operator, string, error) {
	var (
		op   Operator
		hash string
	)
	err := s.db.QueryRow(ctx,
		`SELECT user_id, full_name, password_hash FROM fos.operator WHERE lower(user_id) = lower($1)`,
		username).Scan(&op.UserID, &op.FullName, &hash)
	return op, hash, err
}

func (s *Service) admit(ctx context.Context, m Method, a Attempt, op Operator) (Operator, string, error) {
	token, err := s.openSession(ctx, op.UserID, a.Endpoint)
	if err != nil {
		slog.Error("open session failed", "error", err)
		return s.refuse(ctx, m, a, op.UserID, ErrDatabaseUnavailable)
	}
	s.logAttempt(ctx, m, a, op.UserID, nil)
	return op, token, nil
}

func (s *Service) refuse(ctx context.Context, m Method, a Attempt, operatorUserID string, e *LoginError) (Operator, string, error) {
	s.logAttempt(ctx, m, a, operatorUserID, e)
	return Operator{}, "", e
}

// SessionOperator returns the operator owning a live session token.
func (s *Service) SessionOperator(ctx context.Context, token string, endpoint Endpoint) (Operator, bool) {
	var op Operator
	err := s.db.QueryRow(ctx, `
		UPDATE fos.operator_session s SET last_activity_at = $3
		FROM fos.operator o
		WHERE s.token_hash = $1 AND s.endpoint = $2 AND o.user_id = s.user_id
		RETURNING o.user_id, o.full_name`,
		hashToken(token), string(endpoint), s.now()).Scan(&op.UserID, &op.FullName)
	return op, err == nil
}

func (s *Service) openSession(ctx context.Context, userID string, endpoint Endpoint) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now()
	_, err := s.db.Exec(ctx, `
		INSERT INTO fos.operator_session (token_hash, user_id, endpoint, created_at, last_activity_at)
		VALUES ($1, $2, $3, $4, $4)`, hashToken(token), userID, string(endpoint), now)
	return token, err
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// logAttempt writes one login_log row (FR-LOGIN-006); refusal is nil on
// success. A write failure never changes the login outcome (DB-RULE-072).
func (s *Service) logAttempt(ctx context.Context, m Method, a Attempt, operatorUserID string, refusal *LoginError) {
	result, failReason, errorCode := "SUCCESS", "", ""
	if refusal != nil {
		result, failReason, errorCode = "FAIL", refusal.FailReason, refusal.Code
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO fos.login_log
			(login_id, operator_user_id, auth_method, result, fail_reason, error_code, client_ip, user_agent, attempted_at)
		VALUES ($1, NULLIF($2, ''), $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9)`,
		a.Username, operatorUserID, string(m), result, failReason, errorCode, a.ClientIP, a.UserAgent, s.now())
	if err != nil {
		slog.Error("LOGIN_LOG_WRITE_FAILED", "error", err)
	}
}
