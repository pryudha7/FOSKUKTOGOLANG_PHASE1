# 31: Pre-Screening Submission: Business Entity

**What to build:** With Tipe Nasabah Badan Usaha, the Branch Officer fills Data Badan Usaha (NPWP, Nama Badan Usaha, Tanggal Akta Pendirian) and 1 to 10 Data Pengurus entries; Next validates and stores them as the case's Badan Usaha and Pengurus customer rows.

**Blocked by:** 30: Pre-Screening Submission: Individual

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-002-*` acceptance criterion in SoT §13 for the Badan Usaha form passes
- [ ] NPWP: ERR-010, ERR-011 (digits only), ERR-012 (15-22 digits), in that order; Nama Badan Usaha ERR-017 (30 max) and ERR-008; Tanggal Akta Pendirian ERR-016 (not after today)
- [ ] Data Pengurus: starts with 1 entry, 1 minimum, 10 maximum; Jabatan from BMV BUSINESS_ROLE (ERR-014); NIK/Nama/Tanggal Lahir as Data Pribadi; duplicate NIK across entries ERR-018 on every duplicate; ERR-015 guards zero entries
- [ ] Banner lines for entries read "Pengurus <n> - <Nama Field> : <pesan>"
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
