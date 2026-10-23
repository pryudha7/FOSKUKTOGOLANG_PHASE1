package auth

import (
	"errors"
	"log/slog"
	"net"
	"strings"

	"github.com/go-ldap/ldap/v3"

	"fos/internal/config"
)

// authenticateLDAP runs FR-LOGIN-002 steps 1-4: service bind, search for
// exactly one user entry, then bind as that entry with the typed password.
// It returns nil, ErrInvalidCredential, or another error meaning LDAP is
// unavailable. Errors are logged without any password.
func authenticateLDAP(cfg config.LDAP, username, password string) error {
	// An empty password is an unauthenticated bind, which many directories
	// accept (RFC 4513 §5.1.2); never send one.
	if username == "" || password == "" {
		return ErrInvalidCredential
	}
	conn, err := ldap.DialURL(cfg.URL, ldap.DialWithDialer(&net.Dialer{Timeout: cfg.Timeout}))
	if err != nil {
		slog.Error("LDAP unreachable", "url", cfg.URL, "error", err)
		return ErrLDAPUnavailable
	}
	defer conn.Close()
	conn.SetTimeout(cfg.Timeout)

	if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
		slog.Error("LDAP service bind failed", "bind_dn", cfg.BindDN, "result_code", resultCode(err))
		return ErrLDAPUnavailable
	}

	res, err := conn.Search(ldap.NewSearchRequest(
		cfg.BaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		2, int(cfg.Timeout.Seconds()), false,
		searchFilter(cfg.SearchFilter, username), []string{"dn"}, nil))
	switch {
	case ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded):
		return ErrInvalidCredential // more than one entry
	case err != nil:
		slog.Error("LDAP search failed", "result_code", resultCode(err))
		return ErrLDAPUnavailable
	case len(res.Entries) != 1:
		return ErrInvalidCredential
	}

	if err := conn.Bind(res.Entries[0].DN, password); err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
			return ErrInvalidCredential
		}
		slog.Error("LDAP user bind failed", "result_code", resultCode(err))
		return ErrLDAPUnavailable
	}
	return nil
}

// searchFilter substitutes the escaped username for %v, so filter special
// characters such as * ( ) \ are plain text, and adds the enclosing
// parentheses when the configured filter lacks them (FR-LOGIN-002).
func searchFilter(template, username string) string {
	f := strings.ReplaceAll(template, "%v", ldap.EscapeFilter(username))
	if !strings.HasPrefix(f, "(") {
		f = "(" + f + ")"
	}
	return f
}

func resultCode(err error) uint16 {
	var le *ldap.Error
	if errors.As(err, &le) {
		return le.ResultCode
	}
	return 0
}
