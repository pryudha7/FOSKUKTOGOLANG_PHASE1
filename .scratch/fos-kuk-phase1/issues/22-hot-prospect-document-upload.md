# 22: Hot Prospect: Document Upload

**What to build:** The Branch Officer uploads the Hot Prospect documents, completing Hot Prospect Data Entry.

**Blocked by:** 21: Hot Prospect: Collateral

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-022-*` acceptance criterion in SoT §13 passes
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

Proposed to the user on 2026-10-26, NOT yet answered (ask again before implementing; record the answers here as "Agreed with the user"):

1. Seed BMV DOCUMENTCATEGORY with the seven rows the SoT lists (DOCCAT-1 to DOCCAT-6 and DOCCAT-10, sub_type empty).
2. Interim Document Mapping Value from Gambar 93 until the master import (as ticket 07): Dokumen Karakter: SLIK (Mandatory, 1), PEFINDO (Conditional, 2), File Dokumen Karakter (Mandatory, 3); Dokumen Pengajuan, Underlying, Dll: File Dokumen Pengajuan, Underlying, Dll (Conditional, 8). Categories without types (Legalitas, Jaminan, Capacity, Capital dan Condition) still show as accordions with an empty table (TBD-059b).
3. No Tipe Nasabah filter on the categories (TBD-059b): one list for Perorangan and Badan Usaha.
4. The list is a snapshot formed once (DOCCAT-1 excluded through DSS), only growing; on every opening the DB-RULE-097 copies incl. its PROPOSED parts: SLIK and Pefindo into the master rows of Dokumen Karakter; the other Pre-Screening types into Dokumen Pengajuan, Underlying, Dll with their files; names matched case-insensitively; Mandatory and sort_order copied from Pre-Screening; master rows first on equal sort_order; each copy its own row and GCS object.
5. Upload through the existing Upload Document modal (Category disabled with the category name); replaces only that category's file (DB-RULE-031).
6. Submit: ERR-020 over 100 MB (DSS; Document Upload categories only, copies included) and ERR-026 per Mandatory row without a file plus a warning icon on the accordion header, both at once as UI-011. Passing completes the assignment; nothing opens next (Survey Account Officer, TBD-053, ticket 23), the case stays in Financing Application and the operator returns to the Dashboard.
7. Save only stays (no fields; uploads are saved at once); Back to Previous Step and the breadcrumb open their step without checks; Back returns to the Dashboard.
8. Download All Attachments: "<Case ID>.zip" with every file of the case, streamed without temporary files; pdf, jpg, jpeg, png, zip, rar stored, others deflated; one folder per category (e.g. DOCCAT-6/KK.pdf, DokumenPreScreening/KK.pdf), clashing names numbered (TBD-059c); access as single-file download (DEC-082).
9. ERR-045 "Unduhan sedang diproses, silakan coba lagi beberapa saat": one active ZIP per case and at most 20 at once system-wide (DSS), 300 s maximum (DSS), enforced with PostgreSQL advisory locks so it holds across instances.
10. Seed DSS validation/upload/document_upload_max_total_bytes 104857600, validation/upload/document_upload_excluded_category_codes DOCCAT-1, validation/download/zip_max_concurrent 20, validation/download/zip_timeout_seconds 300.
11. A failed list initialisation (ERR-027, text TBD) shows the page with what the case has and the general message, as ticket 07.
12. Out of scope: AC-FR-APP-022-12 (tab DATA DOKUMEN on Details; the Details tabs have no ticket).
13. Seams: HTTP API against real PostgreSQL (list forming and snapshot, the SLIK/Pefindo/Pre-Screening copies, replacing upload, Submit ERR-020/ERR-026, ZIP content and compression, ERR-045 per case and system-wide, moves); UI with tools/uicheck against Gambar 93.

Reuse found while reading: cases/documents.go (initDocuments, documentList, copySLIKReport, categoryCode), cases/upload.go (UploadDocument, storeUpload, checkUploadTarget: today limited to Document Underlying Upload), cases/create.go (ERR-020 total and ERR-026 checks, totalMB, bytesPerMB), constants categoryPreScreening "DokumenPreScreening", categoryResponse "PreScreeningResponse", docPDFCustomer "SLIK_PDF_CUSTOMER", docPefindoCustomer "PEFINDO_PDF_CUSTOMER", docSLIK "SLIK". For Document Upload, document_category stores the BMV code (DOCCAT-n), not a codename.

