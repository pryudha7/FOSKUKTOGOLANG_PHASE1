# Open domain questions for the FOS KUK SoT owner

Status: needs-info

Questions raised while building `GLOSSARY.md` from `docs/FOS_KUK_SoT_v1.docx` that the SoT does not answer. Each blocks a glossary entry; once answered, add or refine the term in `GLOSSARY.md`.

## Unexpanded acronyms

What do these stand for, and what do they mean in the KUK flow?

- [ ] FAC (stage FAC Checker; decision BMV `FAC_DECISION`; roles FACProc, FACVer, FACCustody)
- [ ] QCA (QCA Data, 10 questions in Hot Prospect)
- [ ] IDIR, RPC, WI Need (computed in the Branch Officer Survey)
- [ ] SAO (assumed Senior Account Officer)
- [ ] FA, FAGH, FADH, FADV (roles that see Submitted Loan Financing by Total Obligor band)
- [ ] POD (roles PODH, PODV)

## Undecided rules

- [ ] Does a rejection at RAC (`ResolvedRejected-RAC`) also add the Customer and Spouse to the Rejected List, or only a Check Rejection at Pre-Screening?
- [ ] Who may Reopen a rejected Financing Case, and how? (TBD-042)
- [ ] What is Exposure, and how does it differ from Total Obligor? (TBD-006)
- [ ] Does Perorangan map to RAC `INDIVIDU` and Badan Usaha to `CORPORATE`? (TBD-023)
- [ ] Which Pre-Screening Checks apply to a Business Entity beyond DHN, Rejected List and Pefindo? (TBD-042)

## Later stages

Terms for Analyst Survey, Recommendation Stage, Pre-Disbursement, FAC Checker, Disbursement, Financing Revision and Phone Verification wait for SoT v2.

## Comments

- 2026-10-23, ticket 09 — SoT conflict on Connect API Log headers: FR-PSAPI-004 rule 2 and AC-FR-PSAPI-004-01 say HTTP headers, including the API key, are not stored; §9.14 has `request_headers`/`response_headers` (CONFIRMED) and DB-RULE-066 masks secret values with "***". Decided with the user for now: follow §9.14 and DB-RULE-066 (headers stored, secrets masked). Ask the SoT owner to align FR-PSAPI-004 rule 2.
- 2026-10-24, ticket 11 — SoT conflict on document references: §9.18 types slik_report.zip_document_id and pdf_document_id as BIGINT, but they reference Case Document, whose key is a UUID (§9.7). Decided with the user: UUID. Ask the SoT owner to correct §9.18.
- 2026-10-24, ticket 13 — DB-RULE-080 has a waiting executor use the stored Pefindo token "bila obtained_at lebih baru dari saat ia mendapat code 06". Implemented with the token's nonce (unique per write): the stored token is used when it is not the one that got 06, which does not depend on clocks agreeing across instances. Decided with the user; ask the SoT owner to word DB-RULE-080 that way.
- 2026-10-24, ticket 12 — §9.15 lists the prescreening_runs states without STOPPED_SLIK_BYPASS, which FR-PSAPI-019 Process 3 proposes for a bypassed SLIK. Implemented as agreed with the user; ask the SoT owner to add it to §9.15.
- 2026-10-24, ticket 35 — §9.38 and §9.39 name no source field for pefindo_inquiry_summary.reporter_count_12_months and inquiry_count_12_months, nor for pefindo_identity_history.value and data_year_month. Stored NULL for now (agreed); ask the SoT owner for a full Get Report sample (code 01 and 36) to map them.
- 2026-10-24, ticket 14 — §9.17 types problem_flows.assignment_id as UUID, while §9.5 assignments.assignment_id is "BIGINT/UUID" and the schema uses BIGINT. Agreed: BIGINT for ticket 37; ask the SoT owner to align §9.17.

