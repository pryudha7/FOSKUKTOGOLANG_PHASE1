# 25: Appraisal

**What to build:** When the Main Collateral is land and building, the Financing Case gets an Appraisal Submission assignment; otherwise Appraisal is skipped.

**Blocked by:** 24: RAC evaluation

**Status:** ready-for-agent

- [ ] Every `AC-FR-APPR-001-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-APPR-002-*` acceptance criterion in SoT §13 passes
- [ ] Case Status is Pending-Appraisal while Appraisal is open
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
