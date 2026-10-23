# 08: Pre-Screening Review by Team Lead

**What to build:** After the Branch Officer creates the Pre-Screening Submission, the Team Lead receives the Pre Screening assignment and decides Sesuai or Tidak Sesuai. Tidak Sesuai returns the case to the Branch Officer with the Team Lead's notes; every decision is kept as a Recommendation in the case history.

**Blocked by:** 07: Underlying Document upload + Nomor Memo

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-009-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-APP-010-*` acceptance criterion in SoT §13 passes
- [ ] Branching uses the decision code, not its description
- [ ] After Tidak Sesuai, Customer Type and branch stay disabled while other fields are editable
- [ ] Each re-Create produces a new Pre Screening assignment for the Team Lead
- [ ] History Pre-screening Notes lists every Recommendation, oldest first
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
