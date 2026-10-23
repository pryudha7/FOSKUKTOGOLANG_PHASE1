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
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
