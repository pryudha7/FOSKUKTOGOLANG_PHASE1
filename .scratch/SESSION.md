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
| 21 | Hot Prospect Collateral Data (UI-034): §9.48, Agunan Utama and Jaminan Pendukung cards by collateral type, Tipe Bukti Kepemilikan number fields, paripasu rule, 10 cards per section (DSS), ERR-044 on forward moves (DSS threshold), Next to Document Upload | beb6da0, 872b8e6 |
| 20 | Hot Prospect QCA Data (UI-033): §9.47, ten required dropdowns from BMV QCA1-QCA10 (none seeded), ERR-010 Pertanyaan <n>, saved Inactive or removed answers still shown and accepted, Next to Collateral Data | ea92510, 9c3f558 |
| 19 | Hot Prospect Data Permohonan Fasilitas (UI-032): §9.41-9.44, server-side Financing Simulation (PMT, RATE, half-up rounding) shown live, Credit Simulation modal, product limit ERR-042/043 on Next, the Data Permohonan Fasilitas recommendation with its simulation, Next to QCA Data; steps after it reachable by breadcrumb only after that recommendation; activity log (DEC-142) moved to ticket 27 | 031edfa, fbce47a |
| 18 | Hot Prospect Data Usaha Nasabah (UI-030) and Data Pembiayaan Lain (UI-031): business address and customer_business (§9.30, written on Save only when complete), other_financing cards (§9.31) replaced per save, Isi Data dari SLIK modal from the received customer SLIK, Save, Back to Previous Step, six-step breadcrumb; Next to Data Permohonan Fasilitas | 94da3f9, b92e8bf |
| 16 | Prospect Data Entry Badan Usaha (UI-025): entity fields, code-or-description autocompletes, Go Public by description, Data Pengurus (Company Officers) on the Pre-Screening rows; Next to Financing Application; ERR-038/ERR-039 for failed dropdowns on all Prospect pages | 610e6d7, 0bec21d |
| 17 | Prospect Data Entry Perorangan (UI-026 to UI-028): Data Pribadi with customer status, region cascade, e-KTP copy; Data Pasangan (Kawin only); Data Keluarga Dekat; Create to Financing Application with Data Usaha Nasabah | 0bfd12f, ceada0f |
| 15 | Pipeline Data Entry (UI-024): assignment on run COMPLETED, Jenis Pengajuan / Produk (product_management §9.26) / Nama Bank autocomplete, Bukti Surat upload and deletion, Submit to Badan Usaha or Data Pribadi | b284728, 37e7bc9 |
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

Next recommended: **22** (Hot Prospect: Document Upload, the assignment ticket 21 opens), **31** (Pre-Screening Submission Badan Usaha; Pengurus rules already in cases/business.go), **37** (Problem Flow; TBD-043 questions first). Follow-up: BR-UI-012 for saved BMV codes no longer Active on the other dropdowns (tickets 15, 17). Also unblocked: **31** Badan Usaha form (now includes the Badan Usaha Document Underlying Upload variant), **32** Case View mode Case + To do, **06** assignment lock, **27** Dashboard. Ticket **29** (operator/master-data management) waits on TBD-003/018. Open domain questions for the SoT owner: `.scratch/fos-kuk-glossary/issues/01-open-domain-questions.md`.

## How each ticket was worked

1. Read the ticket and the SoT sections (grep `docs/sot/FOS_KUK_SoT_v1.txt` by ID); view screenshots in `docs/ui-reference/`.
2. Ask the user about every blocking TBD or §9 gap, with a recommendation each ("yes to all" is common). Record answers in the ticket's Comments.
3. TDD at the agreed seam: HTTP API against a real PostgreSQL (testcontainers), external systems faked; red first, then a mutation check that each rule's test can fail.
4. Build the UI, run it in headless Chrome (`tools/uicheck/`), compare with the Gambar.
5. Commit, then a two-axis review (Standards and Spec sub-agents), present findings, agree fixes, commit the fixes.
6. Commits: "<Title> (ticket NN)", "Fix ticket NN review findings", "Record ticket NN commits in the session handoff" (this file's table and Next); each ends with a Co-Authored-By line. Then stop the smoke environment.

## Conventions worth knowing

- Conversation with the user is in Bahasa Indonesia; code, glossary and docs are English (ADR 0001). Code identifiers use `GLOSSARY.md` terms (e.g. Financing Case, Underlying Document, Customer).
- Schema follows SoT §9 exactly, PROPOSED columns included (CLAUDE.md); additions so far were agreed: `operator`, `operator_session`, `access_groups`, `work.customer_type`.
- Every config key is required, no defaults (ENV-006). On Windows start servers from PowerShell: Git Bash rewrites `/PRWebDB1`-style values into paths.
- Smoke runs: Postgres in Docker on 55432, backend on 18080, `next start` on 13000; seed operators with `go run ./cmd/hashpw`. Uploads need `tools/uicheck/fakegcs.js` and `STORAGE_EMULATOR_HOST` (see `tools/uicheck/README.md`).
- The backend runs the Pre-Screening Check executor in-process (`app.New` returns an `*App`; call `Close`). Each outbound API (Dukcapil, DHN) needs `api/<api>/baseurl` in the database and its API key in the environment variable named by `secret/<api>/apikey_ref`.
- When editing files with Python on Windows, write with `newline="
"`: text mode writes CRLF otherwise.
- Mutation checks are small Python scripts that swap one exact string in the code, run the ticket's tests and expect a failure; write them with the Write tool (Bash heredocs mangle backslashes and quotes) and keep the patterns in step with gofmt's alignment.
- UI checks share one smoke database; a check that relies on master data another check also seeds must set it (upsert, reactivate), not assume it.
- Local demo (demo only, not part of the repo or the SoT): `C:\Users\ASUS\fos-demo\README.md` (start-demo.ps1, stop-demo.ps1, account `demo` that is its own Team Lead, simulated APIs, a SLIK RPA watcher). Its database is the container `fos-demo-pg` on the same port 55432 as the smoke database: stop the demo before a smoke run and never remove that container.

