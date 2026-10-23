package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// FR-ACCESS-005: after IdleWarning without a request the UI warns; at
// IdleTimeout the server ends a desktop session. Whether these come from
// configuration is TBD-040.
const (
	IdleWarning = 14 * time.Minute
	IdleTimeout = 15 * time.Minute
)

// Named is a master-data entry: its stable code and display name.
type Named struct {
	Code string
	Name string
}

// nullableNamed scans the code and name of a master-data entry that may be
// absent.
type nullableNamed struct{ code, name *string }

func (n nullableNamed) named() *Named {
	if n.code == nil {
		return nil
	}
	return &Named{Code: *n.code, Name: *n.name}
}

// Session is a live session and the access context of its operator.
type Session struct {
	Operator
	Method   Method
	Endpoint Endpoint
	Role     Named
	// AccessGroup, WorkGroup and WorkQueue come from the role's active
	// mapping (FR-ACCESS-002); each is nil when there is none. There is no
	// default (AC-FR-ACCESS-002-03).
	AccessGroup, WorkGroup, WorkQueue *Named
	// IdleTimeoutApplies reports whether the session ends after IdleTimeout.
	IdleTimeoutApplies bool
}

// Session returns the live session for a token on an endpoint, ending it if
// it has been idle too long, and counts the call as activity. A token from
// another endpoint is not a session here (FR-ACCESS-004).
func (s *Service) Session(ctx context.Context, token string, endpoint Endpoint) (Session, bool) {
	var (
		sess         Session
		sessionID    int64
		method       string
		lastActivity time.Time
		ag, wg, wq   nullableNamed
	)
	err := s.db.QueryRow(ctx, `
		SELECT s.session_id, s.auth_method, s.last_activity_at, o.user_id, o.full_name, r.role_code, r.role_name,
		       m.ag_code, m.ag_name, m.wg_code, m.wg_name, m.wq_code, m.wq_name
		FROM fos.operator_session s
		JOIN fos.operator o ON o.user_id = s.user_id
		JOIN fos.roles r ON r.role_code = o.role_code
		LEFT JOIN LATERAL (
			-- The active mapping, ignored if it refers to an inactive master (§9.3).
			SELECT ag.access_group_code AS ag_code, ag.access_group_name AS ag_name,
			       wg.work_group_code AS wg_code, wg.work_group_name AS wg_name,
			       wq.work_queue_code AS wq_code, wq.work_queue_name AS wq_name
			FROM fos.role_access_mappings ram
			JOIN fos.access_groups ag ON ag.access_group_id = ram.access_group_id AND ag.is_active
			JOIN fos.work_groups wg ON wg.work_group_id = ram.work_group_id AND wg.is_active
			LEFT JOIN fos.work_queues wq ON wq.work_queue_id = ram.work_queue_id
			WHERE ram.role_id = r.role_id AND ram.is_active AND r.is_active
			  AND (ram.work_queue_id IS NULL OR wq.is_active)
		) m ON TRUE
		WHERE s.token_hash = $1 AND s.endpoint = $2`,
		hashToken(token), string(endpoint)).Scan(
		&sessionID, &method, &lastActivity, &sess.UserID, &sess.FullName, &sess.Role.Code, &sess.Role.Name,
		&ag.code, &ag.name, &wg.code, &wg.name, &wq.code, &wq.name)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("session lookup failed", "error", err)
		}
		return Session{}, false
	}
	sess.Method, sess.Endpoint = Method(method), endpoint
	sess.AccessGroup, sess.WorkGroup, sess.WorkQueue = ag.named(), wg.named(), wq.named()
	sess.IdleTimeoutApplies = idleTimeoutApplies(sess)

	now := s.now()
	if sess.IdleTimeoutApplies && now.Sub(lastActivity) >= IdleTimeout {
		s.endSession(ctx, sessionID)
		return Session{}, false
	}
	if _, err := s.db.Exec(ctx,
		`UPDATE fos.operator_session SET last_activity_at = $2 WHERE session_id = $1`, sessionID, now); err != nil {
		slog.Error("record session activity failed", "error", err)
	}
	return sess, true
}

// Logout ends the session for a token on an endpoint, if there is one. A
// token from another endpoint is left alone (FR-ACCESS-004).
func (s *Service) Logout(ctx context.Context, token string, endpoint Endpoint) {
	if _, err := s.db.Exec(ctx,
		`DELETE FROM fos.operator_session WHERE token_hash = $1 AND endpoint = $2`,
		hashToken(token), string(endpoint)); err != nil {
		slog.Error("logout failed", "error", err)
	}
}

func (s *Service) endSession(ctx context.Context, sessionID int64) {
	if _, err := s.db.Exec(ctx, `DELETE FROM fos.operator_session WHERE session_id = $1`, sessionID); err != nil {
		slog.Error("end idle session failed", "error", err)
	}
}

// idleTimeoutApplies follows FR-ACCESS-005 rules 3-4: desktop only, and not
// for a role whose name starts with "Identity Management" or an operator
// whose access group is Authors or Admin. The two checks are separate, and
// both compare names, as the rule does.
func idleTimeoutApplies(s Session) bool {
	if s.Endpoint != EndpointDesktop || strings.HasPrefix(s.Role.Name, "Identity Management") {
		return false
	}
	return s.AccessGroup == nil || (s.AccessGroup.Name != "Authors" && s.AccessGroup.Name != "Admin")
}

func (s *Service) openSession(ctx context.Context, userID string, m Method, endpoint Endpoint) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now()
	_, err := s.db.Exec(ctx, `
		INSERT INTO fos.operator_session (token_hash, user_id, auth_method, endpoint, created_at, last_activity_at)
		VALUES ($1, $2, $3, $4, $5, $5)`, hashToken(token), userID, string(m), string(endpoint), now)
	return token, err
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
