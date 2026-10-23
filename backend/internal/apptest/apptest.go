// Package apptest runs the FOS backend against a real PostgreSQL for tests at
// the HTTP API seam. External systems are faked by pointing configuration at
// httptest servers owned by the test.
package apptest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"

	"fos/internal/app"
	"fos/internal/config"
	"fos/internal/migrate"
)

const (
	LoginPathDB         = "/PRWebDB1"
	LoginPathLDAP       = "/PRWebLDAP1"
	EndpointPathDesktop = "/app/fossyariah"
	EndpointPathMobile  = "/app/fossyariah-mobile"
)

var (
	containerOnce sync.Once
	containerDSN  func(db string) string
	containerErr  error
	dbCounter     atomic.Int64
)

// startContainer starts one PostgreSQL container per test binary.
func startContainer() {
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("postgres"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		containerErr = err
		return
	}
	host, err := c.Host(ctx)
	if err != nil {
		containerErr = err
		return
	}
	port, err := c.MappedPort(ctx, "5432/tcp")
	if err != nil {
		containerErr = err
		return
	}
	containerDSN = func(db string) string {
		return fmt.Sprintf("postgres://postgres:postgres@%s:%s/%s?sslmode=disable", host, port.Port(), db)
	}
}

// Harness is one running backend with its own fresh, migrated database.
type Harness struct {
	Server *httptest.Server
	DB     *pgxpool.Pool
	Config config.Config
	Clock  *FakeClock
}

// New starts the backend on a fresh database. Requests made through the
// harness arrive as HTTPS forwarded by a trusted proxy unless stated otherwise.
// Each configure function may adjust the configuration before the app starts,
// e.g. to point an integration URL at a fake server:
//
//	fake := httptest.NewServer(handler)
//	t.Cleanup(fake.Close)
//	h := apptest.New(t, func(c *config.Config) { c.SomeAPIURL = fake.URL })
func New(t *testing.T, configure ...func(*config.Config)) *Harness {
	t.Helper()
	containerOnce.Do(startContainer)
	if containerErr != nil {
		t.Fatalf("start postgres container: %v", containerErr)
	}
	ctx := context.Background()

	name := fmt.Sprintf("fos_test_%d", dbCounter.Add(1))
	admin, err := pgx.Connect(ctx, containerDSN("postgres"))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	admin.Close(ctx)

	cfg := config.Config{
		Env:                 "test",
		DatabaseURL:         containerDSN(name),
		LoginPathDB:         LoginPathDB,
		LoginPathLDAP:       LoginPathLDAP,
		EndpointPathDesktop: EndpointPathDesktop,
		EndpointPathMobile:  EndpointPathMobile,
		TrustedProxyCIDRs:   []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")},
	}
	for _, f := range configure {
		f(&cfg)
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	if err := migrate.Up(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clock := &FakeClock{now: time.Now().UTC()}
	srv := httptest.NewServer(app.New(cfg, pool, app.WithClock(clock.Now)))
	t.Cleanup(func() {
		srv.Close()
		pool.Close()
	})
	return &Harness{Server: srv, DB: pool, Config: cfg, Clock: clock}
}

// FakeClock is the app's clock in tests; Advance moves time forward.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// Operator is a seeded FOS operator.
type Operator struct {
	UserID   string
	Password string
	FullName string
	RoleCode string
	// MobileAccess is the operator's mobile access checklist (FR-ACCESS-003).
	MobileAccess bool
}

// SeedOperator inserts an operator (and its role, if missing) directly; there
// is no administration API for operators yet.
func (h *Harness) SeedOperator(t *testing.T, op Operator) {
	t.Helper()
	ctx := context.Background()
	if op.RoleCode == "" {
		op.RoleCode = "AO"
	}
	if op.FullName == "" {
		op.FullName = op.UserID
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(op.Password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	_, err = h.DB.Exec(ctx, `
		INSERT INTO fos.roles (role_code, role_name, is_active, created_at, updated_at)
		VALUES ($1, $1, TRUE, now() AT TIME ZONE 'UTC', now() AT TIME ZONE 'UTC')
		ON CONFLICT (role_code) DO NOTHING`, op.RoleCode)
	if err != nil {
		t.Fatalf("seed role: %v", err)
	}
	_, err = h.DB.Exec(ctx, `
		INSERT INTO fos.operator (user_id, password_hash, full_name, role_code, mobile_access, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, now() AT TIME ZONE 'UTC', now() AT TIME ZONE 'UTC')`,
		op.UserID, string(hash), op.FullName, op.RoleCode, op.MobileAccess)
	if err != nil {
		t.Fatalf("seed operator: %v", err)
	}
}

// Mapping is a role_access_mappings row to seed. Master rows that do not yet
// exist are created active; WorkQueue may be empty.
type Mapping struct {
	Role, AccessGroup, WorkGroup, WorkQueue string
	Inactive                                bool
}

// SeedMapping inserts master data and a mapping directly, the way master data
// is maintained until UI-007 exists (no code change, AC-FR-ACCESS-002-02).
func (h *Harness) SeedMapping(t *testing.T, m Mapping) {
	t.Helper()
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := h.DB.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed mapping: %v", err)
		}
	}
	const now = "now() AT TIME ZONE 'UTC'"
	exec(`INSERT INTO fos.roles (role_code, role_name, is_active, created_at, updated_at)
		VALUES ($1, $1, TRUE, `+now+`, `+now+`) ON CONFLICT (role_code) DO NOTHING`, m.Role)
	exec(`INSERT INTO fos.access_groups (access_group_code, access_group_name, is_active, created_at, updated_at)
		VALUES ($1, $1, TRUE, `+now+`, `+now+`) ON CONFLICT (access_group_code) DO NOTHING`, m.AccessGroup)
	exec(`INSERT INTO fos.work_groups (work_group_code, work_group_name, is_active, created_at, updated_at)
		VALUES ($1, $1, TRUE, `+now+`, `+now+`) ON CONFLICT (work_group_code) DO NOTHING`, m.WorkGroup)
	if m.WorkQueue != "" {
		exec(`INSERT INTO fos.work_queues (work_queue_code, work_queue_name, is_active, created_at, updated_at)
			VALUES ($1, $1, TRUE, `+now+`, `+now+`) ON CONFLICT (work_queue_code) DO NOTHING`, m.WorkQueue)
	}
	exec(`INSERT INTO fos.role_access_mappings (role_id, access_group_id, work_group_id, work_queue_id, is_active, created_at, updated_at)
		SELECT r.role_id, ag.access_group_id, wg.work_group_id,
			(SELECT work_queue_id FROM fos.work_queues WHERE work_queue_code = NULLIF($4, '')), $5, `+now+`, `+now+`
		FROM fos.roles r, fos.access_groups ag, fos.work_groups wg
		WHERE r.role_code = $1 AND ag.access_group_code = $2 AND wg.work_group_code = $3`,
		m.Role, m.AccessGroup, m.WorkGroup, m.WorkQueue, !m.Inactive)
}

// Client is a browser-like client with its own cookie jar.
type Client struct {
	// Header is added to every request, e.g. X-Forwarded-For.
	Header   http.Header
	h        *Harness
	http     *http.Client
	insecure bool
}

// NewClient returns a client whose requests arrive over HTTPS via the proxy.
func (h *Harness) NewClient() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{Header: http.Header{}, h: h, http: &http.Client{Jar: httpsJar{jar}}}
}

// httpsJar stores and sends cookies as if the server URL were https: in
// production TLS ends at the proxy, so Secure cookies do reach the backend.
type httpsJar struct{ http.CookieJar }

func asHTTPS(u *url.URL) *url.URL {
	c := *u
	c.Scheme = "https"
	return &c
}

func (j httpsJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.CookieJar.SetCookies(asHTTPS(u), cookies)
}

func (j httpsJar) Cookies(u *url.URL) []*http.Cookie { return j.CookieJar.Cookies(asHTTPS(u)) }

// NewPlainHTTPClient returns a client whose requests arrive without TLS.
func (h *Harness) NewPlainHTTPClient() *Client {
	c := h.NewClient()
	c.insecure = true
	return c
}

// Response is a decoded API response.
type Response struct {
	Status int
	Body   map[string]any
	Raw    string
	Header http.Header
}

// Do sends a JSON request to path (relative to the server root).
func (c *Client) Do(t *testing.T, method, path string, body any) Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encode body: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.h.Server.URL+path, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "apptest-browser")
	for k, v := range c.Header {
		req.Header[k] = v
	}
	if !c.insecure {
		req.Header.Set("X-Forwarded-Proto", "https")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := Response{Status: resp.StatusCode, Raw: string(raw), Header: resp.Header}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.Body)
	}
	return out
}

