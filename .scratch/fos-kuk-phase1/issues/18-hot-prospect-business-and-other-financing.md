# 18: Hot Prospect: business and other financing

**What to build:** On Financing Application, the Branch Officer records the Customer's business (Data Usaha Nasabah) and their other financing (Data Pembiayaan Lain).

**Blocked by:** 16: Prospect Data Entry: Business Entity, 17: Prospect Data Entry: Individual

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-015-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-APP-016-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-APP-017-*` acceptance criterion in SoT §13 passes
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

Agreed with the user before implementation (2026-10-24):

1. Next: Data Usaha Nasabah validates, saves and opens Data Pembiayaan Lain; Data Pembiayaan Lain opens Data Permohonan Fasilitas at once (FR-ASSIGN-001 rule 5; its page is ticket 19).
2. Save (PROPOSED): stores the current entries without the required-field check and stays on the page; formats and ranges are still checked (the columns are numbers).
3. Back to Previous Step: on Data Pembiayaan Lain validates, saves and opens Data Usaha Nasabah (as Back in ticket 17); on Data Usaha Nasabah, the first Hot Prospect step, it shows (Gambar 71) but is disabled.
4. Close (Data Usaha Nasabah) returns to the Dashboard without saving; "Back" at the bottom left of Data Pembiayaan Lain sits where Close is and does the same.
5. Breadcrumb of six steps: a click validates and saves the active form, then opens the target step (as ticket 17); the pages of steps 3-6 come with their tickets.
6. The card header title is the assignment name: "Data Usaha Nasabah" and "Data Pembiayaan Lain" (Gambar 72; UI-030 does not name it and Gambar 71 is cut).
7. New tables exactly as the SoT: customer_business (§9.30, PK and FK customer_id) and other_financing (§9.31, other_financing_id BIGINT); the business address is customer_address type "Usaha" of the applicant ("Nasabah" or "Badan Usaha", DB-RULE-105). BMV BUSINESSCATEGORY and BUSINESSLOCATION not seeded.
8. Data Pembiayaan Lain: Next and Save replace all other_financing rows of the case in one transaction (DB-RULE-107); an all-empty card is dropped; no cards at all is accepted; a card is deleted without confirmation (PROPOSED); card errors carry the banner prefix "Pembiayaan <n> - " (following "Pengurus <n> - "; not in the SoT).
9. Margin / Ujrah follows BR-UI-013 (PROPOSED): typed with a comma, shown "21,00%" on blur, stored NUMERIC(5,2), 0 to 100; letters, three decimals or over 100 are ERR-037.
10. Isi Data dari SLIK shows only with a RECEIVED CUSTOMER slik_report (from the API), never for Badan Usaha (PROPOSED).
11. The modal: one accordion per condition_description, ordered by the condition code, "Fasilitas <kondisi> (<n>)", closed at first; header and facility checkboxes kept in step; mapping Nama Bank = financial_institution_description, Jenis Pembiayaan = credit_financing_type_description, Jumlah Pembiayaan = initial_credit_limit, Jumlah Outstanding = outstanding_principal, Jangka Waktu = full months from credit_start_date to due_date (PROPOSED), Margin = interest_rate, SLIK Collect = quality, Tujuan = usage_type_description; money rounded to the nearest rupiah (PROPOSED); Submit replaces every card on screen; Submit with nothing ticked changes nothing; no confirmation (PROPOSED). Served by GET .../slik-facilities.
12. Data Usaha Nasabah messages: ERR-010, ERR-040 (Jenis Bidang Usaha 100, Alamat Usaha 350), ERR-037 for ranges (PROPOSED: Lama Usaha 0-99.999, Jumlah Karyawan 0-9.999.999, Lama Tempat Usaha Tahun 0-999 and Bulan 0-11), RT/RW, Nomor Telepon and BMV values; placeholder "0" on Jangka Waktu and SLIK Collect.
13. Out of scope: AC-FR-APP-015-02 to -05 (the Details tabs, Pre-Screening not reopened, Document Upload of ticket 22); AC-FR-APP-015-01 already passes since tickets 16/17.
14. ERR-038/ERR-039 for failed dropdowns as in ticket 16.
15. Seams: HTTP API against real PostgreSQL (both pages, validation, Save, Next, Back to Previous Step and breadcrumb, replacing other_financing, the SLIK facilities endpoint: grouping, mapping, when the button shows); UI with tools/uicheck against Gambar 71-73.
16. Conflict §9.30 (all customer_business columns NOT NULL) vs Save without the required check (agreed after asking): Save stores the business address as it is and writes customer_business only when its five fields are all filled; otherwise the earlier row stays and the empty fields are not saved. The schema stays exactly §9.30. Logged in the open domain questions.

Implementation notes (2026-10-24):

- Endpoints on the assignment: POST business-data and other-financing with ?to=<step name> (Next, Back to Previous Step, breadcrumb) or ?to=save; GET slik-facilities. moveHotProspect shares saveForm with Prospect Data Entry; Save sets fieldCheck.lenient, which drops only the required-field check.
- fieldCheck gained money (whole rupiah, at most 15 digits) and percent (0 to 100, two decimals, BR-UI-013); number takes whether it is mandatory.
- The customer's SLIK facilities come from the RECEIVED CUSTOMER slik_report; groups ordered by the digits of the condition code; amounts and margin with two decimals for the modal (numeric(20,2)), the card values rounded with round(); Jangka Waktu in full months (end day before start day takes a month off).
- Frontend: HotProspectData.tsx (both pages and the SLIK modal), PercentField and Save in FormFields.tsx, the six-step breadcrumb (labels kept on one line), card errors "Pembiayaan <n> - " in ErrorBanner.
- UI check tools/uicheck/hotprospect.js (screens shots/hot-*.png): Gambar 71 layout with Back to Previous Step disabled, 15 ERR-010 on an empty Next, Lama Usaha limited to 5 digits, Save staying without errors, Next storing the business; Gambar 72 with Margin "21" shown "21,00"; Gambar 73 modal with "Fasilitas Aktif (2)" then "Fasilitas Lunas (1)", the header tick ticking both, unticking one clearing the header and ticking it back setting it; Submit replacing the cards (Jangka Waktu 156, Margin 21,00, Outstanding 9.822.753), Tambah and delete, Next storing two rows and opening Data Permohonan Fasilitas.
- Mutation checks (each fails a test): Save checking required fields; Save moving; customer_business written while incomplete; Jumlah Karyawan and Lama Usaha limits; empty cards kept; rows not replaced; margin over 100; SLIK Collect up to 6; the SLIK button for any customer type; months not full; groups unordered; money not rounded; a step targeting itself.

Review fixes (2026-10-24, approved by the user):

- A SLIK facility without kondisi_ket groups by its condition code and is titled by it ("Fasilitas 09 (1)"); before, every such facility fell into one untitled group.
- A test for a Perorangan Next: the "Usaha" address and customer_business belong to the Nasabah row (FR-APP-016 rule 1).
- Penghasilan Kotor Pertahun (ticket 17) now uses fieldCheck.money, which takes whether the amount is mandatory; BusinessDataForm.address() reuses trimAddress and stayAddress of the other address blocks.
- Named fields in place of positional ones (other_financing scan, SLIK facility rows); frontend sectionNumber(prefix, section) replaces companyOfficerNumber and otherFinancingNumber.
- Disabled Back to Previous Step shown grey; "21,00%" with the sign right after the number.
- GLOSSARY: Other Financing and SLIK Facility.
- Kept as they are, noted: other_financing_case_id_idx is an index §9.31 does not list (lookups by case; no column or constraint added); a failed load of the SLIK modal shows unavailableMessage until TBD-053 names a message; a SLIK margin over 100 fills the card but is refused on Next with ERR-037 (BR-UI-013); manual SLIK never creates a slik_report, so the button does not show for it.
- Mutation "no title from the code" added; all 15 mutations fail a test. UI checks hotprospect.js, business.js and prospect.js rerun and pass.

Note from ticket 19 review (2026-10-24, agreed with the user): agreed comment 5 now stops at Data Permohonan Fasilitas: the steps after it open by breadcrumb only once its Next has recorded the recommendation (the product limit is never skipped).
