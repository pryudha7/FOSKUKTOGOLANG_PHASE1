# 06: Assignment lock

**What to build:** Opening an assignment locks it to that operator; another operator cannot work it until the lock is released after 30 idle minutes, or an Author force-unlocks it.

**Blocked by:** 05: Create Financing Case + Case View frame

**Status:** ready-for-agent

- [ ] Every `AC-FR-CASE-011-*` acceptance criterion in SoT §13 passes
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
