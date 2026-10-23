# 03: Access control and endpoints

**What to build:** Access groups, roles, work groups and work queues are loaded from master data (SoT §9.3). The desktop and mobile endpoints are distinct, sessions are isolated per endpoint, and idle operators are warned and logged off.

**Blocked by:** 01: Walking skeleton + Database login

**Status:** ready-for-agent

- [ ] Every `AC-FR-ACCESS-002-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-ACCESS-003-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-ACCESS-004-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-ACCESS-005-*` acceptance criterion in SoT §13 passes
- [ ] The mobile endpoint admits only operators with mobile access enabled
- [ ] Switching endpoint requires logging in again
- [ ] Idle timeout warns at 14 minutes and logs off at 15
- [ ] No permission is inferred from role, work group or work queue names (SoT §1.4 rule 6); the permission matrix itself is out of scope (TBD-003)
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

**Decisions while implementing (2026-10-23), agreed with the user:**

- Scope of AC-FR-ACCESS-002: master tables (§9.3) and role → access group / work group / work queue resolution through the role's **active mapping**, resolved on demand and exposed on `GET <endpoint>/api/me`; no default when there is none. Managing operators and master data through UI-006/UI-007 is deferred (TBD-003, TBD-018); until then master data is maintained in the database, with no code change.
- `fos.access_groups` added (not in §9.3, referenced by `role_access_mappings`), following the `work_groups` pattern.
- Seeded by migration: access groups Authors/Admin/User (§3.2), the 39 roles (§3.3) and 10 work groups (§3.4) with code = name (the SoT defines no code convention), and the only complete mapping BM → User / Committee (§3.6).
- Mobile login paths sit under the mobile endpoint: `{ENDPOINT_PATH_MOBILE}{LOGIN_PATH_DB|LDAP}` (TBD-017). Desktop keeps the bare paths.
- ERR-004 (HTTP 403): "Anda tidak memiliki akses ke aplikasi mobile." Mobile home is an empty page (TBD-017).
- Idle timeout 14 + 1 minutes as constants (TBD-040); the server ends a desktop session 15 minutes after its last request. Exempt: role name starting "Identity Management", access group Authors or Admin. Mobile sessions have no idle timeout (FR-ACCESS-005 rule 4). Tests use a fake clock.
- The popup's X is shown and does nothing (TBD-040). After idle logout the UI returns to the login page the session came from; `operator_session.auth_method` records it (TBD-013).

**After review (agreed):**

- `POST <endpoint>/api/logout` ends the session on the server and clears its cookie; the idle countdown calls it before opening the login page (FR-ACCESS-005 step 4). Ticket 04's logout reuses it.
- `GET <endpoint>/api/me` returns the idle timings (`idle.warnAfterSeconds` 840, `idle.logoffAfterSeconds` 900) or `null` when exempt, so the UI holds no copy.
- The Authors/Admin exemption compares the access group **name**, like the role-name rule.
- `ENDPOINT_DEFAULT` accepts only `desktop` or `mobile`; anything else is a configuration error.
- ERR-004 is logged with an empty `fail_reason`, like ERR-003, pending TBD-047.

**Out of scope:** desktop network restriction to internal/VPN (infrastructure, TBD-017).

**Not covered by automated tests:** the UI-019 popup itself (AC-FR-ACCESS-005-01/02) and the bare-host redirect (AC-FR-ACCESS-003-03) are frontend behaviour outside the HTTP API seam; the redirect was checked manually, the popup was built to UI-019 but not exercised in a browser.
