# 12: SLIK SLA, Manual SLIK Entry and bypass

**What to build:** When a SLIK result does not arrive within its SLA, or the bypass scenario is enabled and a SLIK request fails, the System records an 'Input Manual' Recommendation and the Financing Case returns to the Branch Officer for Manual SLIK Entry. While waiting, the Branch Officer can check SLIK status.

**Blocked by:** 11: SLIK request, callback and rejection

**Status:** ready-for-agent

- [ ] Every `AC-FR-PSAPI-011-*` acceptance criterion in SoT §13 passes, except AC-FR-PSAPI-011-03 (ticket 31)
- [ ] Every `AC-FR-PSAPI-019-*` acceptance criterion in SoT §13 passes
- [ ] The SLIK SLA (default 1 hour) and the bypass flag are read from Dynamic System Settings
- [ ] A bypassed retry writes a Bypass log row without calling SLIK
- [ ] Create is refused until a SLIK document is uploaded on a case returned for Manual SLIK Entry (ERR-036)
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

Moved here from ticket 11 (2026-10-23): creating the SLIKWQFlowAct assignment when the run enters WAITING_SLIK (FR-PSAPI-011 step 1, FR-PSAPI-013) and completing it on a result; AC-FR-PSAPI-010-06 (copy of the automatic SLIK PDF into Dokumen Pre-Screening when Document Underlying Upload reopens; needs ticket 34's PDF).

## Comments

Agreed with the user before implementation (2026-10-24):

1. Split: this ticket keeps the SLA timeout (60-second sweep), the bypass, the System "Input Manual" Recommendation, the return to the Branch Officer, ERR-036, the Timeout SLIK variant of UI-017 and Sesuai going on at Pefindo. The SLIKWQFlowAct assignment (created on WAITING_SLIK, completed on all results, a reject or the timeout), UI-023 and SLIKInquiry (FR-PSAPI-013, AC-FR-PSAPI-013-*) move to ticket 36. AC-FR-PSAPI-011-03 (SLIK Worst Collect on Document Underlying Upload) moves to ticket 31 (the same field as the Business Entity variant). AC-FR-PSAPI-010-06 moves to ticket 34 (needs its PDF).
2. PROPOSED items used as interim decisions: run states STOPPED_SLIK_TIMEOUT and STOPPED_SLIK_BYPASS; setting process/slik/bypass_enabled (false); the "System" operator (FR-PSAPI-011 rule 4); decision_code INPUTMANUAL on BMV FAC_DECISION (DB-RULE-065), seeded Inactive so the Team Lead's dropdown does not offer it while the banner and history still read it; Sesuai going on at Pefindo without repeating SLIK (TBD-048g).
3. Return to the Branch Officer: a new Pre-Screening Submission assignment for branch_officer, as Tidak Sesuai does; Document Underlying Upload follows through Next. case_status stays Pending-Prescreening (TBD-044e).
4. System operator: user_id "System", full_name "System", new role "System"; it cannot log in: its password_hash can never match and it is not in the directory.
5. ERR-036 (TBD-048f): Individual cases only; the Customer's SLIK document only (document_type SLIK, category DokumenPreScreening), the Spouse's is not required; shown as an item of the Notes banner like ERR-020; applies while the case's latest run is STOPPED_SLIK_TIMEOUT or STOPPED_SLIK_BYPASS.
6. Timeout SLIK variant of UI-017 (TBD-048g): the API Log History panel lists the latest run's SLIK, SLIKPasangan and SLIKPBSResponse rows (the timeout or Bypass row included), without the Retry view's two text lines. Sesuai resumes the same run at Pefindo, attempt 1; Dukcapil, DHN, Rejected List and SLIK are not repeated.
7. Bypass: the row has the failed subject's api_name (SLIK or SLIKPasangan), status "Bypass", empty request and response. It replaces only a retry that would have run (with retry_max 0 a Fail stands). An RPA callback after a bypass is refused with Code 1 like one after the timeout.
8. Fix from ticket 13: the Retry view after a Pefindo failure lists the attempts of the failed API (current_api, e.g. PefindoGetReport), not of Search.

Implementation notes (2026-10-24):

- The 60-second recovery sweep also times out SLIK waits: under the run's row lock (which PBSResponse takes after the report's), one timeout row (Service Rest, SLIKPBSResponse, Fail, no request), STOPPED_SLIK_TIMEOUT and the hand-back, in one transaction. A run whose results are all in is left alone.
- The bypass is decided where a retry would be scheduled: the failed attempt's row and the Bypass row are written with STOPPED_SLIK_BYPASS and the hand-back in one transaction. A Connect API Log row with an empty response now stores NULL (only the Bypass row has none).
- The Timeout SLIK variant and ERR-036 key on the case's latest run being STOPPED_SLIK_TIMEOUT or STOPPED_SLIK_BYPASS; Sesuai then resumes that same run at the step after SLIK Pasangan (Pefindo), so the run keeps one history.
- ERR-036 is a form-level error (section form, key slikDocument, no field), so the Notes banner shows "Upload Dokumen SLIK" alone.
- Tests trigger the 60-second check by starting another backend on the same database (it sweeps at start), with the deadline set in the past.
- Mutation checks (each fails a test): no timeout in the sweep; the run not stopped on timeout; no ERR-036; Sesuai starting a new run instead of resuming at Pefindo; no bypass on SLIK; bypass not treated as timeout on PBSResponse; bypass not counted as Manual SLIK Entry; the Retry view on the step's API instead of current_api.
- UI check `tools/uicheck/slik.js`: ERR-036 banner on Document Underlying Upload, and the Timeout SLIK variant of UI-017 (API Log History above History Pre-screening Notes, no Retry text, decision fields kept).

Review (2026-10-24), proposals the user approved ("ya") and that were applied:

1. Bug: an RPA callback after a bypass is accepted once the Team Lead's Sesuai resumes the run (refuseSLIK keys the bypass on the run state, which is RUNNING again); a late Customer result could then reject the case during Pefindo. Fix: refuse on the case's Bypass log row (status Bypass on SLIK/SLIKPasangan), as the timeout uses its log row; add a test of a callback after the resume.
2. Log "SLIK wait timed out" only after the commit succeeds.
3. One shared list of the Manual SLIK Entry states (timeout, bypass) instead of three spellings (pbs.go, inManualSLIKEntry, resumeAfterManualSLIK).
4. Rename RetryView to a neutral APILogView (field SLIKTimeout keeps the SoT variant name); drop the duplicate nil-to-empty conversion in review.go.
5. GLOSSARY.md: Manual SLIK Entry also follows a bypass; add the term SLIK Bypass (FR-PSAPI-019, DEC-106).
6. Open domain question: §9.15 does not list STOPPED_SLIK_BYPASS.
7-8. No change (agreed or not needed): SLIKWQFlowAct completion (ticket 36), only Pre-Screening Submission (decision 3), no bypass with retry_max 0 (decision 7), migration 0013 seeds as written.

Review fixes applied: the PBSResponse refusal after a timeout or bypass now reads their log rows (the timeout row, a Bypass row on SLIK or SLIKPasangan), which stay when the run goes on, and no longer the run state; test TestResultAfterABypassStaysRefusedOnceTheRunGoesOn (fails when the Bypass clause is removed). The timeout is logged after its commit. manualSLIKStates is the one list of Manual SLIK Entry states. RetryView is APILogView. GLOSSARY.md: Manual SLIK Entry also follows a SLIK Bypass, a new term. §9.15 lacking STOPPED_SLIK_BYPASS is in the open domain questions.

