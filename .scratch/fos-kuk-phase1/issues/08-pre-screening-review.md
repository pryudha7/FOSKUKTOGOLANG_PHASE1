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

## Comments

Agreed with the user before implementation (2026-10-23):

1. Sesuai stores the Recommendation, completes Pre Screening and sets case_status to "Pending-Prescreening"; creating the Pre-Screening Check run and its executor is ticket 09.
2. History Details in Case View (AC-FR-APP-010-11) moves to ticket 32. This ticket has History Pre-screening Notes on UI-017 and the Notes banner on UI-010.
3. Until worklists exist (tickets 27, 32) the Team Lead opens the assignment by its Case View URL.
4. Close on UI-017 returns to the Dashboard without saving (TBD-040a), as on UI-010.
5. Errors on UI-017 show as on UI-010 (banner and field). ERR-010 "Keputusan wajib diisi" / "Notes wajib diisi"; ERR-030 "Notes maksimal 1000 karakter" (SoT PROPOSED text).
6. Notes: whitespace only counts as empty; surrounding spaces trimmed; CRLF stored as LF; at most 1000 runes.
7. Migration seeds BMV FAC_DECISION SESUAI "Sesuai" and TIDAKSESUAI "Tidak Sesuai", sort_order 0 (the column is NOT NULL), so order falls to code.
8. `case_pre_screening_decision` is created with every §9.12 column, the Data Permohonan Fasilitas group nullable; no CHECK beyond the SoT.
9. Tanggal & Waktu Submit is formatted on the server as dd/MM/yy H:mm in the viewing operator's timezone; the accordion header is the full name of decided_by (DB-RULE-116).
10. After a passing Submit the Team Lead's browser returns to the Dashboard; Tidak Sesuai creates a new Pre-Screening Submission (status baru) for branch_officer.

Implementation notes (2026-10-23):

- Submit is `POST .../assignments/{id}/review` with `decisionCode` and `notes`; a code that is not an Active option is refused with 400.
- AC-FR-APP-009-01, -02 and -04 are tested with ticket 33 (`create_test.go`, `create_validation_test.go`); AC-FR-APP-009-03 here.
- Review fixes (agreed): glossary terms in comments; a decision code the branching does not know is refused (400); the Notes banner is the last Recommendation; a viewer without a readable timezone sees History in UTC (logged); CRLF is normalised before trimming; the banner reuses the UI-011 info banner style; tests for the locked Tipe Nasabah after return and for History after an option is retired.
- A Recommendation whose decider is not in the operator data (e.g. the case's Branch Officer missing on Sesuai) fails the Submit and records nothing.
