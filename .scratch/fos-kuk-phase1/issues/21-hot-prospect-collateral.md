# 21: Hot Prospect: Collateral

**What to build:** The Branch Officer records the Main Collateral and any Supporting Collateral.

**Blocked by:** 20: Hot Prospect: QCA

**Status:** ready-for-agent

- [ ] Every `AC-FR-APP-021-*` acceptance criterion in SoT §13 passes
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

Agreed with the user before implementation (2026-10-25):

1. Presentase Paripasu with Status Paripasu empty is disabled as Process 5 says, not active as Gambar 88 and 90 show (the SoT: "yang berlaku aturan ini").
2. AC-FR-APP-021-18 (tab DATA KOLATERAL on Details, Gambar 92) is out of scope with the other Details tabs (UI-029), which have no ticket yet; noted in the open domain questions with the advice to make one.
3. Save (TBD-053): no required check (Tanah dan Bangunan Yang Berbentuk, Status Paripasu, Presentase Paripasu when Y); formats, ranges, BMV codes and the 10-card limit still checked; no ERR-044; the case's §9.48 rows replaced with the cards whose type is chosen; stays on the page.
4. Back to Previous Step, the breadcrumb and Next validate every card and save. ERR-044 is checked on every forward move (Next, and the breadcrumb to Document Upload) so the Agunan Utama rule cannot be skipped; not on a move back. Next opens Document Upload (its page is ticket 22). Back (bottom left) returns to the Dashboard without saving.
5. At most 10 cards per section from DSS validation/collateral/max_cards_per_section; the add button disabled at the limit; more is ERR-037 "Harap masukkan Agunan Utama yang valid." (or Jaminan Pendukung).
6. ERR-044 "Agunan Utama minimal 1 untuk total nilai pengajuan lebih dari Rp100.000.000." with the threshold from DSS validation/collateral/main_required_above_amount, on the banner and the Agunan Utama section; the Total Requested Amount from §9.42, 0 without a row. Both DSS rows seeded (LOAN_APPLICATION); a missing or invalid DSS fails Next with the general message, no default value.
7. Berlaku Sampai after today and at most 100 years ahead (operator timezone); Tahun Pembuatan 1900 to the current year + 1; money 0 to 999.999.999.999.999; text limits of Business Rule 6; changing Tipe Bukti Kepemilikan hides and clears the previous number field.
8. Card errors carry the banner prefixes "Agunan Utama <n> - " and "Jaminan Pendukung <n> - " (as "Pengurus <n> - ").
9. BMV seeded: MAINCOLLATERAL ("Tanah dan Bangunan", "Deposito Bank Nano Syariah"), COLLATERALSUPPORT ("Tanah Kosong Produktif", "Kendaraan"), description = code; OWNERSHIPTYPE (1 SHM, 2 SHGB, 3 BPKB, 4 SHMRS, 5 Akta Jaminan Fidusia); STATUS_PARIPASU (Y Ya, T Tidak). Not seeded (examples only): BENTUKTANAHBANGUNAN, COLLATERAL_STATUS, COLLATERAL_JOINT_ACCOUNT, COLLATERAL_ASURANSI.
10. Saved codes now Inactive or removed from BMV still show and are accepted (BR-UI-012), as ticket 20.
11. New table case_collateral exactly as §9.48 with UNIQUE (case_id, collateral_section, sequence_number); columns not applying to the type or hidden are NULL; pari_passu_percent 0 when not Y, NULL on Deposito (DB-RULE-127).
12. Seams: HTTP API against real PostgreSQL (empty and filled page, saving per collateral type and per Tipe Bukti Kepemilikan, paripasu, Berlaku Sampai, Tahun Pembuatan, the 10-card limit, ERR-044 at 100.000.000, empty cards dropped, Save, Back, breadcrumb); UI with tools/uicheck against Gambar 84-90.

Implementation notes (2026-10-25):

- Migration 0022: case_collateral exactly as §9.48 (UNIQUE case_id, collateral_section, sequence_number), the two DSS rows, BMV MAINCOLLATERAL, COLLATERALSUPPORT, OWNERSHIPTYPE and STATUS_PARIPASU.
- POST .../assignments/{id}/collateral with {"main": [cards], "support": [cards]} and ?to=<step name> or ?to=save. SubmitCollateral reuses moveHotProspect; cards without a type are dropped; each kept card is checked in screen order by collateralCheck.card and stored with only the fields of its type and of its Tipe Bukti Kepemilikan (the rest NULL); ERR-044 when moving after Collateral Data (stepAfter).
- One column table (collateralColumns) maps the card fields to §9.48 for reading and inserting.
- Saved codes (any column) pass the BMV check and are added to the page options when Inactive or removed, shown by description or by code.
- The page carries maxCards (DSS) and the operator's today for the date pickers.
- Frontend: CollateralData.tsx (sections, cards by type, number fields by Tipe Bukti Kepemilikan, the details card, paripasu rule); banner prefixes "Agunan Utama <n> - " and "Jaminan Pendukung <n> - " through one table in ErrorBanner.
- UI check tools/uicheck/collateral.js (shots/collateral-*.png): Gambar 84 empty page and type lists; ERR-044 on the banner and the section; Gambar 85 with Presentase Paripasu disabled; SHM then SHGB number fields; AC-03 Deposito reset; paripasu Ya, 25, Tidak (disabled, emptied); Gambar 88-90; Tahun Pembuatan typed "20a24" kept "2024"; delete; add disabled at 10 cards; Next storing two rows and opening Document Upload.
- Mutation checks (each fails a test): ERR-044 at the threshold itself, on every move, or never; eleven cards accepted; empty cards kept; the paripasu percent kept when not Y; Status Paripasu or Tanah dan Bangunan Yang Berbentuk optional; Berlaku Sampai today or past 100 years accepted; Tahun Pembuatan 1899 or after next year accepted; a hidden number or another type's fields stored; a saved Inactive code refused or not shown; dates checked against today; rows not replaced; one empty card always shown.

