# 01: Walking skeleton + Database login

**What to build:** An operator registered in the FOS operator database opens the desktop endpoint, logs in through the Database path and lands on an (empty) dashboard. This slice stands up the Go backend, Next.js frontend, PostgreSQL migrations and the test harness (ephemeral database, fake external servers) that every later ticket builds on.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] Every `AC-FR-LOGIN-001-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-LOGIN-003-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-LOGIN-004-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-LOGIN-005-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-LOGIN-006-*` acceptance criterion in SoT §13 passes
- [ ] Passwords never appear in URLs, logs or responses
- [ ] Every login attempt is recorded in the login log
- [ ] Configuration is read per environment with no hard-coded defaults (ENV-001 to ENV-006)
- [ ] The test harness can start the app against an ephemeral database and register fake external servers
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

**Decisions while implementing (2026-10-23), agreed with the user:**

- Password hashing bcrypt; usernames match case-insensitively; no active/inactive flag (TBD-012).
- `operator` table columns per ENT-005: user_id, password_hash, full_name, role_code, branch_code, report_to, timezone, mobile_access (TBD-018). Timezone stored in the ENT-005 format, default `GMT+7` (TBD-030).
- Server-side `operator_session` table; HttpOnly Secure cookie scoped to the endpoint path.
- Interim for TBD-013: POST LOGIN_PATH_DB takes a JSON body with `username` and `password` only; refusals with an SoT code return JSON `{code, message}` (ERR-001 → 401, ERR-003 → 503), anything else a bare status code.
- ERR-003 text: "Layanan sedang tidak tersedia. Silakan coba beberapa saat lagi."
- Primary keys BIGINT identity (TBD-001 open).
- TLS ends at a reverse proxy that must overwrite X-Forwarded-Proto and X-Forwarded-For.

**Deferred:** the Activity Log "Login" entry of AC-FR-LOGIN-006-01 belongs to ticket 27 (FR-DASH-002). UI criteria (AC-FR-LOGIN-001-02, AC-FR-LOGIN-005-01, AC-UI-001-01) are outside the agreed HTTP API seam and were checked manually.
