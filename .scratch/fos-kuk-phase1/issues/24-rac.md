# 24: RAC evaluation

**What to build:** After the Branch Officer Survey, the Financing Case is evaluated automatically against the RAC of its Product. A case outside the criteria is rejected at RAC.

**Blocked by:** 23: Branch Officer Survey

**Status:** ready-for-agent

- [ ] RAC criteria, variables, conditions and logic strings are read from the database (seeded for tests); no admin UI
- [ ] A failing case moves to Rejected with Case Status ResolvedRejected-RAC
- [ ] A passing case continues to Appraisal or Financing Checker
- [ ] Customer Type is evaluated by code; the INDIVIDU/CORPORATE mapping follows whatever TBD-023 resolves to and is not assumed
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
