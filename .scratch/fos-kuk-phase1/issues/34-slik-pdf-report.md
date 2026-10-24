# 34: SLIK PDF report

**What to build:** When a SLIK result is received, the system renders the "Informasi Debitur" PDF for the Customer or Spouse and stores it as the case's SLIK_PDF_CUSTOMER / SLIK_PDF_SPOUSE document (PreScreeningResponse), referenced by slik_report.pdf_document_id.

**Blocked by:** 11: SLIK request, callback and rejection

**Status:** needs-triage

- [ ] Every `AC-FR-PSAPI-012-*` acceptance criterion in SoT §13 passes
- [ ] The PDF parts of `AC-FR-PSAPI-010-07` pass (PDF slots replace the old document, shown in the attachments panel)
- [ ] The PDF is stored in the same transaction that marks the slik_report RECEIVED (DB-RULE-058)
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

Split out of ticket 11 (2026-10-23). Blocking questions before work: the PDF library (a new dependency), fonts (TBD-045), CustomerCode in the file name (TBD-044).

Moved here from ticket 12 (2026-10-24): AC-FR-PSAPI-010-06, the copy of the automatic SLIK PDF into Dokumen Pre-Screening when Document Underlying Upload reopens (DB-RULE-097 (1)).

## Comments

Agreed with the user before implementation (2026-10-24):

1. Library and font (TBD-045a, h): github.com/go-pdf/fpdf v0.9.0 (pure Go, MIT, no cgo; archived but stable). Tahoma is replaced by Helvetica, a PDF core sans-serif font needing no font file. No logos (SoT). Content flows onto new pages; a table row and a 24-month grid row are never split, a facility block may continue on the next page.
2. REF-SLIK-01/02/03 are not in the repository: the PDF is built from FR-PSAPI-012's text, which says it replaces them; a sample PDF is kept for the user to look at. Comparing with the legacy PDF waits for REF-SLIK-02.
3. File name (TBD-044b): CustomerCode is work.customer_code (as for Pefindo): "SLIK Report - <code>.pdf" and "SLIK Report - <code> - Spouse.pdf".
4. TBD-045 items as the SoT proposes: (b) Kualitas shows kualitas; (c) Fasilitas Lainnya uses Sec as legacy, plafonEfektifLain and bakiDebetLain are not shown; (d) Tanggal Dibentuk is header.tanggalHasil; (e) lc, garansiYgDiberikan and fasilitasLain are not shown, as legacy; (f) guarantor field names as legacy, an empty Keterangan shows "-"; (g) the search keywords are filled from parameterPencarian.
5. The PDF is rendered and uploaded to GCS before the transaction and stored in the transaction that marks the report RECEIVED (DB-RULE-058). When rendering or the upload fails, the result is not processed: Code 1, and the RPA may send it again (as for the ZIP).
6. Copy into Dokumen Pre-Screening (AC-FR-PSAPI-010-06, DB-RULE-097 (1)): each time Document Underlying Upload opens, while the SLIK row of Dokumen Pre-Screening is empty and the Customer's automatic SLIK PDF exists, a new GCS object and row (DokumenPreScreening, document_type SLIK) are made with the source's names, content type and size; created_by is the operator who opens it. A failed copy leaves the page open without it, is logged, and is tried again on the next opening.
7. Tests at the HTTP seam read the stored PDF from the fake GCS and extract its text with compress/zlib (no new test dependency): the sections in order, "9 Mei 2023", "Rp 485,000,000.00", the grid label "Sep 24", the A3 page size.

Implementation notes (2026-10-24):

- The PDF is drawn from the received file's JSON, the same content summary_json and detail_json keep (FR-PSAPI-012 Sumber Data). Rendering is pure; PBSResponse renders it before storing anything, uploads it beside the ZIP before the transaction and slots it as SLIK_PDF_CUSTOMER / SLIK_PDF_SPOUSE (created_by: the RPA account, DB-RULE-064) in the transaction that sets RECEIVED and slik_report.pdf_document_id.
- The 24-month grid's label column is 40 mm so "Kolektabilitas/Jumlah" stays on one line; long values wrap inside their cell and a row moves whole to the next page.
- The copy into Dokumen Pre-Screening happens when an editable Document Underlying Upload page is built. Its insert only runs while the SLIK row is still empty (an upload in between wins); that guard is race-only and survives a sequential mutation check.
- Tests read the stored PDF from the fake GCS and its text from the zlib content streams (no new test dependency). A sample of the rendered PDF was checked visually in Chrome's PDF viewer: sections, grid (empty month 03, white 0/0), formats and disclaimer as FR-PSAPI-012 describes.
- Mutation checks (each fails a test): no pdf_document_id; PDF not slotted; English month in dates; no thousands separator; "Jan 2006" grid labels; no "Data Tidak Ditemukan"; no copy on opening.
- No UI check: the PDF is a document, not a screen; the View SLIK button is ticket 14.

