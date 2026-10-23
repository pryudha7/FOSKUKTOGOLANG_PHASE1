package app_test

import (
	"net/http"
	"strings"
	"testing"

	"fos/internal/apptest"
	"fos/internal/config"
)

// ldapHarness runs the app against a fake directory holding user ao002, who
// is also a registered FOS operator.
func ldapHarness(t *testing.T) (*apptest.Harness, *apptest.FakeLDAP) {
	t.Helper()
	dir := apptest.StartFakeLDAP(t)
	dir.AddUser("ao002", "Ldap#Pass1")
	h := apptest.New(t, apptest.WithLDAP(dir))
	h.SeedOperator(t, apptest.Operator{UserID: "AO002", Password: "db-only-password", FullName: "Siti Aminah"})
	return h, dir
}

// AC-FR-LOGIN-002-01
func TestLDAPLoginForRegisteredOperatorCreatesSession(t *testing.T) {
	h, _ := ldapHarness(t)
	c := h.NewClient()

	login := c.LoginLDAP(t, "ao002", "Ldap#Pass1")
	if login.Status != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body %s", login.Status, login.Raw)
	}
	if me := c.Me(t); me.Status != http.StatusOK || me.Body["userId"] != "AO002" {
		t.Fatalf("me = %d %v", me.Status, me.Body)
	}
	rows := loginLog(t, h)
	if len(rows) != 1 || rows[0].AuthMethod != "LDAP" || rows[0].Result != "SUCCESS" || deref(rows[0].OperatorUserID) != "AO002" {
		t.Fatalf("login_log = %+v", rows)
	}
}

// AC-FR-LOGIN-002-02
func TestLDAPLoginForUnregisteredUserIsRefusedWithERR001(t *testing.T) {
	h, dir := ldapHarness(t)
	dir.AddUser("tamu01", "Tamu#Pass1")

	login := h.NewClient().LoginLDAP(t, "tamu01", "Tamu#Pass1")
	assertRefused(t, login, http.StatusUnauthorized, "ERR-001", errERR001)
	rows := loginLog(t, h)
	if len(rows) != 1 || rows[0].AuthMethod != "LDAP" || deref(rows[0].FailReason) != "NOT_REGISTERED" {
		t.Fatalf("login_log = %+v (reason %s)", rows, deref(rows[0].FailReason))
	}
}

// AC-FR-LOGIN-002-03
func TestLDAPLoginWhenDirectoryIsUnreachableGivesERR002(t *testing.T) {
	h, dir := ldapHarness(t)
	logs := apptest.CaptureLogs(t)
	dir.Stop()

	login := h.NewClient().LoginLDAP(t, "ao002", "Ldap#Pass1")
	assertRefused(t, login, http.StatusServiceUnavailable, "ERR-002", errERR002)
	assertLDAPUnavailableLogged(t, h)
	assertNotLogged(t, logs, "Ldap#Pass1", dir.BindPassword)
}

// AC-FR-LOGIN-002-04
func TestLDAPFilterSpecialCharactersAreTreatedAsPlainText(t *testing.T) {
	h, dir := ldapHarness(t)

	login := h.NewClient().LoginLDAP(t, "*", "Ldap#Pass1")
	assertRefused(t, login, http.StatusUnauthorized, "ERR-001", errERR001)
	// The configured filter has no parentheses (SoT §4.2); they are added,
	// and * goes on the wire escaped, never as a wildcard.
	if got := dir.Filters(); len(got) != 1 || got[0] != `(SamAccountName=\2a)` {
		t.Fatalf(`filters sent = %q, want [(SamAccountName=\2a)]`, got)
	}
}

// FR-LOGIN-002 steps 3-4
func TestLDAPLoginWithWrongPasswordOrUnknownUserIsRefusedWithERR001(t *testing.T) {
	h, _ := ldapHarness(t)
	for name, cred := range map[string][2]string{
		"wrong password": {"ao002", "salah"},
		"unknown user":   {"tidakada", "Ldap#Pass1"},
		"empty password": {"ao002", ""},
	} {
		t.Run(name, func(t *testing.T) {
			c := h.NewClient()
			assertRefused(t, c.LoginLDAP(t, cred[0], cred[1]), http.StatusUnauthorized, "ERR-001", errERR001)
			if me := c.Me(t); me.Status != http.StatusUnauthorized {
				t.Fatalf("session exists after refused login")
			}
		})
	}
}

// FR-LOGIN-002 step 4: any refusal of the user bind is ERR-001, so the
// operator cannot tell a locked account from a wrong password.
func TestLDAPLoginForLockedAccountIsRefusedWithERR001(t *testing.T) {
	h, dir := ldapHarness(t)
	dir.LockUser("ao002")

	login := h.NewClient().LoginLDAP(t, "ao002", "Ldap#Pass1")
	assertRefused(t, login, http.StatusUnauthorized, "ERR-001", errERR001)
	if rows := loginLog(t, h); len(rows) != 1 || deref(rows[0].FailReason) != "INVALID_CREDENTIAL" {
		t.Fatalf("login_log = %+v", rows)
	}
}

