# 34: SLIK PDF report

**What to build:** When a SLIK result is received, the system renders the "Informasi Debitur" PDF for the Customer or Spouse and stores it as the case's SLIK_PDF_CUSTOMER / SLIK_PDF_SPOUSE document (PreScreeningResponse), referenced by slik_report.pdf_document_id.

**Blocked by:** 11: SLIK request, callback and rejection

**Status:** needs-triage

- [ ] Every `AC-FR-PSAPI-012-*` acceptance criterion in SoT §13 passes
- [ ] The PDF parts of `AC-FR-PSAPI-010-07` pass (PDF slots replace the old document, shown in the attachments panel)
- [ ] The PDF is stored in the same transaction that marks the slik_report RECEIVED (DB-RULE-058)
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

Split out of ticket 11 (2026-10-23). Blocking questions before work: the PDF library (a new dependency), fonts (TBD-045), CustomerCode in the file name (TBD-044).
