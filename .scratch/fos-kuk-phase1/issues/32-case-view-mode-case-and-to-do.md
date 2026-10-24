# 32: Case View mode Case + To do

**What to build:** Opening a case by its Case ID shows Case View in mode Case: the To do card lists the active assignments of the case and its direct child cases, with Go only where the operator may open the assignment; opening an assignment is checked on the server.

**Blocked by:** 05: Create Financing Case + Case View frame

**Status:** ready-for-agent

- [ ] Every `AC-FR-CASE-008-*` acceptance criterion in SoT §13 for mode Case passes
- [ ] Every `AC-FR-CASE-009-*` acceptance criterion in SoT §13 passes
- [ ] Go shows for: worklist assigned to the operator; work queue of the operator; any active assignment for Authors (FR-CASE-009)
- [ ] Opening mode Case does not change any assignment's status or first_opened_at
- [ ] Decide who may open a case by Case ID (TBD-035a, TBD-003)
- [ ] History Details: "History User Recommendation and Officer Activity" with the Recommendation Screen panel, one sub-accordion per Recommendation, oldest first, header "<nama> - <role>" (AC-FR-APP-010-11; moved from ticket 08)
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

Note from ticket 19 (2026-10-24): the Recommendation Screen also shows the Data Permohonan Fasilitas recommendations (AC-FR-APP-018-14, Gambar 78): the diajukan, direkomendasi and disetujui groups (0 shown "Rp0,00" and "0,00000%"), Status Rekomendasi by its RECOMMENDED description, Notes.
