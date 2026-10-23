package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"fos/internal/apptest"
)

const errERR001 = "Informasi yang Anda masukkan tidak dikenali."

func seededHarness(t *testing.T) *apptest.Harness {
	t.Helper()
	h := apptest.New(t)
	h.SeedOperator(t, apptest.Operator{UserID: "AO001", Password: "Rahasia#1", FullName: "Budi Santoso"})
	return h
}

// AC-FR-LOGIN-001-01, AC-FR-LOGIN-003-01
func TestDatabaseLoginWithValidCredentialsCreatesSession(t *testing.T) {
	h := seededHarness(t)
	c := h.NewClient()

	login := c.LoginDatabase(t, "AO001", "Rahasia#1")
	if login.Status != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body %s", login.Status, login.Raw)
	}

	me := c.Me(t)
	if me.Status != http.StatusOK {
		t.Fatalf("me status = %d, want 200; body %s", me.Status, me.Raw)
	}
	if me.Body["userId"] != "AO001" || me.Body["fullName"] != "Budi Santoso" {
		t.Fatalf("me body = %v", me.Body)
	}
}

// AC-FR-LOGIN-001-03, AC-FR-LOGIN-003-02, AC-FR-LOGIN-005-02
func TestDatabaseLoginWithInvalidCredentialsIsRefusedWithOneMessage(t *testing.T) {
	h := seededHarness(t)
	cases := map[string][2]string{
		"wrong password": {"AO001", "salah"},
		"unknown user":   {"TIDAKADA", "Rahasia#1"},
	}
	for name, cred := range cases {
		t.Run(name, func(t *testing.T) {
			c := h.NewClient()
			login := c.LoginDatabase(t, cred[0], cred[1])
			if login.Status != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", login.Status)
			}
			if login.Body["code"] != "ERR-001" || login.Body["message"] != errERR001 {
				t.Fatalf("body = %v, want ERR-001 %q", login.Body, errERR001)
			}
			if len(login.Header.Values("Set-Cookie")) != 0 {
				t.Fatalf("a cookie was set on failed login")
			}
			if me := c.Me(t); me.Status != http.StatusUnauthorized {
				t.Fatalf("session exists after failed login: %d", me.Status)
			}
		})
	}
}

// Usernames match case-insensitively (TBD-012, agreed).
func TestDatabaseLoginUsernameIsCaseInsensitive(t *testing.T) {
	h := seededHarness(t)
	c := h.NewClient()
	if login := c.LoginDatabase(t, "ao001", "Rahasia#1"); login.Status != http.StatusOK {
		t.Fatalf("status = %d, want 200", login.Status)
	}
	if me := c.Me(t); me.Body["userId"] != "AO001" {
		t.Fatalf("me = %v", me.Body)
	}
}

// FR-LOGIN-004 rule 3
func TestLoginOverPlainHTTPIsRefused(t *testing.T) {
	h := seededHarness(t)
	c := h.NewPlainHTTPClient()
	login := c.LoginDatabase(t, "AO001", "Rahasia#1")
	if login.Status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", login.Status)
	}
	if len(login.Header.Values("Set-Cookie")) != 0 {
		t.Fatalf("a cookie was set over plain HTTP")
	}
	if login.Raw != "" {
		t.Fatalf("body = %q, want none: the request is not processed", login.Raw)
	}
	if rows := loginLog(t, h); len(rows) != 0 {
		t.Fatalf("login_log rows = %d, want 0 for an unprocessed request", len(rows))
	}
}

// DB-RULE-071: every attempt that reaches the server is logged, including
// one the client should have stopped for an empty field.
func TestEmptyCredentialsReachingTheServerAreRefusedAndLogged(t *testing.T) {
	h := seededHarness(t)
	login := h.NewClient().LoginDatabase(t, "AO001", "")
	if login.Status != http.StatusUnauthorized || login.Body["code"] != "ERR-001" {
		t.Fatalf("status %d body %v, want 401 ERR-001", login.Status, login.Body)
	}
	rows := loginLog(t, h)
	if len(rows) != 1 || rows[0].Result != "FAIL" || deref(rows[0].ErrorCode) != "ERR-001" {
		t.Fatalf("login_log = %+v, want one FAIL row with ERR-001", rows)
	}
}

// AC-FR-LOGIN-004-01: the password never comes back in a response.
func TestPasswordIsNeverEchoedInResponses(t *testing.T) {
	h := seededHarness(t)
	for _, pw := range []string{"Rahasia#1", "SalahSekali#9"} {
		c := h.NewClient()
		login := c.LoginDatabase(t, "AO001", pw)
		if strings.Contains(login.Raw, pw) || strings.Contains(strings.Join(login.Header.Values("Set-Cookie"), ";"), pw) {
			t.Fatalf("password %q echoed in response", pw)
		}
	}
}

