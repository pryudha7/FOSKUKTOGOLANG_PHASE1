# 15: Pipeline Data Entry

**What to build:** After passing the Pre-Screening Check, the Branch Officer records the Submission Type and Product; for a Take Over they also record the other bank and may upload its letter.

**Blocked by:** 10: DHN + Rejected List

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-004-*` acceptance criterion in SoT §13 passes
- [ ] Dropdowns store the BMV code and display its description
- [ ] Contract is copied from the selected Product
- [ ] Changing the Submission Type away from Take Over deletes the bank letter
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
