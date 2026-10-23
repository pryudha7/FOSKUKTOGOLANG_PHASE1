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
