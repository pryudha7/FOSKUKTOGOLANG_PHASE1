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

## Comments

**Decisions (2026-10-23, agreed):**

- Data is saved when Next or the breadcrumb passes validation; Close saves nothing (TBD-023c).
- Birth dates range from 100 years ago to today; age is full years on today's date in the operator's timezone. Interim texts: invalid date "Harap masukkan tanggal yang valid" (code `DATE_INVALID`), out of range "Tanggal Lahir di luar rentang yang diizinkan" (`DATE_OUT_OF_RANGE`) (TBD-023f/g).
- Back from Document Underlying Upload (breadcrumb or Back) needs no validation yet (TBD-027a): it completes Document Underlying Upload and opens a new Pre-Screening Submission row.
- Only the worklist owner submits; Authors may view.
- Tipe Nasabah other than Perorangan cannot be submitted yet (Badan Usaha: ticket 31; others: TBD-029a).
- `fos.customer` holds all §9.28 columns; Pre-Screening writes the "Nasabah" row and, only when Data Pasangan is filled, the "Pasangan" row; a resubmit updates them and an emptied Data Pasangan deletes the "Pasangan" row. Tipe Nasabah goes to `work.customer_type` as its description.
- With Tipe Nasabah not chosen, only its ERR-010 is reported (no data section is shown yet).

**After review (agreed):**

- ERR-019 is reported whenever both NIKs are valid and equal, alongside any other spouse error; surrounding spaces are trimmed, so a field of spaces is empty.
- Failures other than validation show the ERR-003 text in the banner; until ticket 31 only Perorangan can be chosen in Tipe Nasabah (other options are listed but disabled).
- Banner lines for Data Pasangan start with "Data Pasangan - " (like "Pengurus <n> - "); Data Pribadi has no prefix.
- The page carries `editable` (false for an Authors look: read-only, no Next) and the operator's `today` / `earliestBirthDate`, which bound the birth-date picker.
- Field errors carry a stable `key` (`branch`, `customerType`, `nik`, `name`, `birthDate`) besides the label; the Data Pribadi section is `customer` in the API.

**API:** `POST <desktop>/api/cases/{caseId}/assignments/{id}/submit` (Pre-Screening Submission; body `customerType`, `customer`, `spouse` with `nik`, `name`, `birthDate` yyyy-mm-dd; 422 `{errors: [{section, key, field, code, message}]}`), `POST …/back` (Document Underlying Upload).

**Verification:** HTTP API tests cover AC-FR-APP-002-04/06/07/08/11/12/21/23/24 (Perorangan parts), AC-FR-APP-003-02/03, AC-FR-ASSIGN-001-04/05/06. Headless Chrome checked the form, error banner and field errors (Gambar 13), the date field and picker (Gambar 12, BR-UI-011), Next to Document Underlying Upload and Back.
