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
