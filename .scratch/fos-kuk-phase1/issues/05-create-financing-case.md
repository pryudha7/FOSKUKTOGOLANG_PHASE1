# 05: Create Financing Case + Pre-Screening Submission

**What to build:** A Branch Officer chooses Create, then Financing. A Financing Case is created with its Case ID, Customer Code, Team Lead and branch, and opens on the Pre-Screening Submission assignment with the stage bar, case information panel, To do list and both case view modes working.

**Blocked by:** 03: Access control and endpoints, 04: Layout and navigation

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-001-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-APP-002-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-CASE-001-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-CASE-002-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-CASE-004-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-CASE-005-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-CASE-007-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-CASE-008-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-CASE-009-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-ASSIGN-001-*` acceptance criterion in SoT §13 passes
- [ ] Case IDs come from a per-prefix sequence (LF-n)
- [ ] Customer Code is SMSME + two-digit year + Case ID number padded to eight digits
- [ ] Team Lead is the creating Branch Officer's report-to operator
- [ ] Authors cannot create a Financing Case (ERR-022)
- [ ] Going back to an earlier assignment creates a new assignment row
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
