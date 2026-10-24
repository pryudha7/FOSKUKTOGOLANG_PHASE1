# 14: Pre-Screening API Data view + API Log History

**What to build:** Operators can see the latest request and response of each Pre-Screening Check. A system failure in a step that has no retry creates a Problem Flow Assignment in the Broken Process work queue, which Authors can resume.

**Blocked by:** 09: Pre-Screening Check executor + Dukcapil

**Status:** ready-for-agent

- [ ] Every `AC-FR-PSAPI-005-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-UI-020-*` acceptance criterion in SoT §13 passes
- [ ] Only the seven Pre-Screening Check processes are shown, one per process, latest attempt only
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

Note from ticket 09 (2026-10-23): the UI-020 table on the Retry view has no sort/filter menus yet; add them here with History Details. A run stopped STOPPED_BROKEN (missing setting or secret) should raise the Problem Flow assignment (FR-FLOW-001).

Moved here from ticket 11 (2026-10-23): View SLIK on the SLIK / SLIKPasangan RESPONSE tab (AC-FR-PSAPI-010-05; the PDF comes with ticket 34).

Moved here from ticket 13 (2026-10-24): the Pefindo / PefindoPasangan sub-accordions of UI-021 (AC-FR-PSAPI-014-06: only Pefindo and PefindoPasangan shown, data ordered by highest Similarity Score, Download PDF only when the file exists), the Reject banner on the Pefindo sub-accordion (AC-FR-PSAPI-017-08) and the Download PDF button (AC-FR-PSAPI-018-06).

## Comments

Agreed with the user before implementation (2026-10-24):

1. Split: this ticket keeps UI-020 API Log History (sort, filter, "Lihat semua"/"Sembunyikan") and UI-021 Pre-Screening API Data with all its variants (Dukcapil, DHN, Rejected List, SLIK with View SLIK, Pefindo with Download PDF, the Reject banner); the Retry view and the Timeout SLIK variant use the same UI-020 table. The Problem Flow (FR-FLOW-001, UI-022, §9.17, the Broken Process work queue, Authors access) moves to ticket 37.
2. History Details is the accordion card under the assignment card (mode Assignment, UI-013), accordions closed at first; for now only API Log History and Pre-Screening API Data (the other accordions come with their tickets, TBD-033/035). Whoever may open the assignment sees it.
3. UI-020 (TBD-042k): newest first (Gambar 36), no pages; sort and filter run in the browser over all the case's rows (so over all data; the PROPOSED server-side run is not needed without pages); filters of several columns combine with AND; long values in the filter popup are cut like the cells; the cut limits are read from Dynamic System Settings ui/apilog/truncate_chars 200 and ui/apilog/truncate_lines 5 (new seed rows); every connect_api_log row of the case is listed (Service Rest, SLIKInquiry and Bypass included).
4. UI-021: a process without any log row is not shown (TBD-042), except Rejected List as FR-PSAPI-005 rule 6 says; fields as FR-PSAPI-005 rules 5-11; SLIK codes show their BMV description without a status filter; a missing value shows "--"; Tanggal Lahir shows as the JSON log has it (Gambar 38); Pefindo shows the Search row's fields only (TBD-049).
5. View SLIK and Download PDF use the existing case document download and its access rule (FR-FILE-003, ERR-035 popup).

Implementation notes (2026-10-24):

- GET .../assignments/{id}/history answers the History Details card for whoever may open the assignment (worklist holder or Authors, as opening it): every log row of the case newest first, the cut limits, and the Pre-Screening API Data processes built from the latest row per api_name (DISTINCT ON).
- The Pefindo process mapping (Search, Generate Report, Download PDF, Get Report per subject) comes from pefindoSubjects, the one place the API names are set (FR-PSAPI-005 rule 7 calls it configuration; it is not a Dynamic System Setting).
- Pefindo result fields read either naming of the SoT (the sample's nama_debitur, tanggal_lahir, alamat, ... and FR-PSAPI-014's debtor_name, birth_date, address, ...).
- Waktu Connect now shows dd/MM/yy HH:mm (UI-020) on the Retry and Timeout SLIK views too; their tables get the same column menus and cut limits.
- The open/close state of accordions is per page view; nothing is kept.
- UI check tools/uicheck/history.js (screens shots/history-*.png): AC-UI-020-01 (sort, filter on Dukcapil, Clear filter), -02 (cut and "Lihat semua" with indented JSON), the Dukcapil Reject banner on REQUEST, SLIK codes as descriptions, Pefindo results highest Similarity Score first.
- Mutation checks (each fails a test): Pefindo banner only from Search; Rejected List always shown; results not sorted; SLIK codes not described; oldest first; DHN message from the response; no access check; Dukcapil reject not shown; DHN shown without a row.

