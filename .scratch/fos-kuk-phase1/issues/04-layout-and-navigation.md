# 04: Layout and navigation

**What to build:** A logged-in operator sees the sidebar, their avatar with logout, and the Create menu.

**Blocked by:** 01: Walking skeleton + Database login

**Status:** ready-for-agent

- [ ] Every `AC-FR-NAV-001-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-NAV-002-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-NAV-003-*` acceptance criterion in SoT §13 passes
- [ ] UI text matches the SoT screens as written, untranslated, with no Pega branding
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

**Decisions while implementing (2026-10-23), agreed with the user (TBD-019 unless noted):**

- The sidebar expands while hovered or focused and collapses on leaving; the expanded sidebar floats over the content so the page does not shift. Long labels wrap.
- Financing Case Summary Report, Tools and Operator Tools are shown with hover styling and do nothing. Gold Monitoring is shown with no function (AC-FR-NAV-001-02; Gold stays out of scope, §2.3).
- The server decides Create: `GET <endpoint>/api/me` returns `caseTypes: ["Financing"]` only for roles AO and SAO (DEC-029), otherwise `[]`. Choosing "Financing" does nothing until ticket 05 (FR-APP-001, TBD-023).
- "FOS" stands in for the expanded logo; the displayed name is the operator's full name; the Create label uses the normal menu colour #D3D5D6; icons are simple SVGs approximating UI-008.
- "Log off" calls `POST <endpoint>/api/logout` (from ticket 03) with no confirmation and returns to the session's login page (TBD-013).

**After review (agreed):**

- Create is matched on the stable `role_code` (§9.3) AO / SAO, so renaming a role keeps the rule; the idle exemption keeps matching the role *name*, as FR-ACCESS-005 states.
- Rows use ≈ #252C32 in both states and darken to ≈ #1E2328 on hover; Create starts closed on every expansion; the operator name wraps.
- **AC-FR-NAV-003-03 moves to ticket 05**: choosing "Financing" starts the case there. Ticket 05 also settles the case type's identity (code vs label) with the Work table (§9.4); until then `caseTypes` carries the display label.

**Verification:** the Create rule is tested at the HTTP API seam. The UI was driven in headless Chrome (Playwright) for an AO and a TL operator: collapsed sidebar with Home current, expanded menus, Create submenu with only "Financing", hover, avatar popup with "Log off", logout ending the session, and no Create for TL. The same run also exercised the UI-019 idle popup from ticket 03 with a virtual clock (appears at 14:00, counts down, OK resets, expiry logs off).
