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
)

// Method is the login path used (ENV-003).
type Method string

const (
	MethodDatabase Method = "DATABASE"
	MethodLDAP     Method = "LDAP"
)

// Endpoint is the channel a session belongs to (FR-ACCESS-004).
type Endpoint string

const EndpointDesktop Endpoint = "DESKTOP"

var (
	// ErrInvalidCredential is ERR-001: wrong username or password, unknown
	// user, or unregistered operator. Callers must not tell these apart.
	ErrInvalidCredential = errors.New("invalid credential")
	// ErrDatabaseUnavailable is ERR-003.
	ErrDatabaseUnavailable = errors.New("database unavailable")
)

// Operator is the authenticated operator.
type Operator struct {
	UserID   string
	FullName string
}

// Attempt is one login attempt as received by the server.
type Attempt struct {
	Method    Method
	Username  string
	Password  string
	ClientIP  string
	UserAgent string
	Endpoint  Endpoint
}

type Service struct {
	db  *pgxpool.Pool
	now func() time.Time
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db, now: func() time.Time { return time.Now().UTC() }}
}

// dummyHash keeps the cost of an unknown-user attempt close to a wrong
// password, so response time does not reveal which usernames exist.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

// LoginDatabase authenticates against the FOS operator database
// (FR-LOGIN-003) and, on success, opens a session whose token is returned.
func (s *Service) LoginDatabase(ctx context.Context, a Attempt) (Operator, string, error) {
	a.Method = MethodDatabase
	var (
		op   Operator
		hash string
	)
	err := s.db.QueryRow(ctx,
		`SELECT user_id, full_name, password_hash FROM fos.operator WHERE lower(user_id) = lower($1)`,
		a.Username).Scan(&op.UserID, &op.FullName, &hash)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(a.Password))
		s.logAttempt(ctx, a, "", failInvalidCredential)
		return Operator{}, "", ErrInvalidCredential
	case err != nil:
		slog.Error("operator lookup failed", "error", err)
		return Operator{}, "", ErrDatabaseUnavailable
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(a.Password)) != nil {
		s.logAttempt(ctx, a, op.UserID, failInvalidCredential)
		return Operator{}, "", ErrInvalidCredential
	}

	token, err := s.openSession(ctx, op.UserID, a.Endpoint)
	if err != nil {
		slog.Error("open session failed", "error", err)
		return Operator{}, "", ErrDatabaseUnavailable
	}
	s.logAttempt(ctx, a, op.UserID, "")
	return op, token, nil
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

// fail_reason values (SoT §9.21).
const failInvalidCredential = "INVALID_CREDENTIAL"

// logAttempt writes one login_log row (FR-LOGIN-006). A write failure never
// changes the login outcome (DB-RULE-072).
func (s *Service) logAttempt(ctx context.Context, a Attempt, operatorUserID, failReason string) {
	result, errorCode := "SUCCESS", ""
	if failReason != "" {
		result, errorCode = "FAIL", "ERR-001"
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO fos.login_log
			(login_id, operator_user_id, auth_method, result, fail_reason, error_code, client_ip, user_agent, attempted_at)
		VALUES ($1, NULLIF($2, ''), $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9)`,
		a.Username, operatorUserID, string(a.Method), result, failReason, errorCode, a.ClientIP, a.UserAgent, s.now())
	if err != nil {
		slog.Error("LOGIN_LOG_WRITE_FAILED", "error", err)
	}
}
