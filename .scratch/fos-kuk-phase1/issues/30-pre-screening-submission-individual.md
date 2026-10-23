# 30: Pre-Screening Submission: Individual

**What to build:** On the Pre-Screening Submission assignment, the Branch Officer picks Tipe Nasabah Perorangan and fills Data Pribadi and, optionally, Data Pasangan. Next validates the form, stores the data on the case's customer rows (§9.28), fixes Tipe Nasabah, completes the assignment and opens Document Underlying Upload.

**Blocked by:** 05: Create Financing Case + Case View frame

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-002-*` acceptance criterion in SoT §13 for the Perorangan form passes
- [ ] Validation runs on Next only: ERR-010 (required), ERR-007 (NIK 16 digits), ERR-008 / ERR-013 (name characters / 30 max), ERR-009 (age at least 17), ERR-019 (spouse NIK equals own NIK); errors in the banner and under the field
- [ ] Data Pasangan is optional while all three fields are empty; filling one makes all three required
- [ ] Changing Tipe Nasabah before submit clears every data field; after submit Tipe Nasabah is disabled
- [ ] ERR-006 blocks submit when the case has no branch
- [ ] Next completes Pre-Screening Submission and creates an opened Document Underlying Upload assignment for the same operator (FR-ASSIGN-001 rules 2-5)
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
