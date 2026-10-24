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

## Comments

Agreed with the user before implementation (2026-10-24):

1. Title "SLIKWQFlowAct" as on Gambar 44 (TBD-048a). Due is computed from the run's wait_deadline_at when the page opens (not a live countdown): "Due <n> minutes from now", n the whole minutes left rounded down, at least 0.
2. Code "1100" only shows its message; the system does nothing else (TBD-048b, FR-PSAPI-013 rule 7 PROPOSED).
3. The assignment is a worklist item of branch_officer, on To do like other worklist items; it is created in the transaction that puts the run in WAITING_SLIK and completed by the system when every result is in, on a SLIK reject, on the timeout, or when the case stops otherwise (TBD-048d). A page left open after that keeps showing; the next "Check SLIK Status" is refused and shown as the general error message; reopened, the page is read-only without the button.
4. SLIKInquiry settings (§9.25): api/slik/baseurl and secret/slik/apikey_ref (x-api-key) shared with SLIK; new rows api/slikinquiry/path /v1/slik/status/{KodeReff}, timeout_ms 30000, retry_max 0, simulate false, simulate_response {"code":"4040","message":"data not found"} (SoT sample).
5. Message: the response JSON's message; with no body or a timeout, the text logged as Response. The Spouse is asked when a Spouse slik_report exists; Customer then Spouse, one after the other, a failure of one not stopping the other. The button is disabled while the requests run (PROPOSED).

Implementation notes (2026-10-24):

- The assignment opens in the transaction that sets WAITING_SLIK (run.go advance) and completes in the PBSResponse transaction unless a result is still awaited, and in the timeout transaction. A case that reaches Pefindo with every result already in never gets one.
- Check SLIK Status takes the assignment only to check its holder, then calls without holding it. Each call is logged in its own transaction; a failed log write still shows the message and is reported as API_LOG_WRITE_FAILED (as PBSResponse).
- outboundAPI has a secret field: SLIKInquiry reads SLIK's secret/slik/apikey_ref; base and secret default to the API's own settings.
- Due rounds down: right after the run starts waiting it may read 59 (the stored deadline keeps microseconds).
- UI check tools/uicheck/waitslik.js against Gambar 44: header, Due, centred Check SLIK Status, the Message panel hidden until the first check, two titled panels with a Spouse, Close, no Submit; screenshots shots/waitslik-*.png.
- Mutation checks (each fails a test): no assignment on WAITING_SLIK; not completed by the last result, by a reject or by the timeout; completed while a result is still awaited; a simulated answer not judged; the logged Response shown instead of the message; Check SLIK Status on another assignment.

