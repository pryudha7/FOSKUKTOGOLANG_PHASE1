# 33: SLIK Request Data + Create (Individual)

**What to build:** On Document Underlying Upload for Perorangan, the Branch Officer fills SLIK Request Data and presses Create; the system checks total upload size and mandatory documents, completes the assignment and creates the Team Lead's Pre Screening assignment.

**Blocked by:** 07: Underlying Document upload + Nomor Memo

**Status:** needs-triage

- [ ] Every `AC-FR-FILE-004-*` acceptance criterion in SoT §13 for Perorangan passes
- [ ] `AC-UI-011-06..08`, `AC-UI-011-11`, `AC-UI-011-12`, `AC-UI-011-15` pass
- [ ] Tujuan Permintaan, Pilihan Fasilitas, Tipe Pencarian, Segmentasi from BMV SLIK_TUJUANPERMINTAAN, SLIK_PILIHANFASILITAS, SLIK_TIPEPENCARIAN, SLIK_SEGMENTASI (Active, sort_order then code; code stored); required (ERR-010)
- [ ] Alamat Nasabah required, at most 350 runes after CRLF→LF (ERR-029)
- [ ] Total upload over 10 MB: ERR-020 banner "Notes: Maksimal total ukuran file upload: 10 MB (Total: x.xx MB)"; mandatory document without file: ERR-026 "Harap upload dokumen wajib" on the row; both may show together
- [ ] Create passing validation completes the assignment and creates the TL "Pre Screening" worklist assignment (FR-APP-009)
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

Blocking question before work: where SLIK Request Data is stored (TBD-039b; §9.4 has no columns for it).
