// Package config reads the per-environment configuration (SoT §4, ENV-001 to
// ENV-006). Every key is required: there are no hard-coded defaults.
package config

import (
	"fmt"
	"net/netip"
	"os"
	"strings"
)

type Config struct {
	Env                 string // APP_ENV
	ListenAddr          string // LISTEN_ADDR
	DatabaseURL         string // DATABASE_URL
	LoginPathDB         string // LOGIN_PATH_DB
	EndpointPathDesktop string // ENDPOINT_PATH_DESKTOP
	// TrustedProxyCIDRs lists the reverse proxies allowed to assert HTTPS via
	// X-Forwarded-Proto (TRUSTED_PROXY_CIDRS, comma-separated).
	TrustedProxyCIDRs []netip.Prefix
}

// FromEnv loads the configuration, failing on any missing key.
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
		EndpointPathDesktop: get("ENDPOINT_PATH_DESKTOP"),
	}
	proxies := get("TRUSTED_PROXY_CIDRS")
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing configuration: %s", strings.Join(missing, ", "))
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
