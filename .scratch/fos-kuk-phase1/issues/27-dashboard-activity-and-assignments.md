# 27: Dashboard: Latest Activity + assignment panel

**What to build:** After login, operators see their latest activity and their worklist and work queue assignments.

**Blocked by:** 05: Create Financing Case + Case View frame

**Status:** ready-for-agent

- [ ] Every `AC-FR-DASH-001-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-DASH-002-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-DASH-003-*` acceptance criterion in SoT §13 passes
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

Note from ticket 19 (2026-10-24): Next on Data Permohonan Fasilitas must also record the activity "Recommend loan financing application" (DEC-142, AC-FR-APP-018-13). Not done there because the Activity Log (ENT-011) has no §9 table (§9.8); ask for its table first.
