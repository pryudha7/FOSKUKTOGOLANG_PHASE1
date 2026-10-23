# 17: Prospect Data Entry: Individual

**What to build:** For an Individual Customer, the Branch Officer records Data Pribadi, then Data Pasangan, then Data Keluarga Dekat.

**Blocked by:** 15: Pipeline Data Entry

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-012-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-APP-013-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-APP-014-*` acceptance criterion in SoT §13 passes
- [ ] NIK is 16 digits, NPWP 15-22 digits, age at least 17, names at most 30 characters
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