// LoginDatabase submits credentials to LOGIN_PATH_DB (API-002): username and
// password only.
func (c *Client) LoginDatabase(t *testing.T, username, password string) Response {
	t.Helper()
	return c.Do(t, http.MethodPost, LoginPathDB,
		map[string]string{"username": username, "password": password})
}

// LoginLDAP submits credentials to LOGIN_PATH_LDAP (API-001).
func (c *Client) LoginLDAP(t *testing.T, username, password string) Response {
	t.Helper()
	return c.Do(t, http.MethodPost, LoginPathLDAP,
		map[string]string{"username": username, "password": password})
}

// Me returns the operator of the current desktop session.
func (c *Client) Me(t *testing.T) Response {
	t.Helper()
	return c.MeAt(t, EndpointPathDesktop)
}

// MeAt returns the operator of the current session on an endpoint.
func (c *Client) MeAt(t *testing.T, endpointPath string) Response {
	t.Helper()
	return c.Do(t, http.MethodGet, endpointPath+"/api/me", nil)
}

// Logout ends the session on an endpoint.
func (c *Client) Logout(t *testing.T, endpointPath string) Response {
	t.Helper()
	return c.Do(t, http.MethodPost, endpointPath+"/api/logout", nil)
}

// LoginMobileDatabase submits Database-path credentials on the mobile
// endpoint (agreed: login paths nest under ENDPOINT_PATH_MOBILE).
func (c *Client) LoginMobileDatabase(t *testing.T, username, password string) Response {
	t.Helper()
	return c.Do(t, http.MethodPost, EndpointPathMobile+LoginPathDB,
		map[string]string{"username": username, "password": password})
}

// SessionCookie returns the session cookie the browser holds for an
// endpoint, or "".
func (c *Client) SessionCookie(endpointPath string) string {
	u, _ := url.Parse(c.h.Server.URL + endpointPath + "/")
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == "fos_session" {
			return ck.Value
		}
	}
	return ""
}

// DoWithSessionCookie sends a request carrying the given session cookie
// value, bypassing the jar (e.g. to replay a token on another endpoint).
func (h *Harness) DoWithSessionCookie(t *testing.T, method, path, token string) Response {
	t.Helper()
	c := &Client{Header: http.Header{"Cookie": {"fos_session=" + token}}, h: h, http: &http.Client{}}
	return c.Do(t, method, path, nil)
}
