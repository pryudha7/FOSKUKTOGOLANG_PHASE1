package apptest

import (
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jimlambrt/gldap"

	"fos/internal/config"
)

// FakeLDAP is an in-process LDAP server standing in for the bank's Active
// Directory. It answers simple binds and equality searches of the form
// (attribute=value), matched case-insensitively like Active Directory.
type FakeLDAP struct {
	URL          string
	BindDN       string
	BindPassword string
	BaseDN       string

	mu      sync.Mutex
	users   []fakeUser
	filters []string
	server  *gldap.Server
}

type fakeUser struct {
	dn       string
	attrs    map[string][]string
	password string
}

// StartFakeLDAP starts the fake directory with one service account.
func StartFakeLDAP(t *testing.T) *FakeLDAP {
	t.Helper()
	f := &FakeLDAP{
		BindDN:       "CN=svc-fos,OU=Functional ID - System,OU=Account,DC=test,DC=local",
		BindPassword: "svc-secret",
		BaseDN:       "DC=test,DC=local",
	}
	f.users = append(f.users, fakeUser{dn: f.BindDN, password: f.BindPassword})

	s, err := gldap.NewServer()
	if err != nil {
		t.Fatalf("new ldap server: %v", err)
	}
	mux, err := gldap.NewMux()
	if err != nil {
		t.Fatalf("new ldap mux: %v", err)
	}
	if err := mux.Bind(f.bind); err != nil {
		t.Fatalf("route bind: %v", err)
	}
	if err := mux.Search(f.search); err != nil {
		t.Fatalf("route search: %v", err)
	}
	if err := s.Router(mux); err != nil {
		t.Fatalf("router: %v", err)
	}
	addr := freeAddr(t)
	go func() { _ = s.Run(addr) }()
	deadline := time.Now().Add(5 * time.Second)
	for !s.Ready() {
		if time.Now().After(deadline) {
			t.Fatalf("fake ldap not ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	f.server = s
	f.URL = "ldap://" + addr
	t.Cleanup(f.Stop)
	return f
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	defer l.Close()
	return l.Addr().String()
}

// Stop shuts the directory down, making it unreachable.
func (f *FakeLDAP) Stop() { _ = f.server.Stop() }

// AddUser adds a directory user identified by sAMAccountName.
func (f *FakeLDAP) AddUser(samAccountName, password string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users = append(f.users, fakeUser{
		dn:       fmt.Sprintf("CN=%s,OU=Users,%s", samAccountName, f.BaseDN),
		attrs:    map[string][]string{"sAMAccountName": {samAccountName}},
		password: password,
	})
}

// Filters returns every search filter the directory received, as decoded
// from the wire.
func (f *FakeLDAP) Filters() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.filters...)
}

func (f *FakeLDAP) bind(w *gldap.ResponseWriter, r *gldap.Request) {
	resp := r.NewBindResponse(gldap.WithResponseCode(gldap.ResultInvalidCredentials))
	defer func() { _ = w.Write(resp) }()
	m, err := r.GetSimpleBindMessage()
	if err != nil || m.AuthChoice != gldap.SimpleAuthChoice {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		// Like many directories, an empty password is an unauthenticated
		// bind and succeeds (RFC 4513 §5.1.2): the client must refuse it.
		if strings.EqualFold(u.dn, m.UserName) && (m.Password == "" || string(m.Password) == u.password) {
			resp.SetResultCode(gldap.ResultSuccess)
			return
		}
	}
}

var equalityFilter = regexp.MustCompile(`^\(([A-Za-z][A-Za-z0-9-]*)=(.*)\)$`)

func (f *FakeLDAP) search(w *gldap.ResponseWriter, r *gldap.Request) {
	done := r.NewSearchDoneResponse(gldap.WithResponseCode(gldap.ResultSuccess))
	defer func() { _ = w.Write(done) }()
	m, err := r.GetSearchMessage()
	if err != nil {
		done.SetResultCode(gldap.ResultOperationsError)
		return
	}
	f.mu.Lock()
	f.filters = append(f.filters, m.Filter)
	users := append([]fakeUser(nil), f.users...)
	f.mu.Unlock()

	parts := equalityFilter.FindStringSubmatch(m.Filter)
	if parts == nil {
		done.SetResultCode(gldap.ResultUnwillingToPerform)
		return
	}
	value := unescapeFilterValue(parts[2])
	for _, u := range users {
		if !strings.HasSuffix(strings.ToLower(u.dn), strings.ToLower(m.BaseDN)) {
			continue
		}
		for name, vals := range u.attrs {
			if strings.EqualFold(name, parts[1]) && len(vals) > 0 && strings.EqualFold(vals[0], value) {
				_ = w.Write(r.NewSearchResponseEntry(u.dn, gldap.WithAttributes(u.attrs)))
			}
		}
	}
}

// unescapeFilterValue decodes RFC 4515 \XX escapes.
func unescapeFilterValue(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+2 < len(s) {
			if v, err := hex.DecodeString(s[i+1 : i+3]); err == nil {
				b.Write(v)
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// WithLDAP configures the app's LDAP login against the fake directory, using
// the production search filter form (SoT §4.2).
func WithLDAP(f *FakeLDAP) func(*config.Config) {
	return func(c *config.Config) {
		c.LDAP = config.LDAP{
			URL:          f.URL,
			BindDN:       f.BindDN,
			BindPassword: f.BindPassword,
			BaseDN:       f.BaseDN,
			SearchFilter: "SamAccountName=%v",
			Timeout:      2 * time.Second,
		}
	}
}
