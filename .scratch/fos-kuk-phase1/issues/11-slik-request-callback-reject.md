# 11: SLIK request, callback and rejection

**What to build:** For an Individual, the Pre-Screening Check requests SLIK (and SLIK for the Spouse, if any) through SFS, then waits; the RPA callback delivers the result, which is stored with its PDF and evaluated for Check Rejection.

**Blocked by:** 10: DHN + Rejected List

**Status:** ready-for-agent

- [ ] Every `AC-FR-PSAPI-008-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-PSAPI-009-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-PSAPI-010-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-PSAPI-012-*` acceptance criterion in SoT §13 passes
- [ ] SLIK runs only for an Individual Customer
- [ ] A SLIK request is only ever Success or Fail; rejection is decided when the result arrives
- [ ] Rejection applies on Collectibility >=2 with 1-15 Days Past Due in >=7 of 24 months, or Collectibility >=2 with >15 Days Past Due in >=2 months
- [ ] Rejection applies as soon as either the Customer's or the Spouse's result rejects
- [ ] Tests drive the real inbound callback endpoint, not the database
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
