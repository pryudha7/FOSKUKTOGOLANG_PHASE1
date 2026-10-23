# 29: Operator and access master data management

**What to build:** An authorised operator manages operators (UI-006: choose exactly one role, which fills access group, work group and work queue from the role's active mapping; mobile access checklist) and access master data (UI-007: roles, work groups, work queues, mappings), with no code change, rebuild or file replacement.

**Blocked by:** 03: Access control and endpoints; TBD-003 (who may manage operators and master data), TBD-018 (operator fields, UI-006/UI-007 screens), TBD-015 (manual override of filled values, deleting a role or work group in use, whether a work queue is required).

**Status:** needs-info

- [ ] Every `AC-FR-ACCESS-002-*` acceptance criterion in SoT §13 passes through the management screens
- [ ] Only operators permitted by the permission matrix (TBD-003) can manage operators and master data
- [ ] A role without an active mapping fills nothing (AC-FR-ACCESS-002-03)
- [ ] Enforce §9.3 when master data is edited: an active mapping never refers to an inactive or missing master (today the session lookup ignores such a mapping, but nothing prevents saving one)
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
