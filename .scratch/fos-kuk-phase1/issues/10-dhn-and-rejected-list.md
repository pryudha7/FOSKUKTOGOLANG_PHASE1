# 10: DHN + Rejected List

**What to build:** The Pre-Screening Check continues with DHN and the Rejected List. Whenever a Financing Case receives a Check Rejection, its Customer and Spouse are added to the Rejected List.

**Blocked by:** 09: Pre-Screening Check executor + Dukcapil

**Status:** ready-for-agent

- [ ] Every `AC-FR-PSAPI-006-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-PSAPI-007-*` acceptance criterion in SoT §13 passes
- [ ] DHN checks the Customer (Individual or Business Entity), never the Spouse, and currently never rejects
- [ ] The Rejected List is matched by identity number and Customer Type and is not written to the Connect API Log
- [ ] A database failure on the Rejected List check is never treated as a pass and does not change Case Status
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

Note from ticket 09 (2026-10-23): a Check Rejection must also record the customer and spouse in the Rejected List (FR-PSAPI-007); ticket 09 sets status, stage and run state only. Runs paused on DHN resume once the DHN step exists.

## Comments

Agreed with the user before implementation (2026-10-23):

1. Problem Flow (AC-FR-PSAPI-007-10/11/13) stays in ticket 14. Here a failed Rejected List query (error, lost connection, timeout) stops the run STOPPED_BROKEN at "Rejected List", logged in the application log; case_status unchanged; no automatic retry; no Connect API Log row.
2. Pre-Screening API Data (AC-FR-PSAPI-006-05, the Rejected List sub-accordion of AC-FR-PSAPI-007-01) is ticket 14.
3. Rejected List created_by/updated_by: `worked_by_user_id` of the case's last completed Pre Screening assignment (the Team Lead who submitted Sesuai or Retry); FK to operator.user_id (§9.16).
4. Recording runs in the reject's transaction, keyed (id_number, customer_type), updating an existing row back to ACTIVE (DB-RULE-050); customer_type from the case's Tipe Nasabah (BMV code = description), Spouse always "Perorangan"; reason `• Reject Pre-screening "<API>"`; a Rejected List reject writes nothing.
5. Business Entity logic is built now and tested with inserted data: identity is the NPWP of the "Badan Usaha" customer row; recording uses Nama Badan Usaha and Tanggal Akta Pendirian.
6. Seeds: `api/dhn/path` `/v1/customer/dhn/check/{IdentityNumber}` (placeholder replaced by the escaped identity), timeout_ms 30000, retry_max 3, retry_gap_ms 5000, simulate false, simulate_response (DN-00 example), `secret/dhn/apikey_ref` = DHN_APIKEY (`x-api-key` masked by code), `process/rejectedlist/query_timeout_ms` 5000. `api/dhn/baseurl` per environment.
7. DHN: DN-00 and DN-01 are Success and pass (reject criterion not active); other codes or unformatted responses Fail and retry; the log request column holds the identity number only.
8. After the Rejected List passes the run waits on SLIK (ticket 11).

Implementation notes (2026-10-23):

- Outbound calls share `internal/cases/outbound.go` (configuration, call, log fill); Dukcapil moved onto it.
- AC-FR-PSAPI-007-05 and the Business Entity half of AC-FR-PSAPI-007-09 (recording a Business Entity) cannot happen yet: nothing rejects a Business Entity until DHN's reject is enabled or Pefindo (ticket 13). The recording code handles it (NPWP, Nama Badan Usaha, Tanggal Akta Pendirian) but has no test until then.
- Ticket 09 tests now wait for Dukcapil to finish and read only Dukcapil log rows, since DHN and the Rejected List run after it.

Review fixes agreed with the user (2026-10-23):

- Any failure to save a step's result (other than closing or a lost run) stops the run STOPPED_BROKEN instead of retrying the step forever (FR-PSAPI-001 rule 9); e.g. an incomplete Spouse on a Check Rejection.
- The recorder is only `worked_by_user_id` of the last completed Pre Screening assignment, as agreed; the Team Lead always opens it before submitting (tests now do too).
- Recording takes only the customer row of the case's Tipe Nasabah plus the Spouse; reason is built with literal quotes (DB-RULE-052).
- Missing identity stays a Fail at DHN (like Dukcapil) and a broken stop at the Rejected List (no retry there, FR-PSAPI-007); explained in code.
- Shared `outboundAPI` descriptor, `send`/`failed` helpers, `caseCustomerType`, `customerIdentity` in party.go; glossary wording in comments.
- Tests: query timeout (table locked), AC-FR-PSAPI-007-09 in its stated direction, an unsavable rejection stops once.
