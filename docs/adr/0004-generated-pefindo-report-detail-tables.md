# Pefindo report detail tables are generated from one transcription of §9

Migration 0014 and `internal/cases/pefindo_details_columns.go` are not hand-written: `tools/codegen/pefindo_details.py` generates both from a single transcription of SoT §9.35-9.39 (103 debtor and 79 facility columns). We chose generation because with that many columns a hand-written table and a hand-written JSON-to-column mapping would drift apart unnoticed. To change these tables, change the transcription and rerun the generator; editing the generated files by hand is overwritten on the next run.
