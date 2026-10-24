# 19: Hot Prospect: financing request

**What to build:** The Branch Officer records the requested facility: Requested Amount, Tenor and Margin Rate, runs the Financing Simulation, and sees the Total Obligor.

**Blocked by:** 18: Hot Prospect: business and other financing

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-018-*` acceptance criterion in SoT §13 passes
- [ ] Margin Rate is 0-100 with five decimals (BR-UI-014; agreed comment 1)
- [ ] Total Obligor is the Customer's existing financing plus the Requested Amount
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

Agreed with the user before implementation (2026-10-24):

1. Margin follows the SoT, not the earlier ticket text: five decimals (BR-UI-014, NUMERIC(8,5), AC-FR-APP-018-09).
2. Gambar 77 "Plafond Pembiayaan" vs the text "Plafon Pembiayaan": the text wins.
3. Angsuran Per Bulan not yet computed: "Rp0,00" as the text (AC-01, AC-18), not "--" of Gambar 74/75.
4. The activity "Recommend loan financing application" (DEC-142) is not recorded here: the Activity Log (ENT-011) has no §9 table and §9.8 defers its design. Moved to ticket 27; logged in the open domain questions.
5. AC-FR-APP-018-14 (Recommendation Screen in History Details, Gambar 78) stays with ticket 32; this ticket stores the recommendation (§9.12, §9.44).
6. Save (PROPOSED): no required-field check, formats and ranges still checked, stays on the page; no recommendation, activity or product limit check. Each table only when its NOT NULL inputs are complete, otherwise its earlier row stays: §9.41 with Tujuan, Pembiayaan and Jangka Waktu; §9.42 with Jumlah Pembiayaan Eksisting (Total Requested from §9.41, 0 without rows; the three notes saved with it); §9.43 with Pembiayaan, Jangka Waktu, Tipe Margin, Margin and Jumlah Pembiayaan Eksisting.
7. Back to Previous Step and the breadcrumb: validate every field, save §9.41-9.43, open the target step; no limit check, recommendation or activity. Back (bottom left) returns to the Dashboard without saving, as Close.
8. Next: field validation (BR-UI-010), then the product limit (ERR-042 below financing_limit_start or above financing_limit_end, equal accepted; ERR-043 when the limit cannot be read). Passing, in one transaction: §9.41 and §9.42 overwritten, §9.43 overwritten with the server's simulation, one recommendation in §9.12 with §9.44, the assignment completed, QCA Data created and opened (its page is ticket 20).
9. On reopening, Notes, Catatan Deviasi and Catatan Mitigasi come back from §9.42 (Process 1, TBD-055a); Total Obligor is computed on every read and on every write, the client's value ignored (DB-RULE-119).
10. ERR-042 on the banner and on Pembiayaan yang di ajukan; ERR-043 on the banner (TBD-055c, PROPOSED).
11. One calculation, on the server: POST .../facility-simulation computes the installments, the other margin and the simulation without saving; the frontend calls it (debounced) whenever a related field changes. High precision (big.Float), installments rounded to whole rupiah half up, margins to five decimals half up, RATE found numerically; matches AC-04/05 (8.884.879 and 6,61855).
12. The recommendation: type "Data Permohonan Fasilitas", decided_by the submitting AO, role snapshot of that operator, decision_code NULL, status RECOMMEND, limit SESUAILMT, the "diajukan" group from the screen, "direkomendasi" and "disetujui" 0. Nama Debitur is the applicant (the Nasabah row, or Badan Usaha).
13. Margin / Ujrah accepts digits and one comma, at most five decimals (a sixth cannot be typed); shown "12,00000" on blur with "%" inside the field; 0 to 100, 100,00001 is ERR-037.
14. The other margin is labelled "Margin Flat (%)" for Effective and "Margin Efektif (%)" for Flat (PROPOSED); hidden until a Tipe Margin is chosen (Gambar 75), then "0,00000%" until the four fields are filled.
15. Tipe Akad is a disabled dropdown with work.contract_type as it is; placeholder "0" on Jangka Waktu; Pembiayaan Yang Disetujui and the approval Jangka Waktu read-only but not grey (AC-03); Total Requested Amount "--" when empty; Total Obligor "Rp0"; Hitung Simulasi Angsuran disabled until the four fields are valid; Home and X only close the modal.
16. BMV seeded: MARGINTYPE (FLAT = Flat, EFFECTIVE = Effective), RECOMMENDED (RECOMMEND = Rekomendasi only; NOTRECOMMEND's description is not given), RECOMMEND_LIMIT (TURUNLMT, SESUAILMT, description = code, never shown). DPFHPDE not seeded (its codes are not given).
17. New tables exactly as §9: case_financing_request (§9.41), case_financing (§9.42), case_credit_simulation (§9.43), recommendation_credit_simulation (§9.44, decision_id UNIQUE FK).
18. Seams: HTTP API against real PostgreSQL (page, validation, Save, Next with limit, recommendation, simulation and QCA Data, Back to Previous Step, breadcrumb, the simulation endpoint for AC-04/05/18); UI with tools/uicheck against Gambar 74-77.

Implementation notes (2026-10-24):

- Migration 0020: case_financing_request (§9.41), case_financing (§9.42), case_credit_simulation (§9.43), recommendation_credit_simulation (§9.44) exactly as §9, no extra index; MARGINTYPE, RECOMMENDED (RECOMMEND) and RECOMMEND_LIMIT seeded.
- Endpoints on the assignment: POST facility-request with ?to=next (Next), ?to=<step name> (Back to Previous Step, breadcrumb) or ?to=save; POST facility-simulation (the four fields, nothing saved, answer ready plus both installments and both margins). SubmitFacilityRequest reuses moveHotProspect; "next" maps to QCA Data and adds the limit check and the recommendation.
- The arithmetic is cases/financing_simulation.go: 256-bit floats, PMT by exponentiation, RATE by bisection, rounding half up to rupiah and to five decimals; the other installment from the rounded other margin (Perhitungan 3). Expected values beyond AC-04/05 come from an independent Python decimal computation.
- fieldCheck gained positive (1 to max) and margin (0 to 100, five decimals, BR-UI-014). Notes count a CRLF as one character (DB-RULE-037).
- Total Obligor is computed by the server on every write from §9.41 and the existing amount; the page shows the sum of the typed values; a client total is refused (unknown field, 400).
- Frontend: FacilityRequestData.tsx (three sections, Hitung Simulasi Angsuran, the Credit Simulation modal), PercentField with five decimals, NumberField readOnly for the approval side; HotProspectCard and leave shared from HotProspectData.tsx.
- UI check tools/uicheck/facility.js (shots/facility-*.png): AC-01 (--, Rp0, Rp0,00, button disabled, Tipe Akad Qardh disabled), AC-02/03, a sixth decimal not typed, AC-04 (Rp8.884.879,00, 6,61855%), AC-06 (24 months Rp4.707.347,00), AC-05 (12,00000%), AC-10/11 modal with Home and X, AC-18, AC-12, ERR-042 "batas atas" with nothing stored, AC-13 Next to QCA Data with the recommendation.
- Mutation checks (each fails a test): either limit itself refused; no recommendation on Next; the limit on every move; installments truncated; six decimals or over 100 accepted; 0 accepted; totals not recomputed; CRLF counted twice; the flat installment from the typed margin; a simulation of invalid inputs; recommended margin NULL; the reopened margin from the wrong side; a simulation without the existing amount; the other margin not rounded.

