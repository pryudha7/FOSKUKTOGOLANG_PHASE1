// Package auth authenticates operators (FR-LOGIN-001 to FR-LOGIN-006) and
// manages their sessions.
package auth

import (
	"context"
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

const (
	EndpointDesktop Endpoint = "DESKTOP"
	EndpointMobile  Endpoint = "MOBILE"
)

// LoginError is a login refusal: the SoT error code shown to the operator
// (§12) and the internal login_log fail_reason (§9.21).
type LoginError struct {
	Code       string
	FailReason string
}

func (e *LoginError) Error() string { return "login refused: " + e.Code }

var (
	// ErrInvalidCredential is ERR-001 for a wrong username or password, an
	// unknown user, or a directory refusing the user's bind.
	ErrInvalidCredential = &LoginError{Code: "ERR-001", FailReason: "INVALID_CREDENTIAL"}
	// ErrNotRegistered is ERR-001 for a directory user who is not a FOS
	// operator. The operator sees the same message as ErrInvalidCredential.
	ErrNotRegistered = &LoginError{Code: "ERR-001", FailReason: "NOT_REGISTERED"}
	// ErrLDAPUnavailable is ERR-002: LDAP unreachable, service bind failed,
	// or LDAP configuration incomplete.
	ErrLDAPUnavailable = &LoginError{Code: "ERR-002", FailReason: "LDAP_UNAVAILABLE"}
	// ErrDatabaseUnavailable is ERR-003. The SoT defines no fail_reason for it.
	ErrDatabaseUnavailable = &LoginError{Code: "ERR-003"}
	// ErrNoMobileAccess is ERR-004: valid credentials on the mobile endpoint
	// without the operator's mobile access checklist (FR-ACCESS-003 rule 4).
	ErrNoMobileAccess = &LoginError{Code: "ERR-004"}
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

func NewService(db *pgxpool.Pool, ldap config.LDAP, now func() time.Time) *Service {
	return &Service{db: db, ldap: ldap, now: now}
}

// dummyHash keeps the cost of an unknown-user attempt close to a wrong
// password, so response time does not reveal which usernames exist.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

// LoginDatabase authenticates against the FOS operator database
// (FR-LOGIN-003) and, on success, opens a session whose token is returned.
// Every attempt is written to login_log (DB-RULE-071).
func (s *Service) LoginDatabase(ctx context.Context, a Attempt) (Operator, string, error) {
	l := s.newLogin(ctx, MethodDatabase, a)
	rec, found, err := s.findOperator(ctx, a.Username)
	switch {
	case err != nil:
		return l.refuse("", err)
	case !found:
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(a.Password))
		return l.refuse("", ErrInvalidCredential)
	case bcrypt.CompareHashAndPassword([]byte(rec.passwordHash), []byte(a.Password)) != nil:
		return l.refuse(rec.UserID, ErrInvalidCredential)
	}
	return l.admit(rec)
}

// LoginLDAP authenticates against the directory, then requires the user to
// be a registered FOS operator (FR-LOGIN-002).
// VerifyServiceAccount checks Basic credentials of a system account (e.g.
// the RPA calling PBSResponse, FR-PSAPI-009): an operator with roleCode and
// a matching bcrypt password. It returns the stored user ID. No session is
// made and no login_log row is written (DB-RULE-069 concerns operators).
func (s *Service) VerifyServiceAccount(ctx context.Context, username, password, roleCode string) (string, bool, error) {
	var userID, hash, role string
	err := s.db.QueryRow(ctx, `SELECT user_id, password_hash, role_code FROM fos.operator WHERE lower(user_id) = lower($1)`, username).
		Scan(&userID, &hash, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password)) // same cost as a known user
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil || role != roleCode {
		return "", false, nil
	}
	return userID, true, nil
}

func (s *Service) LoginLDAP(ctx context.Context, a Attempt) (Operator, string, error) {
	l := s.newLogin(ctx, MethodLDAP, a)
	if !s.ldap.Complete() {
		slog.Error("LDAP login refused: LDAP configuration incomplete (ENV-008)")
		return l.refuse("", ErrLDAPUnavailable)
	}
	if refusal := authenticateLDAP(s.ldap, a.Username, a.Password); refusal != nil {
		return l.refuse("", refusal)
	}
	rec, found, err := s.findOperator(ctx, a.Username)
	switch {
	case err != nil:
		return l.refuse("", err)
	case !found:
		return l.refuse("", ErrNotRegistered)
	}
	return l.admit(rec)
}

type operatorRecord struct {
	Operator
	passwordHash string
	mobileAccess bool
}

// findOperator matches the username case-insensitively (TBD-012, agreed). A
// lookup failure is ErrDatabaseUnavailable.
func (s *Service) findOperator(ctx context.Context, username string) (operatorRecord, bool, *LoginError) {
	var rec operatorRecord
	err := s.db.QueryRow(ctx,
		`SELECT user_id, full_name, password_hash, mobile_access FROM fos.operator WHERE lower(user_id) = lower($1)`,
		username).Scan(&rec.UserID, &rec.FullName, &rec.passwordHash, &rec.mobileAccess)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return operatorRecord{}, false, nil
	case err != nil:
		slog.Error("operator lookup failed", "error", err)
		return operatorRecord{}, false, ErrDatabaseUnavailable
	}
	return rec, true, nil
}

// login is one attempt on one login path; it ends in admit or refuse, each
// of which writes the login_log row.
type login struct {
	s      *Service
	ctx    context.Context
	method Method
	a      Attempt
}

func (s *Service) newLogin(ctx context.Context, m Method, a Attempt) login {
	return login{s: s, ctx: ctx, method: m, a: a}
}

// admit opens a session for an authenticated operator, unless the mobile
// endpoint is used without mobile access.
func (l login) admit(rec operatorRecord) (Operator, string, error) {
	if l.a.Endpoint == EndpointMobile && !rec.mobileAccess {
		return l.refuse(rec.UserID, ErrNoMobileAccess)
	}
	token, err := l.s.openSession(l.ctx, rec.UserID, l.method, l.a.Endpoint)
	if err != nil {
		slog.Error("open session failed", "error", err)
		return l.refuse(rec.UserID, ErrDatabaseUnavailable)
	}
	l.log(rec.UserID, nil)
	return rec.Operator, token, nil
}

func (l login) refuse(operatorUserID string, e *LoginError) (Operator, string, error) {
	l.log(operatorUserID, e)
	return Operator{}, "", e
}

// log writes one login_log row (FR-LOGIN-006); refusal is nil on success. A
// write failure never changes the login outcome (DB-RULE-072).
func (l login) log(operatorUserID string, refusal *LoginError) {
	result, failReason, errorCode := "SUCCESS", "", ""
	if refusal != nil {
		result, failReason, errorCode = "FAIL", refusal.FailReason, refusal.Code
	}
	_, err := l.s.db.Exec(l.ctx, `
		INSERT INTO fos.login_log
			(login_id, operator_user_id, auth_method, result, fail_reason, error_code, client_ip, user_agent, attempted_at)
		VALUES ($1, NULLIF($2, ''), $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9)`,
		l.a.Username, operatorUserID, string(l.method), result, failReason, errorCode, l.a.ClientIP, l.a.UserAgent, l.s.now())
	if err != nil {
		slog.Error("LOGIN_LOG_WRITE_FAILED", "error", err)
	}
}
