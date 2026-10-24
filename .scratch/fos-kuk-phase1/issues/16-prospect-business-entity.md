# 16: Prospect Data Entry: Business Entity

**What to build:** For a Business Entity Customer, the Branch Officer records the entity's prospect data in one assignment.

**Blocked by:** 15: Pipeline Data Entry

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-005-*` acceptance criterion in SoT §13 passes
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

Agreed with the user before implementation (2026-10-24):

1. Next on Badan Usaha (TBD-052; FR-APP-015 PROPOSED "dengan cara yang sama"): as Create on Data Keluarga Dekat (ticket 17): the assignment completes, the case enters Financing Application, Pending-Submission, a new (baru) Data Usaha Nasabah is created, and the page returns to the Dashboard.
2. Close: back to the Dashboard without saving (PROPOSED).
3. Data Pengurus starts from the Pre-Screening Pengurus rows; with none (the Pre-Screening Badan Usaha form is ticket 31) one empty card.
4. 1 to 10 cards; Tambah does nothing at 10, delete nothing at 1; ERR-014 Jabatan, the Pre-Screening rules for NIK, Nama and Tanggal Lahir (ERR-010, ERR-007, ERR-008, ERR-013, ERR-009), ERR-018 on every duplicate NIK, ERR-015 as a guard for 0 cards; banner "Pengurus <n> - <Nama Field> : <pesan>".
5. The same Pengurus rows are updated in card order, extra cards inserted, rows of removed cards deleted (DEC-112: no copy).
6. Saved on the customer row "Badan Usaha": Kode Badan Usaha, Tempat Pendirian, Nomor Akta Pendirian, Nomor Akta Perubahan Terakhir, Tanggal Akta Perubahan Terakhir, Kode Bidang Usaha, Hubungan dengan Bank, Tipe Bisnis, Golongan Nasabah (BUSINESSENTITY_CATEGORY), Go Public (its description, §9.6), Alamat Email, Nama Group Usaha Nasabah, is_existing_customer. Alamat, Nomor Telepon, Nomor HP and the region on customer_address type "Badan Usaha" (DB-RULE-099).
7. "Alamat * ... (sama dengan Alamat Nasabah, UI-011)" means the same rules (required, at most 350), not a copy: SLIK Request Data on UI-011 is Perorangan only.
8. Tanggal Akta Perubahan Terakhir: optional; before today in the operator's timezone (today or later ERR-037); the picker ends yesterday, no lower bound.
9. The ticket 17 economic-sectors endpoint becomes GET .../suggestions?type=ECONOMIC_SECTOR|BUSINESSENTITY_CATEGORY&q= (code or description); two-line items (Gambar 50); the corner marker of Gambar 48 drawn by CSS on both fields.
10. ERR-039/ERR-038 (AC-FR-APP-005-07): a failed initial dropdown query leaves the page served with the failed dropdowns named; each shows only the ERR-038 item and the ERR-039 popup (OK) opens; the ticket 17 pages get the same. Checked in the UI check by faking the API answer at the network boundary (Playwright route).
11. Customer status: NPWP and "Badan Usaha" against both tables, stored only when the owner opens the unfinished assignment (as ticket 17).
12. BMV master data not seeded (BUSINESS_ENTITY, GO_PUBLIC_STATUS, BUSINESSENTITY_CATEGORY, BUSINESS_ROLE, ...); tests and smoke add their own.
13. Out of scope: the blank Details while the assignment is active (Details ticket); the Pre-Screening Badan Usaha form (ticket 31).
14. Seams: HTTP API against real PostgreSQL (Pipeline Submit to Badan Usaha, page prefill and customer status, validation with Pengurus, saving with Pengurus added, changed and removed, Next to Financing Application, the suggestions endpoint); UI with tools/uicheck against Gambar 48-50 and the ERR-039 popup.

Implementation notes (2026-10-24):

- POST .../assignments/{id}/business-entity takes only this form's fields (BR-UI-010); more than 10 cards is a request the page cannot send (400). Next shares saveForm and enterFinancingApplication with ticket 17.
- The entity's NPWP is read from customer.npwp, as the Pre-Screening Check already reads it (party.go, pefindo.go), not id_number.
- Field rules come from cases/field_checks.go, now with pastDate (Tanggal Akta Perubahan Terakhir) and description (Go Public, stored and validated by description). Pengurus checks (officers) and saveOfficers live in cases/business.go.
- pageOptions no longer fails the page: a failed list is named in failedOptions (also on the ticket 17 pages), the dropdown shows only the ERR-038 item and DropdownsFailedPopup shows ERR-039 once (the ERR-035 popup with its own message).
- economic-sectors became GET suggestions?type=ECONOMIC_SECTOR|BUSINESSENTITY_CATEGORY&q=; the Autocomplete component appends q to a source that may already carry a query.
- Changing the number of Pengurus cards drops the cards' errors, whose numbers no longer hold.
- UI check tools/uicheck/business.js (screens shots/business*.png): Gambar 48-49 layout with the NPWP, name, deed date and Pengurus prefilled, Kota disabled, delete doing nothing on the last card, ERR-037 phone and email, "Pengurus 1/2 - NIK : NIK tidak boleh sama" and the card errors, "pertanian" listing three two-line items (Gambar 50) with the description shown after Enter, Golongan Nasabah by "bum", Next to Financing Application, Pending-Submission with Data Usaha Nasabah new; then the page answer faked with businessRole failed: the ERR-039 popup and Jabatan holding only the ERR-038 item.
- Mutation checks (each fails a test): no ERR-018; officers always inserted; removed officers kept; today accepted as the last deed date; Go Public options by code; customer type ignored; no ERR-015; Next staying in Prospect; empty Jabatan as ERR-010; Golongan Nasabah from CUSTOMER_CATEGORY; Go Public not checked.

Review fixes (2026-10-24, agreed with the user):

- Names follow the glossary (ADR 0001): Company Officer for Pengurus (CompanyOfficer, companyOfficers, saveCompanyOfficers; JSON companyOfficers; banner section company-officer-<n>, made and read by companyOfficerSection/companyOfficerNumber in types.ts).
- PageOptions bundles a page's options and failed dropdowns (Go and JSON, embedded in the four page structs); saveForm's second step is afterSave; keyedByDescription states the Go Public rule; toOptionJSON serves both pages.
- The shared form pieces (useMove, TextField, CodeSelect, CodeRadios, NumberField, ShownValue, useRegion, Pair, RegionRows) moved from ProspectDataEntry.tsx to FormFields.tsx.
- UI-025 "teks abu": the long date of a disabled date field is grey too.
- BR-UI-012: a saved Go Public whose BMV row is no longer Active is still offered and shown. The same gap on the other BMV dropdowns (tickets 15, 17) is a follow-up, not done here.
- Person names (Nama Pengurus; Nama Pasangan, Nama Lengkap Keluarga Dekat, Nama Gadis Ibu Kandung of ticket 17) have no maxLength, so ERR-013 shows as on Pre-Screening; the ERR-040 maxlength note does not cover ERR-013.
- Noted: AC-FR-APP-005-01's "Details kosong" waits for the Details ticket (comment 13), so that AC is not fully passed here; more than 10 cards is refused with 400, an error path the SoT does not define.
- Rerun after the fixes: tests, all 11 mutations caught, UI checks business.js, prospect.js and pipeline.js.

