# 36: Waiting SLIK assignment and SLIK status check

**What to build:** While a run waits for SLIK results, the Branch Officer has a "SLIKWQFlowAct" assignment (UI-023) where "Check SLIK Status" asks SFS for the status of each reference code (SLIKInquiry) and shows the API's message. The system completes the assignment when every result arrives, on a SLIK reject, or on the SLA timeout.

**Blocked by:** 12: SLIK SLA, Manual SLIK Entry and bypass

**Status:** needs-triage

- [ ] Every `AC-FR-PSAPI-013-*` acceptance criterion in SoT §13 passes
- [ ] The assignment is created when the run enters WAITING_SLIK (FR-PSAPI-011 Process 1) and completed on all results, a reject or the timeout (DEC-089)
- [ ] UI-023 matches Gambar 44 (no Submit button, DEC-086)
- [ ] SLIKInquiry is logged per request and never retried; it changes no SLIK data, run or case_status
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

Split out of ticket 12 (2026-10-24). Blocking questions before work: TBD-048 (a) display title and "Due" format, (b) meaning of code 1100, (d) the view when results arrive while it is open.
