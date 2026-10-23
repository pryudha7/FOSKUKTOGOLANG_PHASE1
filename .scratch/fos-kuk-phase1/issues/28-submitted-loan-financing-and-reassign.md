# 28: Submitted Loan Financing + reassign

**What to build:** FA roles see the Submitted Loan Financing list, restricted by the Total Obligor band decision table, and can reassign a Financing Case to a member of the FA work group.

**Blocked by:** 19: Hot Prospect: financing request, 27: Dashboard: Latest Activity + assignment panel

**Status:** ready-for-agent

- [ ] Every `AC-FR-DASH-004-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-DASH-005-*` acceptance criterion in SoT §13 passes
- [ ] Total Obligor bands (500M / 1.5B / 2B / 3.5B / 75B IDR) are read from the decision table, not hard-coded
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
