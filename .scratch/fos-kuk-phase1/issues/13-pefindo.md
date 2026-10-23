# 13: Pefindo

**What to build:** The Pre-Screening Check queries Pefindo for the Customer and then the Spouse (Search, Generate Report, Download PDF, Get Report) and rejects on a risk grade of E1-E3.

**Blocked by:** 09: Pre-Screening Check executor + Dukcapil

**Status:** ready-for-agent

- [ ] Every `AC-FR-PSAPI-014-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-PSAPI-015-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-PSAPI-016-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-PSAPI-017-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-PSAPI-018-*` acceptance criterion in SoT §13 passes
- [ ] Pefindo runs for both Individual and Business Entity Customers
- [ ] All Customer calls finish before any Spouse call starts
- [ ] The integration token is reused until it expires
- [ ] Pefindo code 32 is retried at 60-second intervals
- [ ] The PDF is stored as a Case Document named after the Customer Code
- [ ] Stores pefindo_report, pefindo_scoring and pefindo_scoring_reason (children §9.35-9.39: ticket 35)
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

Agreed with the user before implementation (2026-10-24):

1. Position inside a Pefindo step: new PROPOSED column `prescreening_runs.current_api VARCHAR NULL` holds the api_name of the Pefindo API running (e.g. PefindoGetReport); `attempt` counts per API; a TL Retry resumes at the failed API. NULL on steps that are one API.
2. Token key (DEC-099): `secret/encryption/integration_token_key_ref` names an environment variable holding a base64 32-byte AES-256-GCM key; `key_version` stores that name, and a token stored under another version counts as absent (a new one is requested). getToken's whole Authorization value comes from the variable named by `secret/pefindotoken/authorization_ref`; x-gateway-apikey (`secret/pefindo/apikey_ref`) is sent on Search only.
3. Get Report and Download PDF wait their single `retry_gap_ms` (60000) after every Fail, not only code 32; a code 06 is retried at once.
4. Simulation: `simulate` false for the five Pefindo APIs; `simulate_response` from the SoT samples: Search code 31, Generate Report 01, Download PDF JSON 31, Get Report 31. A simulated API requests no token.
5. Business Entity: Search uses the 'Badan Usaha' customer row (npwp, full_name as company name, no date_of_birth). Pefindo Pasangan runs only when a 'Pasangan' customer row exists (none for a Business Entity; TBD-042c). An empty customer_code or identity is a Fail for missing request data (as ticket 09).
6. The PDF is read whole in memory with no application limit (SoT: no size limit); JSON responses keep the 10 MB bound. The log response is "[application/pdf, <n> bytes]".
7. Report dates (header_inquiry_date, inquiry_date): RFC3339 with offset, or yyyy-MM-dd HH:mm:ss read as Asia/Jakarta, stored UTC. Numbers may be strings or numbers. An unreadable value is stored NULL (it stays in report_json) and does not fail Get Report.
8. Split: this ticket stores pefindo_report, pefindo_scoring and pefindo_scoring_reason; the normalized children §9.35-9.39 move to ticket 35. The UI parts of AC-FR-PSAPI-014-06, AC-FR-PSAPI-017-08 (banner) and AC-FR-PSAPI-018-06 (Download PDF button) move to ticket 14.
9. The PDF's created_by and the Rejected List record are the Team Lead who submitted the last Pre-Screening Review (`lastReviewer`, as ticket 11). Reason: Reject Pre-screening "Pefindo" (or "PefindoPasangan") terhadap kriteria "Risk Grade di bawah D3".

Implementation notes (2026-10-24):

- DB-RULE-080 merge: a waiting executor uses the stored token when it is not the one that got code 06, told apart by its nonce (unique per write) rather than by obtained_at, so equal timestamps cannot hide or fake a renewal. The getToken call is made while holding the integration_token row lock.
- With no stored token and a failed getToken, the attempt fails without calling the API and is retried with the normal gap. A code 06 renews the token even on the last attempt.
- The PDF is stored (its own transaction, slot replaced per DB-RULE-031) before the Download PDF attempt is recorded; a repeated attempt replaces it. A GCS failure fails the attempt; GCS not configured stops the run broken (as for SLIK).
- A Search code 01 without a data element carrying id_pefindo, or without inquiry_id, is a Fail (not the documented format). A Get Report scoring element without a yyyy-MM-dd period is a Fail (pefindo_scoring.period is NOT NULL).
- getToken has no simulate or retry settings (it is never simulated on its own and never repeated). The Download PDF and Get Report paths carry an {EventID} placeholder. api/pefindo/baseurl and product_id are not seeded (per environment, DB-RULE-085).
- A Pefindo reject leaves current_step "Pefindo" (or "Pefindo Pasangan") with current_api PefindoGetReport; the Rejected List reason names the step's API ("Pefindo" / "PefindoPasangan").
- New env: PEFINDO_APIKEY, PEFINDO_TOKEN_AUTHORIZATION, INTEGRATION_TOKEN_KEY (backend/.env.example).
- Mutation checks (each fails a test): no token merge; last array element instead of latest period; no immediate retry on 06; report save failure not turned into Fail; current_api ignored on resume; first Search hit instead of highest similarity; 9s codes not passing; PDF not recognised; Business Entity sent as PERSONAL; token not masked in the log; only E1 rejecting; no criterion in the reason.
- No UI check: the Pefindo screens are ticket 14.

Review fixes (2026-10-24, agreed with the user):

- The wait after Generate Report is taken before the first Download PDF attempt, so a run resumed by another executor waits it too.
- An unknown current_api (e.g. a renamed API) stops the run broken instead of restarting the chain at Search.
- Integers in the report are parsed as integers (a whole number with a fraction is accepted); one outside INTEGER is stored NULL. similarity_score may be a number or a string.
- A code 01/36 answer with scoring but no report object is judged on its scoring and stores no pefindo_report row ("response-nya berisi report").
- An empty name or birth date (Individual) is a Fail for missing request data too: Search needs every field of its body (extends point 5).
- DB-RULE-080 is implemented with the token's nonce instead of obtained_at (same intent, clock independent); asked of the SoT owner in the open domain questions.
- Kept as is: the token row lock is held during getToken (DB-RULE-080); any failure to save a Get Report result is a Fail (DB-RULE-075); a missing Customer row is a Fail for missing request data (as ticket 09).
- A Pefindo reject of a Business Entity records it on the Rejected List with its establishment_date (FR-PSAPI-007); test fixtures now carry it.
- Tests added: the Download PDF wait and Get Report's own retry gap (fake call times); report shapes (scoring without report, string similarity, out-of-range score); unknown API. Test log helpers order by writing transaction (xmin) then ctid: ctid alone is not insertion order and made the ordering flaky.

