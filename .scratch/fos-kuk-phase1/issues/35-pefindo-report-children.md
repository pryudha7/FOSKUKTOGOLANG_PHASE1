# 35: Pefindo report details

**What to build:** A Get Report result also stores the normalized parts of the report: pefindo_debtor (§9.35), pefindo_facility (§9.36), pefindo_inquiry (§9.37), pefindo_inquiry_summary (§9.38) and pefindo_identity_history (§9.39), in the transaction that stores pefindo_report, and replaced with it.

**Blocked by:** 13: Pefindo

**Status:** needs-triage

- [ ] The five tables are created exactly as §9.35-9.39 define them, with DB-RULE-112's unique (pefindo_report_id, sequence_number)
- [ ] They are written with pefindo_report in one transaction before the Reject is decided (DB-RULE-075, DB-RULE-112) and deleted with the row they replace (DB-RULE-074)
- [ ] Dates and amounts follow DB-RULE-113; unknown structures are kept as JSONB (DB-RULE-114)
- [ ] Personal data in these tables is never written to the application log (DB-RULE-077)
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

Split out of ticket 13 (2026-10-24). Blocking question before work: a full Get Report sample (code 01 and 36) to check the field names and types of §9.35-9.39.
