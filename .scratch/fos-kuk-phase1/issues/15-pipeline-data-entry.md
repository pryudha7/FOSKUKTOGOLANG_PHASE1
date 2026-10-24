# 15: Pipeline Data Entry

**What to build:** After passing the Pre-Screening Check, the Branch Officer records the Submission Type and Product; for a Take Over they also record the other bank and may upload its letter.

**Blocked by:** 10: DHN + Rejected List

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-004-*` acceptance criterion in SoT §13 passes
- [ ] Dropdowns store the BMV code and display its description
- [ ] Contract is copied from the selected Product
- [ ] Changing the Submission Type away from Take Over deletes the bank letter
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

Agreed with the user before implementation (2026-10-24):

1. When the Pre-Screening API run ends COMPLETED, the same transaction creates the "Pipeline Data Entry" assignment (worklist, new) for branch_officer, sets case_status "Pending-Submission" and records the Pipeline Data Entry process in case_process_progress (FR-ASSIGN-001 rule 14, AC-FR-APP-004-01).
2. Submit (TBD-052a/b): completes the assignment and creates "Badan Usaha" (Badan Usaha case) or "Data Pribadi" (Perorangan case) for branch_officer per FR-ASSIGN-001 rules 15/16, with its process progress; case_status stays "Pending-Submission". Their pages come with tickets 16/17.
3. Close (PROPOSED, TBD-052c): closes the page without saving; uploaded documents stay.
4. Changing Jenis Pengajuan (rule 3, DB-RULE-090): every change calls the server at once; one transaction clears bank_name and deletes the Bukti Surat Case Document row, the GCS object is deleted after commit. The new value is saved only on Submit. A failed delete puts the dropdown back and shows the existing system-failure banner; a failed GCS delete after commit is only logged, as DB-RULE-031 does today.
5. product_management is created exactly as §9.26 (DB-RULE-088: all rows, code ascending); no product rows are seeded (TBD-052h); tests and smoke use the §9.26 test products 794/798.
6. Seeds: DSS ui/autocomplete/max_results = 50 (PROPOSED §9.25) and the Document Mapping Value row category "Dokumen Inisiasi", sub_category "TO", description "Bukti Surat Take Over dari Bank". BMV SUBMISSION_TYPE and BANK are not seeded (SoT names only the "Take Over" code); tests and smoke add their own.
7. Nama Bank autocomplete (BR-UI-005): direct DB query, contains, case-insensitive, limited by max_results, sort_order then code; 300 ms debounce and empty text shows the first 50 (PROPOSED); matching letters bold, arrow keys and Enter; free text is not validated.
8. Bukti Surat upload (PROPOSED): uploaded when chosen; file_name = "Bukti Surat Take Over dari Bank" + original extension; file type (ERR-021) and validation/upload/max_file_bytes (ERR-041) as FR-FILE-002; choosing again replaces (DB-RULE-031); ERR-041 on the page banner with the ticket 07 text; no manual delete button (TBD-052h).
9. Submit validation: ERR-010 "<Nama Field> wajib diisi" for Branch, Jenis Pengajuan, Produk, Nama Bank (Take Over only); ERR-037 for a Jenis Pengajuan or Produk not in its source; on the banner and the field. Branch is checked Active on open and on Submit; not Active clears branch_code.
10. Badan Usaha variant: label "NPWP Badan Usaha *" with the NPWP Badan Usaha document item, as the text says (no Gambar).
11. Out of scope: rule 11 (blank Details while the assignment is active) goes with the Details ticket (FR-CASE-008); Product Management maintenance (TBD-052h).
12. Seams: HTTP API against real PostgreSQL with GCS faked at the network boundary (run COMPLETED -> assignment; page GET; bank autocomplete; Bukti Surat upload, replace and delete; Submit); UI checked with tools/uicheck against Gambar 45-47.

Implementation notes (2026-10-24):

- The completed run's last transaction also creates the Pipeline Data Entry assignment (new, branch_officer), sets case_status Pending-Submission and records the process (startProcess, now shared with case creation).
- Endpoints on the assignment: GET (page), POST pipeline (Submit), GET banks?q= (autocomplete), POST submission-type-change (rule 3), POST bank-letter (upload). All but GET are for the owner of the open Pipeline Data Entry only.
- The upload shares the Upload Document checks and storage order (storeUpload, split out of UploadDocument): category DokumenInisiasi, type and name "Bukti Surat Take Over dari Bank" plus the original extension, replacing an earlier one.
- A Branch not Active is cleared when the owner opens the page and on Submit; a refused Submit still keeps the clearing (DB-RULE-089).
- The UI clears Nama Bank when Jenis Pengajuan changes and locks it until the server answers, so text typed meanwhile is not wiped; a failed change puts both back with the system-failure banner.
- Gambar 46: the grey clip sits under the Attachment button until a file is stored (Gambar 47 shows the item instead).
- UI check tools/uicheck/pipeline.js (screens shots/pipeline-*.png): Gambar 45 layout and labels, ERR-010 banner and field errors, Take Over showing Nama Bank and Bukti Surat, "bca" suggesting BCA and BCA Syariah with the match bold, Enter choosing, upload showing the item "DokumenInisiasi • Dokumen Inisiasi", a Jenis Pengajuan change deleting the row and clearing Nama Bank, Submit storing Take Over|798|Murabahah|Bank Lain and opening Data Pribadi, one column at 700 px.
- Mutation checks (each fails a test): no Pipeline assignment; contains ignored; case-sensitive match; Nama Bank kept on change; Nama Bank stored for any type; Branch not cleared on opening; products by name; no ERR-037 for an unknown product; Badan Usaha sent to Data Pribadi; letter object kept in GCS.

