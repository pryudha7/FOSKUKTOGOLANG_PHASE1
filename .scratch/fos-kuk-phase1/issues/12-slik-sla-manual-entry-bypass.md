# 12: SLIK SLA, Manual SLIK Entry and bypass

**What to build:** When a SLIK result does not arrive within its SLA, or the bypass scenario is enabled and a SLIK request fails, the System records an 'Input Manual' Recommendation and the Financing Case returns to the Branch Officer for Manual SLIK Entry. While waiting, the Branch Officer can check SLIK status.

**Blocked by:** 11: SLIK request, callback and rejection

**Status:** ready-for-agent

- [ ] Every `AC-FR-PSAPI-011-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-PSAPI-013-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-PSAPI-019-*` acceptance criterion in SoT §13 passes
- [ ] The SLIK SLA (default 1 hour) and the bypass flag are read from Dynamic System Settings
- [ ] A bypassed retry writes a Bypass log row without calling SLIK
- [ ] Create is refused until a SLIK document is uploaded on a case returned for Manual SLIK Entry (ERR-036)
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
