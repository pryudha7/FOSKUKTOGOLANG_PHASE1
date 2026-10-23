package app_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"fos/internal/apptest"
)

func loggedIn(t *testing.T, h *apptest.Harness, op apptest.Operator) *apptest.Client {
	t.Helper()
	if op.Password == "" {
		op.Password = "Rahasia#1"
	}
	h.SeedOperator(t, op)
	c := h.NewClient()
	if login := c.LoginDatabase(t, op.UserID, op.Password); login.Status != http.StatusOK {
		t.Fatalf("login %s: %d %s", op.UserID, login.Status, login.Raw)
	}
	return c
}

func field(t *testing.T, r apptest.Response, path ...string) any {
	t.Helper()
	var v any = r.Body
	for _, p := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[p]
	}
	return v
}

// AC-FR-ACCESS-002-01: the seeded BM mapping (SoT §3.6) resolves to access
// group User and work group Committee; its work queue is TBD, so none.
func TestRoleMappingResolvesAccessContext(t *testing.T) {
	h := apptest.New(t)
	me := loggedIn(t, h, apptest.Operator{UserID: "BM001", RoleCode: "BM"}).Me(t)

	if got := field(t, me, "accessGroup", "name"); got != "User" {
		t.Fatalf("accessGroup = %v, want User; body %s", got, me.Raw)
	}
	if got := field(t, me, "workGroup", "name"); got != "Committee" {
		t.Fatalf("workGroup = %v, want Committee", got)
	}
	if got := field(t, me, "workQueue"); got != nil {
		t.Fatalf("workQueue = %v, want none", got)
	}
	if got := field(t, me, "role", "name"); got != "BM" {
		t.Fatalf("role = %v, want BM", got)
	}
}

// AC-FR-ACCESS-002-03
func TestRoleWithoutActiveMappingGetsNoDefault(t *testing.T) {
	h := apptest.New(t)
	h.SeedMapping(t, apptest.Mapping{Role: "Pending Role", AccessGroup: "User", WorkGroup: "Default", Inactive: true})
	for _, role := range []string{"AO", "Pending Role"} {
		me := loggedIn(t, h, apptest.Operator{UserID: "OP-" + role, RoleCode: role}).Me(t)
		for _, k := range []string{"accessGroup", "workGroup", "workQueue"} {
			if got := field(t, me, k); got != nil {
				t.Fatalf("role %s: %s = %v, want none", role, k, got)
			}
		}
	}
}

// AC-FR-ACCESS-002-02: master data added in the database is used with no
// code change; a mapping pointing at an inactive master is not.
func TestNewMasterDataTakesEffectWithoutCodeChange(t *testing.T) {
	h := apptest.New(t)
	h.SeedMapping(t, apptest.Mapping{Role: "Pilot Role", AccessGroup: "Admin", WorkGroup: "Pilot Group", WorkQueue: "Pilot Queue"})
	me := loggedIn(t, h, apptest.Operator{UserID: "PILOT1", RoleCode: "Pilot Role"}).Me(t)
	if field(t, me, "accessGroup", "name") != "Admin" || field(t, me, "workGroup", "name") != "Pilot Group" ||
		field(t, me, "workQueue", "name") != "Pilot Queue" {
		t.Fatalf("me = %s", me.Raw)
	}

	if _, err := h.DB.Exec(context.Background(),
		`UPDATE fos.work_groups SET is_active = FALSE WHERE work_group_code = 'Pilot Group'`); err != nil {
		t.Fatal(err)
	}
	me = loggedIn(t, h, apptest.Operator{UserID: "PILOT2", RoleCode: "Pilot Role"}).Me(t)
	if got := field(t, me, "accessGroup"); got != nil {
		t.Fatalf("mapping to an inactive work group still applied: %s", me.Raw)
	}
}

