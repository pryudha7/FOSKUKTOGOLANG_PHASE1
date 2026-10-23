# 19: Hot Prospect: financing request

**What to build:** The Branch Officer records the requested facility: Requested Amount, Tenor and Margin Rate, runs the Financing Simulation, and sees the Total Obligor.

**Blocked by:** 18: Hot Prospect: business and other financing

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-018-*` acceptance criterion in SoT §13 passes
- [ ] Margin Rate is 0-100 with two decimals
- [ ] Total Obligor is the Customer's existing financing plus the Requested Amount
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
