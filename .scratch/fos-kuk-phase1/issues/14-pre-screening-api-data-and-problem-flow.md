# 14: Pre-Screening API Data view + Problem Flow

**What to build:** Operators can see the latest request and response of each Pre-Screening Check. A system failure in a step that has no retry creates a Problem Flow Assignment in the Broken Process work queue, which Authors can resume.

**Blocked by:** 09: Pre-Screening Check executor + Dukcapil

**Status:** ready-for-agent

- [ ] Every `AC-FR-PSAPI-005-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-FLOW-001-*` acceptance criterion in SoT §13 passes
- [ ] Only the seven Pre-Screening Check processes are shown, one per process, latest attempt only
- [ ] Only Authors can see and resume Broken Process items
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

Note from ticket 09 (2026-10-23): the UI-020 table on the Retry view has no sort/filter menus yet; add them here with History Details. A run stopped STOPPED_BROKEN (missing setting or secret) should raise the Problem Flow assignment (FR-FLOW-001).

Moved here from ticket 11 (2026-10-23): View SLIK on the SLIK / SLIKPasangan RESPONSE tab (AC-FR-PSAPI-010-05; the PDF comes with ticket 34).

Moved here from ticket 13 (2026-10-24): the Pefindo / PefindoPasangan sub-accordions of UI-021 (AC-FR-PSAPI-014-06: only Pefindo and PefindoPasangan shown, data ordered by highest Similarity Score, Download PDF only when the file exists), the Reject banner on the Pefindo sub-accordion (AC-FR-PSAPI-017-08) and the Download PDF button (AC-FR-PSAPI-018-06).
