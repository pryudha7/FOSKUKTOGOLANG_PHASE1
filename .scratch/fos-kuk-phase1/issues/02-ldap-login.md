# 02: LDAP login

**What to build:** An operator logs in through the LDAP path: credentials are validated against LDAP, then the operator must also be registered in the FOS operator database.

**Blocked by:** 01: Walking skeleton + Database login

**Status:** ready-for-agent

- [ ] Every `AC-FR-LOGIN-002-*` acceptance criterion in SoT §13 passes
- [ ] Login via LDAP is refused, and ERR-002 logged, when any LDAP configuration key is empty (ENV-008)
- [ ] The LDAP bind password never appears in code, repository or logs
- [ ] Tests use a fake LDAP server
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.
