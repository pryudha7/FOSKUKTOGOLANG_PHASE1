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
