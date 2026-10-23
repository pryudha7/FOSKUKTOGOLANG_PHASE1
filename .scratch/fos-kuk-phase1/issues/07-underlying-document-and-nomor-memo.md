# 07: Underlying Document upload + Nomor Memo

**What to build:** On Financing Submission the Branch Officer moves between Pre-Screening Submission and Document Underlying Upload, uploads, views and downloads Underlying Documents (stored in GCS), and sees the Nomor Memo generated for the SLIK request.

**Blocked by:** 05: Create Financing Case + Pre-Screening Submission

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-003-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-APP-008-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-FILE-001-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-FILE-002-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-FILE-003-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-FILE-004-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-FILE-005-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-CASE-006-*` acceptance criterion in SoT §13 passes
- [ ] Total upload size is limited to 10 MB for an Individual and 100 MB for a Business Entity, checked at Create (ERR-020)
- [ ] Create is refused while a mandatory document is missing (ERR-026)
- [ ] Uploads are refused, and the error logged without credentials, when GCS configuration is empty (ENV-010)
- [ ] A Nomor Memo is generated and stored for Business Entity cases too
- [ ] Tests use a fake GCS
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