// ENV-008: a key left as the SoT placeholder "TBD", or an unusable timeout,
// counts as empty when the configuration is loaded from the environment.
func TestLDAPLoginWithPlaceholderConfigurationGivesERR002(t *testing.T) {
	for name, env := range map[string][2]string{
		"base DN is TBD":  {"LDAP_BASE_DN", "TBD"},
		"timeout invalid": {"LDAP_TIMEOUT_SECONDS", "sepuluh"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := apptest.StartFakeLDAP(t)
			dir.AddUser("ao002", "Ldap#Pass1")
			loaded := loadConfigFromEnv(t, dir, env[0], env[1])
			h := apptest.New(t, func(c *config.Config) { c.LDAP = loaded.LDAP })

			login := h.NewClient().LoginLDAP(t, "ao002", "Ldap#Pass1")
			assertRefused(t, login, http.StatusServiceUnavailable, "ERR-002", errERR002)
			if f := dir.Filters(); len(f) != 0 {
				t.Fatalf("LDAP was searched: %q", f)
			}
		})
	}
}

// loadConfigFromEnv runs config.FromEnv with a complete environment pointing
// at dir, except that key is set to value.
func loadConfigFromEnv(t *testing.T, dir *apptest.FakeLDAP, key, value string) config.Config {
	t.Helper()
	env := map[string]string{
		"APP_ENV": "test", "LISTEN_ADDR": ":0", "DATABASE_URL": "unused",
		"LOGIN_PATH_DB": apptest.LoginPathDB, "LOGIN_PATH_LDAP": apptest.LoginPathLDAP,
		"ENDPOINT_PATH_DESKTOP": apptest.EndpointPathDesktop, "TRUSTED_PROXY_CIDRS": "127.0.0.1/32",
		"LDAP_URL": dir.URL, "LDAP_BIND_DN": dir.BindDN, "LDAP_BIND_PASSWORD": dir.BindPassword,
		"LDAP_BASE_DN": dir.BaseDN, "LDAP_SEARCH_FILTER": "SamAccountName=%v", "LDAP_TIMEOUT_SECONDS": "2",
	}
	env[key] = value
	for k, v := range env {
		t.Setenv(k, v)
	}
	cfg, err := config.FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	return cfg
}

// Usernames match the FOS operator case-insensitively (agreed).
func TestLDAPLoginMatchesOperatorCaseInsensitively(t *testing.T) {
	h, dir := ldapHarness(t)
	dir.AddUser("AO003", "Ldap#Pass3")
	h.SeedOperator(t, apptest.Operator{UserID: "ao003", Password: "x"})
	if login := h.NewClient().LoginLDAP(t, "AO003", "Ldap#Pass3"); login.Status != http.StatusOK {
		t.Fatalf("status = %d, want 200", login.Status)
	}
}

// FR-LOGIN-002 exception: the service account bind fails.
func TestLDAPServiceBindFailureGivesERR002(t *testing.T) {
	dir := apptest.StartFakeLDAP(t)
	dir.AddUser("ao002", "Ldap#Pass1")
	logs := apptest.CaptureLogs(t)
	h := apptest.New(t, apptest.WithLDAP(dir), func(c *config.Config) { c.LDAP.BindPassword = "wrong-svc-secret" })

	login := h.NewClient().LoginLDAP(t, "ao002", "Ldap#Pass1")
	assertRefused(t, login, http.StatusServiceUnavailable, "ERR-002", errERR002)
	assertLDAPUnavailableLogged(t, h)
	assertNotLogged(t, logs, "Ldap#Pass1", "wrong-svc-secret")
}

// ENV-008: an empty LDAP key refuses LDAP login without contacting LDAP.
func TestLDAPLoginWithIncompleteConfigurationGivesERR002(t *testing.T) {
	dir := apptest.StartFakeLDAP(t)
	dir.AddUser("ao002", "Ldap#Pass1")
	h := apptest.New(t, apptest.WithLDAP(dir), func(c *config.Config) { c.LDAP.BaseDN = "" })

	login := h.NewClient().LoginLDAP(t, "ao002", "Ldap#Pass1")
	assertRefused(t, login, http.StatusServiceUnavailable, "ERR-002", errERR002)
	assertLDAPUnavailableLogged(t, h)
	if f := dir.Filters(); len(f) != 0 {
		t.Fatalf("LDAP was searched with incomplete configuration: %q", f)
	}
	// The Database path is unaffected.
	h.SeedOperator(t, apptest.Operator{UserID: "AO009", Password: "Rahasia#9"})
	if login := h.NewClient().LoginDatabase(t, "AO009", "Rahasia#9"); login.Status != http.StatusOK {
		t.Fatalf("database login status = %d, want 200", login.Status)
	}
}

const errERR002 = "Layanan autentikasi sedang tidak tersedia. Silakan coba beberapa saat lagi."

func assertRefused(t *testing.T, r apptest.Response, status int, code, message string) {
	t.Helper()
	if r.Status != status || r.Body["code"] != code || r.Body["message"] != message {
		t.Fatalf("got %d %v, want %d %s %q", r.Status, r.Body, status, code, message)
	}
	if len(r.Header.Values("Set-Cookie")) != 0 {
		t.Fatalf("a cookie was set on a refused login")
	}
}

func assertLDAPUnavailableLogged(t *testing.T, h *apptest.Harness) {
	t.Helper()
	rows := loginLog(t, h)
	if len(rows) != 1 || rows[0].AuthMethod != "LDAP" || rows[0].Result != "FAIL" ||
		deref(rows[0].FailReason) != "LDAP_UNAVAILABLE" || deref(rows[0].ErrorCode) != "ERR-002" {
		t.Fatalf("login_log = %+v", rows)
	}
}

func assertNotLogged(t *testing.T, logs *apptest.LogBuffer, secrets ...string) {
	t.Helper()
	out := logs.String()
	if out == "" {
		t.Fatalf("expected the failure to be logged")
	}
	for _, s := range secrets {
		if strings.Contains(out, s) {
			t.Fatalf("log contains secret %q:\n%s", s, out)
		}
	}
}
