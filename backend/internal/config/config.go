// Package config reads the per-environment configuration (SoT §4, ENV-001 to
// ENV-010). There are no hard-coded defaults.
package config

import (
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env                 string // APP_ENV
	ListenAddr          string // LISTEN_ADDR
	DatabaseURL         string // DATABASE_URL
	LoginPathDB         string // LOGIN_PATH_DB
	LoginPathLDAP       string // LOGIN_PATH_LDAP
	EndpointPathDesktop string // ENDPOINT_PATH_DESKTOP
	EndpointPathMobile  string // ENDPOINT_PATH_MOBILE
	// TrustedProxyCIDRs lists the reverse proxies allowed to assert HTTPS via
	// X-Forwarded-Proto (TRUSTED_PROXY_CIDRS, comma-separated).
	TrustedProxyCIDRs []netip.Prefix
	LDAP              LDAP
	GCS               GCS
}

// GCSAPIBaseURL is Google Cloud Storage's JSON API; it is the same in every
// environment, so it is not a configuration key.
const GCSAPIBaseURL = "https://storage.googleapis.com"

// GCS is the file storage configuration (SoT §4.3). Like LDAP it may be
// incomplete at startup: uploads and downloads are then refused (ENV-010).
type GCS struct {
	ProjectID  string // GCS_PROJECT_ID
	BucketName string // GCS_BUCKET_NAME
	// Credential is the path of a service-account JSON key file
	// (GCS_CREDENTIAL, agreed for TBD-031b); its content is a secret (ENV-009).
	Credential string
	// APIBaseURL is GCSAPIBaseURL; tests point it at a fake.
	APIBaseURL string
}

// gcsAPIBaseURL is GCSAPIBaseURL, or a local emulator named by Google's own
// STORAGE_EMULATOR_HOST convention (manual UI checks; never set in an
// environment).
func gcsAPIBaseURL() string {
	host := optional("STORAGE_EMULATOR_HOST")
	if host == "" {
		return GCSAPIBaseURL
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	return strings.TrimSuffix(host, "/")
}

// Missing names the GCS keys without a value, never their values.
func (g GCS) Missing() []string {
	var out []string
	for _, k := range []struct{ key, v string }{
		{"GCS_PROJECT_ID", g.ProjectID}, {"GCS_BUCKET_NAME", g.BucketName}, {"GCS_CREDENTIAL", g.Credential},
	} {
		if k.v == "" {
			out = append(out, k.key)
		}
	}
	return out
}

// LDAP is the LDAP login configuration (SoT §4.2). Unlike the keys above it
// may be incomplete at startup: LDAP login is then refused (ENV-008).
type LDAP struct {
	URL          string        // LDAP_URL
	BindDN       string        // LDAP_BIND_DN
	BindPassword string        // LDAP_BIND_PASSWORD, a secret (ENV-007)
	BaseDN       string        // LDAP_BASE_DN
	SearchFilter string        // LDAP_SEARCH_FILTER, %v replaced by the username
	Timeout      time.Duration // LDAP_TIMEOUT_SECONDS (TBD-011, agreed)
}

// Complete reports whether every LDAP key has a value.
func (l LDAP) Complete() bool {
	return l.URL != "" && l.BindDN != "" && l.BindPassword != "" && l.BaseDN != "" &&
		l.SearchFilter != "" && l.Timeout > 0
}

// FromEnv loads the configuration, failing on any missing required key.
func FromEnv() (Config, error) {
	var missing []string
	get := func(key string) string {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}
	cfg := Config{
		Env:                 get("APP_ENV"),
		ListenAddr:          get("LISTEN_ADDR"),
		DatabaseURL:         get("DATABASE_URL"),
		LoginPathDB:         get("LOGIN_PATH_DB"),
		LoginPathLDAP:       get("LOGIN_PATH_LDAP"),
		EndpointPathDesktop: get("ENDPOINT_PATH_DESKTOP"),
		EndpointPathMobile:  get("ENDPOINT_PATH_MOBILE"),
		LDAP: LDAP{
			URL:          optional("LDAP_URL"),
			BindDN:       optional("LDAP_BIND_DN"),
			BindPassword: optional("LDAP_BIND_PASSWORD"),
			BaseDN:       optional("LDAP_BASE_DN"),
			SearchFilter: optional("LDAP_SEARCH_FILTER"),
		},
		GCS: GCS{
			ProjectID:  optional("GCS_PROJECT_ID"),
			BucketName: optional("GCS_BUCKET_NAME"),
			Credential: optional("GCS_CREDENTIAL"),
			APIBaseURL: gcsAPIBaseURL(),
		},
	}
	proxies := get("TRUSTED_PROXY_CIDRS")
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing configuration: %s", strings.Join(missing, ", "))
	}
	// SoT §4.2: "Path login LDAP dan Database harus berbeda"; the auth method
	// is chosen by the path.
	if cfg.LoginPathDB == cfg.LoginPathLDAP {
		return Config{}, fmt.Errorf("LOGIN_PATH_DB and LOGIN_PATH_LDAP must differ")
	}
	if v := optional("LDAP_TIMEOUT_SECONDS"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			cfg.LDAP.Timeout = time.Duration(secs) * time.Second
		} else {
			slog.Error("LDAP_TIMEOUT_SECONDS is not a positive number of seconds; LDAP login will be refused (ENV-008)")
		}
	}
	for _, p := range strings.Split(proxies, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(p))
		if err != nil {
			return Config{}, fmt.Errorf("TRUSTED_PROXY_CIDRS: %w", err)
		}
		cfg.TrustedProxyCIDRs = append(cfg.TrustedProxyCIDRs, prefix)
	}
	return cfg, nil
}

// optional reads a key whose absence is handled where it is used; the SoT
// placeholder "TBD" counts as absent (ENV-008).
func optional(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if strings.EqualFold(v, "TBD") {
		return ""
	}
	return v
}
