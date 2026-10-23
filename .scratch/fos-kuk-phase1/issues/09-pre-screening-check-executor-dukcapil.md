# 09: Pre-Screening Check executor + Dukcapil

**What to build:** A Sesuai Pre-Screening Review starts a background Pre-Screening Check run, beginning with Dukcapil. A Check Rejection sends the Financing Case to Rejected; a Check Failure after retries returns it to the Team Lead with a Retry view. The run survives browser close and process restart.

**Blocked by:** 08: Pre-Screening Review by Team Lead

**Status:** ready-for-agent

- [ ] Every `AC-FR-PSAPI-001-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-PSAPI-004-*` acceptance criterion in SoT §13 passes
- [ ] Submitting Sesuai returns immediately and sets Case Status to Pending-Prescreening
- [ ] Only a Fail is retried (3 retries at 5-second intervals, read from configuration)
- [ ] Dukcapil checks only an Individual Customer, never the Spouse
- [ ] Every API attempt adds a new Connect API Log row; old rows are never changed
- [ ] A Check Rejection sets Case Status to ResolvedRejected-Prescreening and stops all later checks
- [ ] A Check Failure sets Case Status to Failed-PrescreeningConnection; Team Lead Retry restarts from the failed check
- [ ] An interrupted run is resumed by the recovery sweep without running a step twice
- [ ] Tests use a fake Dukcapil that can return Success, Rejected, Fail or time out
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

Agreed with the user before implementation (2026-10-23):

1. A run keeps the whole FR-PSAPI-001 order (Dukcapil, DHN, Rejected List, SLIK, SLIK Pasangan, Pefindo, Pefindo Pasangan). When the next step is not implemented yet, the run stays RUNNING on that step and stops quietly (logged); the recovery sweep skips such runs, so they resume once the step exists.
2. Reject: case_status ResolvedRejected-Prescreening, alternate stage Rejected, run STOPPED_REJECTED. Writing the Rejected List is ticket 10. Reopen (AC-FR-PSAPI-001-10/15) stays TBD-042.
3. Fail after all attempts: case_status Failed-PrescreeningConnection, run STOPPED_FAILED, a new Pre Screening assignment (baru) for the Team Lead. Submit on the Retry view sets Pending-Prescreening and resumes the same run at the failed step with a fresh 4-attempt cycle; it adds no Recommendation.
4. Retry view (Gambar 35; rows TBD-042): panel "API Log History" with its two lines and a table (Nama API, Waktu Connect, Status, Request, Response) of the failed API's attempts in the last cycle, Request/Response cut at 200 characters or 5 lines with "Lihat semua"/"Sembunyikan"; then History Pre-screening Notes, Close and Submit. Sort/filter menus come with ticket 14.
5. API key: DSS `secret/dukcapil/apikey_ref` names an environment variable (e.g. DUKCAPIL_APIKEY) injected from Secret Manager at deploy (DEC-060). Empty is a system failure of the step; the value is never logged.
6. Migration seeds environment-independent settings (path, timeout_ms 30000, retry_max 3, retry_gap_ms 5000, simulate false, masked_headers, apikey_ref). `api/dukcapil/baseurl` is set per environment (DB-RULE-085). A missing required setting: run STOPPED_BROKEN, logged, case_status unchanged (Problem Flow is ticket 14).
7. SoT gap: `x-Gateway-APIKey` is added to the masked headers seed (DEC-060); header names match case-insensitively.
8. New setting `api/dukcapil/simulate_response` (JSON), seeded with the "data ditemukan" example (TBD-042).
9. Response without body: HTTP 401 uses the SoT text; other codes "the server responded with an HTTP <kode> code."; timeout "Connect to <host>:<port> timed out after <n> ms"; connection errors their message without credentials.
10. Executor: goroutines in the backend process; owner_id hostname/pid/random; heartbeat every 15 s; recovery sweep at start and every 60 s with the atomic take-over (DB-RULE-047) and the partial unique index (DB-RULE-046). Tests use a fake Dukcapil and a small retry_gap_ms.
11. Request data from the Nasabah customer row (dd-MM-yyyy), read again on every attempt. TGL_LHR starting "Sesuai" passes, "Tidak Sesuai" rejects, anything else fails.

Implementation notes (2026-10-23):

- The executor lives in `internal/cases` (run.go, dukcapil.go, apilog.go); `app.New` returns an `*App` whose `Close` stops it.
- Retry is `POST .../assignments/{id}/retry`; a decision on the Retry view is refused with 400.
- Two take-over guards are race-only and have no deterministic test: the `heartbeat_at` condition in the take-over UPDATE (DB-RULE-047) and the executor's in-memory "already executing" check. The ownership check before recording is tested.
- `go test -race` needs cgo, which this machine lacks; the race detector was not run.