// AC-FR-ACCESS-003-01
func TestMobileLoginWithMobileAccessCreatesMobileSession(t *testing.T) {
	h := apptest.New(t)
	h.SeedOperator(t, apptest.Operator{UserID: "AO010", Password: "Rahasia#1", MobileAccess: true})
	c := h.NewClient()
	if login := c.LoginMobileDatabase(t, "AO010", "Rahasia#1"); login.Status != http.StatusOK {
		t.Fatalf("mobile login = %d %s", login.Status, login.Raw)
	}
	if me := c.MeAt(t, apptest.EndpointPathMobile); me.Status != http.StatusOK || me.Body["userId"] != "AO010" {
		t.Fatalf("mobile me = %d %s", me.Status, me.Raw)
	}
	if rows := loginLog(t, h); len(rows) != 1 || rows[0].Result != "SUCCESS" {
		t.Fatalf("login_log = %+v", rows)
	}
}

// AC-FR-ACCESS-003-02
func TestMobileLoginWithoutMobileAccessIsRefusedWithERR004(t *testing.T) {
	h := apptest.New(t)
	h.SeedOperator(t, apptest.Operator{UserID: "AO011", Password: "Rahasia#1"})
	c := h.NewClient()

	login := c.LoginMobileDatabase(t, "AO011", "Rahasia#1")
	assertRefused(t, login, http.StatusForbidden, "ERR-004", "Anda tidak memiliki akses ke aplikasi mobile.")
	if me := c.MeAt(t, apptest.EndpointPathMobile); me.Status != http.StatusUnauthorized {
		t.Fatalf("mobile session exists: %d", me.Status)
	}
	rows := loginLog(t, h)
	if len(rows) != 1 || rows[0].Result != "FAIL" || deref(rows[0].ErrorCode) != "ERR-004" || deref(rows[0].OperatorUserID) != "AO011" {
		t.Fatalf("login_log = %+v", rows)
	}
}

// AC-FR-ACCESS-003-04
func TestDesktopLoginIgnoresMobileAccess(t *testing.T) {
	h := apptest.New(t)
	if me := loggedIn(t, h, apptest.Operator{UserID: "AO012"}).Me(t); me.Status != http.StatusOK {
		t.Fatalf("desktop me = %d", me.Status)
	}
}

// AC-FR-ACCESS-004-01, AC-FR-ACCESS-004-02: a session is valid only on the
// endpoint it was created on, even if its token is replayed on the other.
func TestSessionsAreIsolatedPerEndpoint(t *testing.T) {
	h := apptest.New(t)
	h.SeedOperator(t, apptest.Operator{UserID: "AO013", Password: "Rahasia#1", MobileAccess: true})

	mobile := h.NewClient()
	mobile.LoginMobileDatabase(t, "AO013", "Rahasia#1")
	if me := mobile.Me(t); me.Status != http.StatusUnauthorized {
		t.Fatalf("mobile session accepted on desktop: %d", me.Status)
	}
	token := mobile.SessionCookie(apptest.EndpointPathMobile)
	if token == "" {
		t.Fatalf("no mobile session cookie")
	}
	if r := h.DoWithSessionCookie(t, http.MethodGet, apptest.EndpointPathDesktop+"/api/me", token); r.Status != http.StatusUnauthorized {
		t.Fatalf("mobile token replayed on desktop: %d", r.Status)
	}

	desktop := h.NewClient()
	desktop.LoginDatabase(t, "AO013", "Rahasia#1")
	if me := desktop.MeAt(t, apptest.EndpointPathMobile); me.Status != http.StatusUnauthorized {
		t.Fatalf("desktop session accepted on mobile: %d", me.Status)
	}
	token = desktop.SessionCookie(apptest.EndpointPathDesktop)
	if r := h.DoWithSessionCookie(t, http.MethodGet, apptest.EndpointPathMobile+"/api/me", token); r.Status != http.StatusUnauthorized {
		t.Fatalf("desktop token replayed on mobile: %d", r.Status)
	}
}

