# 07: Underlying Document upload + Nomor Memo

**What to build:** On Document Underlying Upload the Branch Officer sees the case's document list (initialised from master data), uploads Underlying Documents through the Upload Document modal (stored in GCS), downloads them, sees them in the attachments panel, and sees the Nomor Memo generated for the SLIK request.

**Blocked by:** 30: Pre-Screening Submission: Individual

**Status:** ready-for-agent

- [ ] Every `AC-FR-FILE-001-*`, `AC-FR-FILE-002-*`, `AC-FR-FILE-003-*`, `AC-FR-FILE-005-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-APP-008-*` acceptance criterion in SoT §13 passes (Nomor Memo also stored for Badan Usaha)
- [ ] Every `AC-FR-CASE-006-*` acceptance criterion in SoT §13 passes
- [ ] `AC-UI-011-01..05`, `AC-UI-011-09`, `AC-UI-011-10`, `AC-UI-015-*` pass (the SLIK Request Data section shows Nomor Memo; its fields arrive in ticket 33)
- [ ] Uploads are refused, and the error logged without credentials, when GCS configuration is empty (ENV-010)
- [ ] Tests use a fake GCS
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

Split out: SLIK Request Data fields and Create validation (ERR-020, ERR-026, ERR-029, FR-APP-009) are ticket 33; the Badan Usaha variant (SLIK Worst Collect, 100 MB) is in ticket 31.

## Comments

Agreed with the user before implementation (2026-10-23):

1. Split as above.
2. `document_category` (Case Document and Case Document Category) stores the codename, e.g. `DokumenPreScreening` (DB-RULE-087). For PRESCREENING categories the codename is the BMV DOCUMENTPRESCREENING description without spaces and punctuation; the label shown is that BMV row's description (found by `source_code`).
3. Interim message texts (SoT TBD):
   - ERR-021 "Hanya file yang diperbolehkan: gambar, pdf, excel, rar, dan zip"
   - ERR-024 "Nama file maksimal 100 karakter."
   - ERR-025 "Upload dokumen gagal. Silakan coba lagi."
   - ERR-027 "Daftar dokumen gagal dimuat. Silakan coba lagi."
   - ERR-041 "Ukuran file melebihi batas maksimum"
4. File type (TBD-027) is checked on modal Submit by extension: jpg, jpeg, png, gif, bmp, pdf, xls, xlsx, csv, rar, zip.
5. `validation/upload/max_file_bytes` is seeded 10485760 by migration (§9.25 table created). Missing or invalid setting = system failure of the upload (DB-RULE-084); read with a 60-second cache.
6. Seed: BMV DOCUMENTPRESCREENING `DOCPRE-2` (sub_type Perorangan, "Dokumen Pre-Screening") and the 8 Perorangan Document Mapping Value rows of Gambar 16. Badan Usaha rows come with ticket 31.
7. Download access (DEC-082, interim): an operator holding an open assignment of the case, or the case's creator. Others get ERR-035.
8. Object name `LoanFinancing/<Case ID>/<document_category>/<file_id>-<file_name>`; `file_name` (Name + original extension) is shown in the attachments panel and used on download.
9. `GCS_PROJECT_ID`, `GCS_BUCKET_NAME`, `GCS_CREDENTIAL` (path to a service-account JSON) are read from env without defaults. When one is empty the server still starts; upload and download are refused (ERR-025) and the log names the empty key, never its value. The client talks to the GCS JSON API over HTTP so tests point it at a fake.
10. Branch Code empty: no Nomor Memo is formed, the error is logged, the field shows empty; the next opening tries again.
11. The modal banner clears on Submit, on file change and on close; changing the file refills Name from the new file name.

Implementation notes (2026-10-23):

- The GCS client speaks the JSON API with a service-account JWT; tests run against a fake that checks the signature. `STORAGE_EMULATOR_HOST` (Google's emulator convention) points the backend at a local stand-in for manual UI checks only.
- Deferred, still TBD in the SoT: Submit without a file does nothing (TBD-036a); a file that cannot be fetched from GCS on download answers 502 without a message (TBD-037e); a replaced object that fails to delete is only logged (TBD-031e); the attachments panel shows three items before "View all" and short times such as "0m ago" (Gambar 18, TBD-020).
- The breadcrumb, Back and upload icon are hidden or inert for a read-only (Authors) look.

Review fixes agreed with the user (2026-10-23):

- `dynamic_system_settings` gained `updated_at`/`updated_by` (§9.25); CHECK constraints the SoT does not list were dropped.
- Create shows disabled until ticket 33 (AC-UI-011-05).
- Downloads with a GCS key empty give ERR-025 and log the key; downloads read the bucket stored on the row (DB-RULE-023).
- A failure reading the list is ERR-027 too, and the page still opens.
- The attachments panel no longer downloads on click or loads thumbnails (TBD-033/037, not to be implemented yet): names are plain blue text, thumbnails a generic tile.
- Interim texts live in one place (`cases`); the per-file limit is read once per request; an oversized request is ERR-041; the GCS token exchange runs outside the lock; an operator missing from the operator data gets no Nomor Memo instead of a failed page.
- Left as is: `STORAGE_EMULATOR_HOST` (manual UI checks only), "View all" shown for any non-empty list (Gambar 18), a concurrent upload to the same row may get ERR-025.
