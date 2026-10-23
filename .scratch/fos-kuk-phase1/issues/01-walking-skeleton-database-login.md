# 01: Walking skeleton + Database login

**What to build:** An operator registered in the FOS operator database opens the desktop endpoint, logs in through the Database path and lands on an (empty) dashboard. This slice stands up the Go backend, Next.js frontend, PostgreSQL migrations and the test harness (ephemeral database, fake external servers) that every later ticket builds on.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] Every `AC-FR-LOGIN-001-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-LOGIN-003-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-LOGIN-004-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-LOGIN-005-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-LOGIN-006-*` acceptance criterion in SoT §13 passes
- [ ] Passwords never appear in URLs, logs or responses
- [ ] Every login attempt is recorded in the login log
- [ ] Configuration is read per environment with no hard-coded defaults (ENV-001 to ENV-006)
- [ ] The test harness can start the app against an ephemeral database and register fake external servers
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