// AC-FR-ACCESS-005-03, AC-FR-ACCESS-005-04: the server ends an idle desktop
// session at 15 minutes, whatever the browser does.
func TestIdleDesktopSessionEndsAtFifteenMinutes(t *testing.T) {
	h := apptest.New(t)
	c := loggedIn(t, h, apptest.Operator{UserID: "AO014"})
	// The server states the FR-ACCESS-005 timings, so the UI holds no copy.
	me := c.Me(t)
	if field(t, me, "idle", "warnAfterSeconds") != float64(14*60) || field(t, me, "idle", "logoffAfterSeconds") != float64(15*60) {
		t.Fatalf("idle = %v, want 840/900 seconds", field(t, me, "idle"))
	}

	h.Clock.Advance(15*time.Minute - time.Second)
	if me := c.Me(t); me.Status != http.StatusOK {
		t.Fatalf("session ended before 15 minutes idle: %d", me.Status)
	}
	h.Clock.Advance(15*time.Minute + time.Second)
	if me := c.Me(t); me.Status != http.StatusUnauthorized {
		t.Fatalf("session still valid after 15 minutes idle: %d", me.Status)
	}
	h.Clock.Advance(-15 * time.Minute)
	if me := c.Me(t); me.Status != http.StatusUnauthorized {
		t.Fatalf("an ended session must stay ended: %d", me.Status)
	}
}

// AC-FR-ACCESS-005-06: a request resets the idle timer.
func TestRequestResetsIdleTimer(t *testing.T) {
	h := apptest.New(t)
	c := loggedIn(t, h, apptest.Operator{UserID: "AO015"})

	h.Clock.Advance(10 * time.Minute)
	c.Me(t)
	h.Clock.Advance(14*time.Minute + 59*time.Second)
	if me := c.Me(t); me.Status != http.StatusOK {
		t.Fatalf("timer was not reset by the request at minute 10: %d", me.Status)
	}
}

// AC-FR-ACCESS-005-05: no idle timeout for a role named "Identity
// Management…" or an Authors/Admin access group.
func TestIdleTimeoutExemptions(t *testing.T) {
	h := apptest.New(t)
	h.SeedMapping(t, apptest.Mapping{Role: "Ops Authors", AccessGroup: "Authors", WorkGroup: "Default"})
	h.SeedMapping(t, apptest.Mapping{Role: "Ops Admin", AccessGroup: "Admin", WorkGroup: "Default"})
	// Rule 3b names the access group; a code that differs from the name is
	// irrelevant.
	if _, err := h.DB.Exec(context.Background(), `INSERT INTO fos.access_groups
		(access_group_code, access_group_name, is_active, created_at, updated_at)
		VALUES ('AG-AUTHORS-2', 'Authors', TRUE, now(), now())`); err != nil {
		t.Fatal(err)
	}
	h.SeedMapping(t, apptest.Mapping{Role: "Ops Authors 2", AccessGroup: "AG-AUTHORS-2", WorkGroup: "Default"})
	exempt := map[string]*apptest.Client{
		"Identity Management SectHead": loggedIn(t, h, apptest.Operator{UserID: "IM01", RoleCode: "Identity Management SectHead"}),
		"Identity Management X":        loggedIn(t, h, apptest.Operator{UserID: "IM02", RoleCode: "Identity Management X"}),
		"Authors":                      loggedIn(t, h, apptest.Operator{UserID: "AU01", RoleCode: "Ops Authors"}),
		"Admin":                        loggedIn(t, h, apptest.Operator{UserID: "AD01", RoleCode: "Ops Admin"}),
		"Authors, other code":          loggedIn(t, h, apptest.Operator{UserID: "AU02", RoleCode: "Ops Authors 2"}),
	}
	// Only the role-name prefix counts, not a name that merely contains it.
	ordinary := loggedIn(t, h, apptest.Operator{UserID: "OP01", RoleCode: "Former Identity Management"})

	for name, c := range exempt {
		if got := field(t, c.Me(t), "idle"); got != nil {
			t.Fatalf("%s: idle = %v, want none", name, got)
		}
	}
	h.Clock.Advance(2 * time.Hour)
	for name, c := range exempt {
		if me := c.Me(t); me.Status != http.StatusOK {
			t.Fatalf("%s was logged off: %d", name, me.Status)
		}
	}
	if me := ordinary.Me(t); me.Status != http.StatusUnauthorized {
		t.Fatalf("non-exempt operator kept the session: %d", me.Status)
	}
}

