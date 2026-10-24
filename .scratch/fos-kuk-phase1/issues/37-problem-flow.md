# 37: Problem Flow Assignment (Resume Broken Process)

**What to build:** A system failure in an automatic step that has no retry of its own (today the Rejected List query, and a run stopped STOPPED_BROKEN) records a problem flow and creates a "Problem Flow Assignment" in the Broken Process work queue; Authors open it (UI-022) and Submit runs the failed step again once.

**Blocked by:** 09: Pre-Screening Check executor + Dukcapil

**Status:** needs-triage

- [ ] Every `AC-FR-FLOW-001-*` acceptance criterion in SoT §13 passes
- [ ] problem_flows created exactly as §9.17 defines it, except assignment_id BIGINT to match assignments (agreed for ticket 14; open domain question)
- [ ] The Broken Process work queue is master data; only Authors reach it by default (FR-FLOW-001 rule 2)
- [ ] UI-022 matches Gambar 42; Submit repeats the failed step once (FR-FLOW-001 Process 3)
- [ ] case_status stays Pending-Prescreening while broken (rule 4)
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

Split out of ticket 14 (2026-10-24). The To do item (Gambar 41) and the work queue list depend on tickets 32 and 27. Blocking questions before work: TBD-043 (Return, Cancel, Admin access).
