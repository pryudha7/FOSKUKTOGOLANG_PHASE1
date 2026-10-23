# AGENTS.md

## Agent skills

### Issue tracker

Issues are tracked as local markdown files under `.scratch/<feature>/`. See `docs/agents/issue-tracker.md`.

### Triage labels

Uses the five default triage labels (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one root `GLOSSARY.md` plus `docs/adr/`. See `docs/agents/domain.md`.

## Source of truth

`docs/FOS_KUK_SoT_v1.docx` defines what to build. Cite its IDs (FR-, BR-, UI-, DB-RULE-, AC-) in tickets, tests and commits. Search it in `docs/sot/FOS_KUK_SoT_v1.txt` (extracted text; regenerate with `tools/sot/` when the document changes).

### UI

Build each screen to match its screenshot in the SoT: the _Gambar NN_ referenced by the screen's UI- entry in §10 and by its FR, viewable as `docs/ui-reference/gambar-NNN.png`. Check the result in a browser with `tools/uicheck/`. Match layout, sections, field order, labels and text exactly as shown, untranslated. When the screenshot and the written spec disagree, report the conflict and wait for a decision.

### Database schema

Create every table exactly as SoT §9 defines it: table and column names, types, nullability, keys, defaults and the DB-RULE- constraints, PROPOSED columns included. This deliberately extends SoT §1.4 rule 10: tables 9.7–9.56 are the schema too, not just 9.3–9.7. Read the table's §9 section before writing its migration. When a feature needs a table or column §9 lacks, stop and ask.
