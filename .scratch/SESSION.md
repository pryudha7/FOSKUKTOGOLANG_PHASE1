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
| 14 | Pre-Screening API Data (UI-021) and API Log History (UI-020) in History Details; Problem Flow split to ticket 37 | 70d82e3, 06c7a80 |
| 36 | Waiting SLIK assignment (SLIKWQFlowAct, UI-023) and SLIKInquiry status check | 029ca8e, c4ddf15 |
| 35 | Pefindo report details (§9.35-9.39), generated from one transcription of §9 by tools/codegen | 0ef8195, 4bd0c3d |
| 34 | SLIK report PDF (fpdf, Helvetica), stored with the result; copy into Dokumen Pre-Screening on opening Document Underlying Upload | 8be9b65, e638e11 |
| 12 | SLIK SLA timeout, bypass, Manual SLIK Entry (System Input Manual, ERR-036, UI-017 Timeout SLIK, Sesuai resumes at Pefindo); SLIKWQFlowAct/UI-023/SLIKInquiry split to ticket 36 | fdd5051, 4171104 |
| 13 | Pefindo Search, Generate Report, Download PDF, Get Report; encrypted shared token; prescreening_runs.current_api; report children split to ticket 35 | 1412cf8, ad28c09 |
| 11 | SLIK request (ZIP, kode reff), PBSResponse service at /api/NBS/, SLIK parsing into six tables, SLIK Check Rejection; PDF split to ticket 34 | dd1b15a, a46e75f |
| 10 | DHN check, Rejected List check and recording (§9.16); shared outbound API helper | 0fcc14a, 7998b64 |
| 09 | Pre-Screening Check run (§9.15) with executor, retry, recovery sweep; Dukcapil; Connect API Log (§9.14); Retry view | fd597c1, 2cd49c6 |
| 08 | Team Lead Pre-Screening Review: Sesuai / Tidak Sesuai, Recommendation history (§9.12), Notes banner on UI-010 | 465d390, 73dfabb |
| 33 | SLIK Request Data (stored on work), Create with ERR-010/020/026/029, Team Lead Pre Screening assignment | 69cefcc, 8b6c891 |
| 07 | Underlying Document list (master snapshot), Upload Document modal to GCS with replacement, download with ERR-035, attachments panel, Nomor Memo | cc1f096, 54f7594 |

Each ticket file in `.scratch/fos-kuk-phase1/issues/` has a **Comments** section with the decisions agreed with the user (many are interim answers to SoT TBDs) and what was deferred.

## Next

Next recommended: **15** (Pipeline Data Entry, after a run completes), **37** (Problem Flow; TBD-043 questions first). Also unblocked: **31** Badan Usaha form (now includes the Badan Usaha Document Underlying Upload variant), **32** Case View mode Case + To do, **06** assignment lock, **27** Dashboard. Ticket **29** (operator/master-data management) waits on TBD-003/018. Open domain questions for the SoT owner: `.scratch/fos-kuk-glossary/issues/01-open-domain-questions.md`.

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
- The backend runs the Pre-Screening Check executor in-process (`app.New` returns an `*App`; call `Close`). Each outbound API (Dukcapil, DHN) needs `api/<api>/baseurl` in the database and its API key in the environment variable named by `secret/<api>/apikey_ref`.
- When editing files with Python on Windows, write with `newline='
'`: text mode writes CRLF otherwise.
