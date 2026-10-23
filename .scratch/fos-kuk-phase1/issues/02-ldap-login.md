# 02: LDAP login

**What to build:** An operator logs in through the LDAP path: credentials are validated against LDAP, then the operator must also be registered in the FOS operator database.

**Blocked by:** 01: Walking skeleton + Database login

**Status:** ready-for-agent

- [ ] Every `AC-FR-LOGIN-002-*` acceptance criterion in SoT §13 passes
- [ ] Login via LDAP is refused, and ERR-002 logged, when any LDAP configuration key is empty (ENV-008)
- [ ] The LDAP bind password never appears in code, repository or logs
- [ ] Tests use a fake LDAP server
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

**Decisions while implementing (2026-10-23), agreed with the user:**

- ERR-002 text: "Layanan autentikasi sedang tidak tersedia. Silakan coba beberapa saat lagi." (HTTP 503, interim per TBD-013).
- Transport follows the `LDAP_URL` scheme (`ldap://` or `ldaps://` with certificate validation); no StartTLS yet. New required-for-LDAP key `LDAP_TIMEOUT_SECONDS` (TBD-011).
- `LDAP_BIND_PASSWORD` is injected through the environment by the deployment (TBD-010).
- With any LDAP key empty or "TBD" the app still starts; LDAP login is refused with ERR-002 / `LDAP_UNAVAILABLE` (ENV-008).
- A directory user who is not a FOS operator gets ERR-001 / `NOT_REGISTERED`; operator matching is case-insensitive.
- An empty password is refused before any bind: directories treat it as an unauthenticated bind that succeeds (RFC 4513 §5.1.2).

- Any refusal of the user bind by the directory (e.g. 49 invalid credentials, 53 for a locked account) is ERR-001 / `INVALID_CREDENTIAL`; only a lost connection or timeout is ERR-002. A failed search (e.g. wrong `LDAP_BASE_DN`) is ERR-002.

**Tests** use an in-process fake directory (`apptest.FakeLDAP`) that matches `(attribute=value)` filters case-insensitively like Active Directory, accepts unauthenticated binds like many directories, and records the filters it receives.