// FR-ACCESS-005 rule 4: idle timeout applies on desktop; mobile is TBD-017,
// so mobile sessions are not timed out.
func TestMobileSessionHasNoIdleTimeout(t *testing.T) {
	h := apptest.New(t)
	h.SeedOperator(t, apptest.Operator{UserID: "AO016", Password: "Rahasia#1", MobileAccess: true})
	c := h.NewClient()
	c.LoginMobileDatabase(t, "AO016", "Rahasia#1")
	h.Clock.Advance(time.Hour)
	if me := c.MeAt(t, apptest.EndpointPathMobile); me.Status != http.StatusOK {
		t.Fatalf("mobile session timed out: %d", me.Status)
	}
}

// FR-ACCESS-005 step 4: logging off ends the session on the server, so the
// browser's Back button cannot revive it.
func TestLogoutEndsTheSessionOnTheServer(t *testing.T) {
	h := apptest.New(t)
	c := loggedIn(t, h, apptest.Operator{UserID: "AO019"})
	token := c.SessionCookie(apptest.EndpointPathDesktop)

	if r := c.Logout(t, apptest.EndpointPathDesktop); r.Status != http.StatusNoContent {
		t.Fatalf("logout = %d, want 204", r.Status)
	}
	if me := c.Me(t); me.Status != http.StatusUnauthorized {
		t.Fatalf("session alive after logout: %d", me.Status)
	}
	if r := h.DoWithSessionCookie(t, http.MethodGet, apptest.EndpointPathDesktop+"/api/me", token); r.Status != http.StatusUnauthorized {
		t.Fatalf("logged-out token still accepted: %d", r.Status)
	}
	if r := c.Logout(t, apptest.EndpointPathDesktop); r.Status != http.StatusNoContent {
		t.Fatalf("second logout = %d, want 204", r.Status)
	}
}

// A logout on one endpoint leaves the operator's session on the other alone.
func TestLogoutOnlyEndsTheSessionOfItsEndpoint(t *testing.T) {
	h := apptest.New(t)
	h.SeedOperator(t, apptest.Operator{UserID: "AO020", Password: "Rahasia#1", MobileAccess: true})
	c := h.NewClient()
	c.LoginDatabase(t, "AO020", "Rahasia#1")
	c.LoginMobileDatabase(t, "AO020", "Rahasia#1")
	mobileToken := c.SessionCookie(apptest.EndpointPathMobile)

	if r := h.DoWithSessionCookie(t, http.MethodPost, apptest.EndpointPathDesktop+"/api/logout", mobileToken); r.Status != http.StatusNoContent {
		t.Fatalf("logout = %d", r.Status)
	}
	if me := c.MeAt(t, apptest.EndpointPathMobile); me.Status != http.StatusOK {
		t.Fatalf("mobile session ended by a desktop logout: %d", me.Status)
	}
}

// /api/me tells the UI which login page to return to (agreed for TBD-013).
func TestMeReportsTheLoginPathOfTheSession(t *testing.T) {
	h := apptest.New(t)
	if got := field(t, loggedIn(t, h, apptest.Operator{UserID: "AO017"}).Me(t), "loginPath"); got != apptest.LoginPathDB {
		t.Fatalf("desktop loginPath = %v", got)
	}
	h.SeedOperator(t, apptest.Operator{UserID: "AO018", Password: "Rahasia#1", MobileAccess: true})
	c := h.NewClient()
	c.LoginMobileDatabase(t, "AO018", "Rahasia#1")
	if got := field(t, c.MeAt(t, apptest.EndpointPathMobile), "loginPath"); got != apptest.EndpointPathMobile+apptest.LoginPathDB {
		t.Fatalf("mobile loginPath = %v", got)
	}
}