// FR-LOGIN-001 exception ERR-003
func TestDatabaseUnavailableGivesERR003(t *testing.T) {
	h := seededHarness(t)
	h.DB.Close()
	login := h.NewClient().LoginDatabase(t, "AO001", "Rahasia#1")
	if login.Status != http.StatusServiceUnavailable || login.Body["code"] != "ERR-003" {
		t.Fatalf("status %d body %v, want 503 ERR-003", login.Status, login.Body)
	}
}

type loginLogRow struct {
	LoginID, AuthMethod, Result                   string
	OperatorUserID, FailReason, ErrorCode, IP, UA *string
}

// login_log has no read API yet (TBD-047), so the audit table itself is the
// interface under test for AC-FR-LOGIN-006.
func loginLog(t *testing.T, h *apptest.Harness) []loginLogRow {
	t.Helper()
	rows, err := h.DB.Query(context.Background(), `
		SELECT login_id, auth_method, result, operator_user_id, fail_reason, error_code, client_ip, user_agent
		FROM fos.login_log ORDER BY login_log_id`)
	if err != nil {
		t.Fatalf("query login_log: %v", err)
	}
	defer rows.Close()
	var out []loginLogRow
	for rows.Next() {
		var r loginLogRow
		if err := rows.Scan(&r.LoginID, &r.AuthMethod, &r.Result, &r.OperatorUserID, &r.FailReason, &r.ErrorCode, &r.IP, &r.UA); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, r)
	}
	return out
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// AC-FR-LOGIN-006-01
func TestSuccessfulLoginWritesOneLoginLogRow(t *testing.T) {
	h := seededHarness(t)
	h.NewClient().LoginDatabase(t, "ao001", "Rahasia#1")

	rows := loginLog(t, h)
	if len(rows) != 1 {
		t.Fatalf("login_log rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.LoginID != "ao001" || r.AuthMethod != "DATABASE" || r.Result != "SUCCESS" ||
		deref(r.OperatorUserID) != "AO001" || r.FailReason != nil || r.ErrorCode != nil {
		t.Fatalf("row = %+v (operator %s)", r, deref(r.OperatorUserID))
	}
	if deref(r.UA) != "apptest-browser" || deref(r.IP) != "127.0.0.1" {
		t.Fatalf("client_ip %s user_agent %s", deref(r.IP), deref(r.UA))
	}
}

// AC-FR-LOGIN-006-02
func TestFailedLoginWritesOneLoginLogRow(t *testing.T) {
	h := seededHarness(t)
	h.NewClient().LoginDatabase(t, "AO001", "salah")
	h.NewClient().LoginDatabase(t, "Tidak Dikenal", "Rahasia#1")

	rows := loginLog(t, h)
	if len(rows) != 2 {
		t.Fatalf("login_log rows = %d, want 2", len(rows))
	}
	want := []struct{ loginID, operator string }{{"AO001", "AO001"}, {"Tidak Dikenal", "<nil>"}}
	for i, r := range rows {
		if r.LoginID != want[i].loginID || deref(r.OperatorUserID) != want[i].operator ||
			r.Result != "FAIL" || deref(r.FailReason) != "INVALID_CREDENTIAL" || deref(r.ErrorCode) != "ERR-001" {
			t.Fatalf("row %d = %+v (operator %s, reason %s, code %s)", i, r,
				deref(r.OperatorUserID), deref(r.FailReason), deref(r.ErrorCode))
		}
	}
}

// AC-FR-LOGIN-006-03
func TestLoginLogNeverContainsThePassword(t *testing.T) {
	h := seededHarness(t)
	h.NewClient().LoginDatabase(t, "AO001", "Rahasia#1")
	h.NewClient().LoginDatabase(t, "AO001", "SalahSekali#9")

	var leaks int
	if err := h.DB.QueryRow(context.Background(), `
		SELECT count(*) FROM fos.login_log l
		WHERE l::text LIKE '%Rahasia#1%' OR l::text LIKE '%SalahSekali#9%'`).Scan(&leaks); err != nil {
		t.Fatalf("query: %v", err)
	}
	if leaks != 0 {
		t.Fatalf("%d login_log rows contain a password", leaks)
	}
}

// DB-RULE-068
func TestClientIPComesFromForwardedForBehindTrustedProxy(t *testing.T) {
	h := seededHarness(t)
	c := h.NewClient()
	c.Header.Set("X-Forwarded-For", "10.20.30.40, 127.0.0.1")
	c.LoginDatabase(t, "AO001", "Rahasia#1")

	if rows := loginLog(t, h); deref(rows[0].IP) != "10.20.30.40" {
		t.Fatalf("client_ip = %s, want 10.20.30.40", deref(rows[0].IP))
	}
}
