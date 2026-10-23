# 33: SLIK Request Data + Create (Individual)

**What to build:** On Document Underlying Upload for Perorangan, the Branch Officer fills SLIK Request Data and presses Create; the system checks total upload size and mandatory documents, completes the assignment and creates the Team Lead's Pre Screening assignment.

**Blocked by:** 07: Underlying Document upload + Nomor Memo

**Status:** ready-for-agent

- [ ] Every `AC-FR-FILE-004-*` acceptance criterion in SoT §13 for Perorangan passes
- [ ] `AC-UI-011-06..08`, `AC-UI-011-11`, `AC-UI-011-12`, `AC-UI-011-15` pass
- [ ] Tujuan Permintaan, Pilihan Fasilitas, Tipe Pencarian, Segmentasi from BMV SLIK_TUJUANPERMINTAAN, SLIK_PILIHANFASILITAS, SLIK_TIPEPENCARIAN, SLIK_SEGMENTASI (Active, sort_order then code; code stored); required (ERR-010)
- [ ] Alamat Nasabah required, at most 350 runes after CRLF→LF (ERR-029)
- [ ] Total upload over 10 MB: ERR-020 banner "Notes: Maksimal total ukuran file upload: 10 MB (Total: x.xx MB)"; mandatory document without file: ERR-026 "Harap upload dokumen wajib" on the row; both may show together
- [ ] Create passing validation completes the assignment and creates the TL "Pre Screening" worklist assignment (FR-APP-009)
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.


## Comments

Agreed with the user before implementation (2026-10-23):

1. SLIK Request Data is stored on `fos.work` (TBD-039b; like `work.customer_type`): `slik_request_purpose_code`, `slik_facility_option_code`, `slik_identity_address`, `slik_search_type_code`, `slik_segmentation_code`, BMV codes for the dropdowns. Saved when Create passes and shown again when the assignment reopens. `slik_worst_collect` comes with ticket 31.
2. ERR-010 and ERR-029 show as on UI-010: top banner "<Field> : <pesan>", red label and message under the field. ERR-020 is a banner line "Notes: Maksimal total ukuran file upload: …" (Gambar 24). ERR-029 text (interim): "Alamat Nasabah maksimal 350 karakter".
3. Total in ERR-020 is rounded up to two decimals (TBD-027), so a total just over the limit never reads as the limit.
4. Screenshot/text conflict resolved: the accordion error marker sits left of the chevron as in Gambar 24 (text says right). It shows when the accordion has an ERR-026 row, and that accordion opens.
5. Errors clear only when Create is pressed again.
6. After Create passes, the browser returns to the Dashboard. The Document Underlying Upload assignment is completed and a worklist "Pre Screening" assignment is created for `branch_officer_team_lead`; case_status becomes New if empty; process progress is unchanged.
7. Team Lead empty or not in the operator data (TBD-029): Create is refused with the banner message "Team Lead case tidak ditemukan" (interim), and the failure is logged.
8. Back and Close keep no validation; SLIK fields not yet created are not saved on Back or Close.
9. Alamat Nasabah is checked only for 350 characters for now.

Implementation notes (2026-10-23):

- Create is `POST .../assignments/{id}/create`; a code that is not an Active option, an unknown field, or a Badan Usaha case is refused with 400.
- The total counts only files of the case's PRESCREENING categories (FR-FILE-004 rule 1).
- The Team Lead check runs after the field checks pass, so its message never mixes with field errors.
