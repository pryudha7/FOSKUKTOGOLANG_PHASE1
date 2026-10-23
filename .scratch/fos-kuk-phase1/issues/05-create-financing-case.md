# 05: Create Financing Case + Case View frame

**What to build:** A Branch Officer chooses Create, then Financing. A Financing Case is created with its Case ID, Customer Code, Team Lead and branch, and opens in Case View (mode Assignment) on the Pre-Screening Submission assignment: stage bar, case information panel, attachments panel and the Case lifecycle popup work, and the assignment page shows Nama Branch and Tipe Nasabah.

**Blocked by:** 03: Access control and endpoints, 04: Layout and navigation

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-001-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-CASE-001-*`, `AC-FR-CASE-002-*`, `AC-FR-CASE-004-*`, `AC-FR-CASE-005-*`, `AC-FR-CASE-007-*` acceptance criterion in SoT §13 passes
- [ ] The `AC-FR-CASE-008-*` and `AC-FR-ASSIGN-001-*` criteria for mode Assignment and the first assignment pass (mode Case and To do: ticket 32)
- [ ] Case IDs come from a per-prefix sequence (LF-n), atomic and unique
- [ ] Customer Code is SMSME + two-digit year (creator's timezone) + Case ID number padded to eight digits
- [ ] Team Lead is the creating Branch Officer's report-to operator; branch is the creator's Branch Code, its name read from BMV BRANCH
- [ ] Only AO and SAO create cases; anyone else, Authors included, gets ERR-022 and nothing is created
- [ ] Choosing "Financing" in the sidebar Create menu starts the case (AC-FR-NAV-003-03, moved from ticket 04)
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

**Split (2026-10-23, agreed):** the original ticket 05 was split into 05 (this), 30 (Perorangan form), 31 (Badan Usaha form) and 32 (mode Case + To do).

**Decisions (agreed):**

- The case is created when "Financing" is chosen (TBD-023a); Close returns to the Dashboard and leaves case and assignment in place (TBD-023b).
- An empty report-to leaves `branch_officer_team_lead` empty (TBD-029c); an empty or unknown Branch ID leaves `branch_code` empty and shows ERR-006, with submit blocked in ticket 30 (TBD-026).
- One PostgreSQL sequence per prefix (LF, PV) starting at 1; gaps after failed transactions are tolerated (TBD-022).
- Work gains `customer_type`, holding the BMV CUSTOMER_TYPE **description** per §9.6 (more specific than BR-UI-012); filled on submit (ticket 30).
- New cases start with Case Status `New`; a process counts as started when its first assignment is created (TBD-034a).
- An assignment opens only for its worklist owner or an Authors operator (FR-CASE-009 Go rules), checked on the server; who may open mode Case is decided in ticket 32 (TBD-035).
- `/api/me` `caseTypes` items are `{code, label}`: code `Loan Financing` (the stored `case_type`), label `Financing`.

**While implementing (interim, to confirm with the SoT owner):**

- ERR-022 text "Anda tidak memiliki akses untuk membuat pengajuan pembiayaan." (403) and ERR-005 text "Case ID gagal dibentuk. Silakan coba lagi." (503); both are TBD in §12 (TBD-032). ERR-006 uses the SoT text and shows under Nama Branch.
- Assignment status values are stored as the SoT words: `baru`, `sudah dibuka`, `sudah selesai dikerjakan`; types `worklist`, `work queue`.
- BMV CUSTOMER_TYPE (Perorangan, Badan Usaha) is seeded by migration as in §9.6; BRANCH and the rest come from the separate BMV import.
- Case View is served at `<desktop>/case/<caseId>/assignment/<assignmentId>`. Until ticket 32 decides TBD-035a, any authenticated desktop operator can read a case's frame; opening an assignment is restricted.

**Not in this ticket:** the History Details accordion (TBD-035, not to be implemented), the Details page (FR-CASE-010), attachments list content (Case Document arrives in ticket 07; AC-FR-CASE-006 then), Phone Verification child cases (TBD-024, so AC-FR-CASE-001-03 and AC-FR-CASE-002-01's PV row have no creation path yet), mode Case and To do (ticket 32), the form body and Next (tickets 30, 31).

**Verification:** HTTP API tests cover AC-FR-APP-001-01..06, AC-FR-CASE-001-01/02/04, AC-FR-CASE-004-01..07, AC-FR-CASE-005-01/02, AC-FR-CASE-007-01..03, AC-FR-ASSIGN-001-01/03. The UI was driven in headless Chrome (Create → Financing → Case View, lifecycle popup, hide/show panel, attachments panel) and compared with Gambar 11, 17, 19 and 83.
