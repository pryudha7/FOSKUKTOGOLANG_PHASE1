# Session handoff (2026-10-23)

Where the FOS KUK rebuild stands, so the next session can continue without re-deriving it.

## Done (committed on `main`)

| Ticket | What works | Commits |
|---|---|---|
| 01 | Database-path login, sessions, login_log, HTTPS via trusted proxy, test harness | c2029da, 87079e2 |
| 02 | LDAP-path login (fake directory in tests) | 4beb790, 7336d6f |
| 03 | Access master data (§9.3), desktop/mobile endpoints, session isolation, idle timeout + UI-019, logout API | 7113840, ef094d7 |
| 04 | UI-008 sidebar, Create menu (AO/SAO), avatar Log off | f26b5ba, 1afbc13 |
| 05 | Create Financing Case, Case ID / Customer Code, Case View frame (stage bar, info panel, lifecycle popup, attachments panel) | b512c0b, aa8d56d |
| 30 | Pre-Screening Submission (Perorangan): validation, customer rows, Next to Document Underlying Upload, Back | 6a7229c, 4d258a9 |
| 08 | Team Lead Pre-Screening Review: Sesuai / Tidak Sesuai, Recommendation history (§9.12), Notes banner on UI-010 | (this commit) |
| 33 | SLIK Request Data (stored on work), Create with ERR-010/020/026/029, Team Lead Pre Screening assignment | 69cefcc, (review fixes) |
| 07 | Underlying Document list (master snapshot), Upload Document modal to GCS with replacement, download with ERR-035, attachments panel, Nomor Memo | cc1f096, (review fixes) |

Each ticket file in `.scratch/fos-kuk-phase1/issues/` has a **Comments** section with the decisions agreed with the user (many are interim answers to SoT TBDs) and what was deferred.

## Next

Next recommended: **09** (Pre-Screening Check executor + Dukcapil), now unblocked by 08. Also unblocked: **31** Badan Usaha form (now includes the Badan Usaha Document Underlying Upload variant), **32** Case View mode Case + To do, **06** assignment lock, **27** Dashboard. Ticket **29** (operator/master-data management) waits on TBD-003/018. Open domain questions for the SoT owner: `.scratch/fos-kuk-glossary/issues/01-open-domain-questions.md`.

## How each ticket was worked

1. Read the ticket and the SoT sections (grep `docs/sot/FOS_KUK_SoT_v1.txt` by ID); view screenshots in `docs/ui-reference/`.
2. Ask the user about every blocking TBD or §9 gap, with a recommendation each ("yes to all" is common). Record answers in the ticket's Comments.
3. TDD at the agreed seam: HTTP API against a real PostgreSQL (testcontainers), external systems faked; red first, then a mutation check that each rule's test can fail.
4. Build the UI, run it in headless Chrome (`tools/uicheck/`), compare with the Gambar.
5. Commit, then a two-axis review (Standards and Spec sub-agents), present findings, agree fixes, commit the fixes.

## Conventions worth knowing

- Conversation with the user is in Bahasa Indonesia; code, glossary and docs are English (ADR 0001). Code identifiers use `GLOSSARY.md` terms (e.g. Financing Case, Underlying Document, Customer).
- Schema follows SoT §9 exactly, PROPOSED columns included (CLAUDE.md); additions so far were agreed: `operator`, `operator_session`, `access_groups`, `work.customer_type`.
- Every config key is required, no defaults (ENV-006). On Windows start servers from PowerShell: Git Bash rewrites `/PRWebDB1`-style values into paths.
- Smoke runs: Postgres in Docker on 55432, backend on 18080, `next start` on 13000; seed operators with `go run ./cmd/hashpw`. Uploads need `tools/uicheck/fakegcs.js` and `STORAGE_EMULATOR_HOST` (see `tools/uicheck/README.md`).
- When editing files with Python on Windows, write with `newline='
'`: text mode writes CRLF otherwise.
