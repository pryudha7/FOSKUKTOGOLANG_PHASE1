# 20: Hot Prospect: QCA

**What to build:** The Branch Officer answers the ten QCA questions.

**Blocked by:** 19: Hot Prospect: financing request

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-020-*` acceptance criterion in SoT §13 passes
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

Agreed with the user before implementation (2026-10-25):

1. Save (TBD-053, TBD-057b): no required check, answer codes still checked (ERR-037), stays on the page; the case's §9.47 rows are replaced with the answered questions only (answer_code is NOT NULL); DB-RULE-126 (no partial rows) holds for Next.
2. Back to Previous Step and the breadcrumb validate all ten answers, save them and open the target step (as tickets 17-19). Back (bottom left) returns to the Dashboard without saving.
3. Next: the ten answers validated, §9.47 replaced with ten rows in one transaction (DB-RULE-126), QCA Data completed, Collateral Data created and opened (its page is ticket 21).
4. A saved answer whose code is now Inactive shows its description as the chosen value (BR-UI-012, Process 1) although Inactive options are not offered otherwise; on Next the server accepts that code when it equals the saved answer of that question.
5. Errors (DEC-151): unanswered "Pertanyaan <n> wajib diisi" (ERR-010, field "Pertanyaan <n>"); a code not in BMV QCA<n> "Harap masukkan Pertanyaan <n> yang valid." (ERR-037); failed dropdowns ERR-038/ERR-039.
6. BMV QCA1 to QCA10 not seeded (the SoT gives no options); the question texts are fixed labels in code and not stored.
7. Layout as Gambar 79: card and section title "QCA Data", lavender panel, ten full-width dropdowns with "Select", buttons Back, Save, Back to Previous Step, Next.
8. New table case_qca_answer exactly as §9.47: PK (case_id, question_number), FK to work, question_number SMALLINT, answer_code VARCHAR NOT NULL.
9. Out of scope: how QCA answers feed scoring (TBD-057a).
10. Seams: HTTP API against real PostgreSQL (empty and filled page including an Inactive code, error order, ERR-037, Next with ten rows, Save, Back to Previous Step, breadcrumb); UI with tools/uicheck against Gambar 79.

Implementation notes (2026-10-25):

- Migration 0021: case_qca_answer exactly as §9.47, no seed.
- POST .../assignments/{id}/qca with {"answers": [ten codes]} and ?to=<step name> or ?to=save; exactly ten entries or 400 (BR-UI-010). SubmitQCA reuses moveHotProspect; a question's saved answer skips the BMV check (agreed comment 4), others go through fieldCheck.code against QCA<n>.
- The page carries answers (ten codes, "" unanswered), options qca1 to qca10 (pageOptions, with the saved Inactive code appended to its question's list) and facilityRequested for the breadcrumb.
- Frontend: QCAData.tsx on HotProspectCard; the question texts are a fixed list (Gambar 79).
- UI check tools/uicheck/qca.js (shots/qca-*.png): Gambar 79 with ten "Select", options by sort_order, the Collateral Data crumb open, "Pertanyaan 3 : Pertanyaan 3 wajib diisi" on the banner and under the dropdown, Save storing nine and staying, Next storing ten and opening Collateral Data, reopened with Q1A now Inactive still shown.
- Mutation checks (each fails a test): the saved Inactive answer refused or not shown; rows not replaced; unanswered stored; fewer than ten accepted; one BMV type for all questions; answers optional.

