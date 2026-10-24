# 17: Prospect Data Entry: Individual

**What to build:** For an Individual Customer, the Branch Officer records Data Pribadi, then Data Pasangan, then Data Keluarga Dekat.

**Blocked by:** 15: Pipeline Data Entry

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-012-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-APP-013-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-APP-014-*` acceptance criterion in SoT §13 passes
- [ ] NIK is 16 digits, NPWP 15-22 digits, age at least 17, names at most 30 characters
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

Agreed with the user before implementation (2026-10-24):

1. Label conflict: Gambar 51 shows "Alamat Tempat Bekerja", FR-APP-012 Field 5 and UI-026 Language say "Alamat Tempat Kerja". The text wins (UI-026 Reference).
2. Conflict FR-APP-013 Process 3 vs DB-RULE-100/101: when Status Perkawinan is not MS-2, the Pasangan row stays with its Pre-Screening data (NIK, name, birth date); only the Prospect columns (birth_place, job_code, work_duration, mobile_phone, education_code, prenup_code) are cleared and the "Kantor" address deleted. Logged in the open domain questions.
3. Moves (TBD-052, PROPOSED): Next and a breadcrumb click to any step validate and save the active form; passing completes it and creates the target for the same operator, opened at once (FR-ASSIGN-001 rule 5). Back (Data Pasangan to Data Pribadi, Data Keluarga Dekat to Data Pasangan) does the same. A reopened page is filled from the saved data.
4. Close only on Data Pribadi (as the Gambar): back to the Dashboard without saving (PROPOSED).
5. Create on Data Keluarga Dekat (FR-APP-014 Process 3; TBD-053b): completes it, the case enters stage Financing Application, case_status Pending-Submission, and a new (baru) "Data Usaha Nasabah" assignment with its process is created for branch_officer; the page returns to the Dashboard (its page is ticket 18).
6. New tables exactly as the SoT: registered_customers and existing_customers (§9.27, PK (id_number, customer_type)) and customer_address (§9.29, PK (customer_id, address_type), FK to customer, BIGINT as customer). No seeds.
7. Customer status: worked out on every opening (NIK and "Perorangan" against both tables); is_existing_customer is stored only when the owner opens the unfinished assignment; an Authors look shows the banner and stores nothing.
8. BMV master data is not seeded (import); tests and smoke add their own. "MS-2" is a literal (DEC-128); MARITAL_STATUS Value1 goes to marital_status_pbs_code.
9. Regions: Provinsi loads with the page; Kota, Kecamatan, Kelurahan and Kode Pos via GET .../regions?type=&parent= (Active, sort_order then code); Dati II from the Kota's Value1, Kode Pos the first POSTALCODE code of the Kelurahan. On Next the server checks the chain and works out Dati II and Kode Pos itself.
10. Bidang Usaha Tempat Bekerja autocomplete (BR-UI-005): contains on code or description, case-insensitive, max_results, 300 ms; the list shows code then description (Gambar 50); the field shows the description, the code is stored.
11. Messages: ERR-040 "<Nama Field> maksimal <n> karakter" (RT/RW over 3 digits too); ERR-037 for invalid values, Lama Tinggal over its limit and Penghasilan over 15 digits; phone rules FR-APP-005 rule 8, email rule 9, NPWP ERR-011/012. Error order by the Field numbering, left column then right. Data Pribadi banner prefixes "e-KTP - " and "Domisili - " (PROPOSED). The spouse NIK is not compared with the customer's.
12. Alamat Surat Menyurat kept as it is; empty on save means the first BMV MAILING_ADDRESS option. Checked "sama dengan e-KTP": Domisili saved as a copy of e-KTP, sent values ignored.
13. Out of scope: the blank Details while the assignment is active (Details ticket), sending the marital-status Value1 at disbursement, Hot Prospect content (ticket 18).
14. Seams: HTTP API against real PostgreSQL (Pipeline Submit to Data Pribadi, page and customer status, validation of the three forms, e-KTP copy and mailing default, Next/Back/breadcrumb, the not-married branch, Create to Financing Application, the regions and sector endpoints); UI with tools/uicheck against Gambar 51-56.

Implementation notes (2026-10-24):

- Endpoints on the assignment: POST personal, spouse and family with ?to=<step name> (Next, Back, breadcrumb) or ?to=create (Data Keluarga Dekat only); a step cannot target itself; each takes only its own form's fields (unknown field: 400). GET regions?type=CITY|DISTRICT|SUBDISTRICT|POSTALCODE&parent= and GET economic-sectors?q= for whoever may open the assignment.
- One check per field in layout order (fieldCheck); the region cascade is checked against the BMV parent (sub_type) and the server stores Dati II (the Kota's Value1) and Kode Pos (the Kelurahan's first POSTALCODE). Below an invalid unit nothing is judged again; an empty mandatory unit is still ERR-010.
- validatePerson was split into nikError, nameError and birthDateError so Data Pasangan uses the Pre-Screening rules with its own labels; startProcess and newBranchOfficerAssignment now take the stage.
- The Autocomplete component is shared by Nama Bank (ticket 15) and Bidang Usaha Tempat Bekerja (two-line items, Gambar 50); the Breadcrumb fits three steps and, for Prospect, darkens the line up to the active step and bolds the steps passed (Gambar 54-56). Disabled date fields have the grey background.
- UI check tools/uicheck/prospect.js (screens shots/prospect-*.png): Gambar 51-53 layout, 43 errors on an empty Next with "e-KTP - " prefixes, "Rp" income shown 15.000.000 with minus and decimals refused, sector "kon" listing "4100 / Konstruksi Gedung" and showing the description, RW kept to 3 digits, Kota disabled until Provinsi, Dati II and Kode Pos filled, domicile hidden when checked and a copy when unchecked, Next with Belum Kawin to "Status Nasabah Tidak Kawin" (Gambar 55), Back refilled from the saved data, Kawin to the prefilled spouse (Gambar 54) with its four ERR-010, Data Keluarga Dekat (Gambar 56) and Create to Financing Application, Pending-Submission, Data Usaha Nasabah new.
- Mutation checks (each fails a test): no mailing default; domicile not copied; no Kode Pos; no Dati II; region parent ignored; not married keeps the Prospect data; Kantor kept when empty; move to itself; Create keeps the stage; customer status not stored; 16-digit phone; marital Value1 not stored; income over 15 digits; sector matched on description only.

