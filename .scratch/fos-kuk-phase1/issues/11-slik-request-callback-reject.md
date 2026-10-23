# 11: SLIK request, callback and rejection

**What to build:** For an Individual, the Pre-Screening Check requests SLIK (and SLIK for the Spouse, if any) through SFS, then waits; the RPA callback delivers the result, which is stored with its PDF and evaluated for Check Rejection.

**Blocked by:** 10: DHN + Rejected List

**Status:** ready-for-agent

- [ ] Every `AC-FR-PSAPI-008-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-PSAPI-009-*` acceptance criterion in SoT §13 passes
- [ ] Every `AC-FR-PSAPI-010-*` acceptance criterion in SoT §13 passes, except -05 (View SLIK, ticket 14) and -06 (copy to Dokumen Pre-Screening, ticket 12); the PDF parts of -07 come with ticket 34
- [ ] SLIK runs only for an Individual Customer
- [ ] A SLIK request is only ever Success or Fail; rejection is decided when the result arrives
- [ ] Rejection applies on Collectibility >=2 with 1-15 Days Past Due in >=7 of 24 months, or Collectibility >=2 with >15 Days Past Due in >=2 months
- [ ] Rejection applies as soon as either the Customer's or the Spouse's result rejects
- [ ] Tests drive the real inbound callback endpoint, not the database
- [ ] Verified at the HTTP API seam against a real PostgreSQL, with external systems faked at the network boundary.

## Comments

Agreed with the user before implementation (2026-10-23):

1. Split: this ticket keeps the SLIK request (ZIP request, kode reff, SLIK then SLIK Pasangan, WAITING_SLIK), the PBSResponse service (Basic auth, Code 0/1/2, logging every /api/NBS/* hit, API_LOG_WRITE_FAILED), parsing into slik_report, slik_facility, slik_facility_history, slik_debtor, slik_collateral, slik_guarantor, SLIK Worst Collect, the SLIK Check Rejection with Rejected List, and resuming the run (Pefindo, ticket 13). The PDF report (FR-PSAPI-012) is ticket 34; the SLIKWQFlowAct assignment, SLA timeout and AC-FR-PSAPI-010-06 are ticket 12; View SLIK (AC-FR-PSAPI-010-05) is ticket 14.
2. Until ticket 34, RECEIVED is set without the PDF (pdf_document_id NULL), in the same transaction as the data (DB-RULE-058 otherwise).
3. Branch code in the kode reff (TBD-044): `work.branch_code`; `SKL253/LOS<branch>/<MM>/<yyyy>/<count(slik_report)+1>` (Asia/Jakarta); a unique clash raises the number once.
4. RPA account (TBD-044): an operator with role_code RPA, bcrypt password; the migration adds only the role.
5. `/api/NBS/*` is served by the backend at the root under the same HTTPS rule; in production the proxy routes it to the backend, not through Next.js (README).
6. Interim texts (TBD-044): Code 0 partial "SLIK result for <kode> received; waiting for the other result"; Code 1 already received "an unexpected error occurred. Reason: SLIK result for <kode> has already been received"; other Code 1 "an unexpected error occurred. Reason: <message>" without secrets.
7. Rejected List reason: `• Reject Pre-screening "SLIK" terhadap kriteria "Kolektibilitas 2 dengan maksimum tunggakan hari 15 (sebanyak 6x) ATAU Kolektabilitas 2 dengan maksimum tunggakan hari di atas 15 (sebanyak 1x)"` ("SLIKPasangan" for the Spouse).
8. `work.slik_worst_collect` is added now (planned for ticket 31, TBD-039b), filled from the Customer's result.
9. SLIK field → column mapping follows the column names and FR-PSAPI-012's field lists; it is listed below; unclear columns stay NULL and are flagged; raw data stays whole in detail_json/summary_json.
10. Settings: `api/slik/path` /v1/slik/check, timeout_ms 30000, retry_max 3, retry_gap_ms 5000, simulate false, simulate_response {"code":"1100","message":"success"}, `secret/slik/apikey_ref` SLIK_APIKEY; baseurl per environment. The log request is JSON of the eight text fields plus the ZIP reference (file name, document id), no content. Simulation still builds the ZIP.
11. ZIP request: a GCS failure or no Pre-Screening document is a Fail of the attempt (retried); empty GCS configuration is a system failure (STOPPED_BROKEN).

SLIK field to column mapping (agreed point 9). Facility: financial_institution ← ljk, financial_institution_description ← ljkKet, branch ← cabang, branch_description ← cabangKet, account_number ← noRekening, start/end_contract_number ← noAkadAwal/noAkadAkhir, credit_financing_type(_description) ← jenisKreditPembiayaan(Ket), contract_description ← akadKreditPembiayaanKet, start/end_contract_date ← tanggalAkadAwal/Akhir, start_date ← tanggalMulai, due_date ← tanggalJatuhTempo, condition_date ← tanggalKondisi, currency_code ← valutaKode, initial_credit_limit ← plafonAwal, credit_limit ← plafon, outstanding_principal ← bakiDebet, interest_rate ← sukuBungaImbalan, principal/interest_arrears ← tunggakanPokok/Bunga, penalty ← denda, quality ← kualitas, arrears_days ← jumlahHariTunggakan, arrears_frequency ← frekuensiTunggakan, condition(_description) ← kondisi(Ket), data_period ← tahun + bulan; the other columns as §9.19 names them. Debtor: address/village/subdistrict/country read alamat/kelurahan/kecamatan/negara (FR-PSAPI-012) or the English names of §9.32. Report: effective_credit_limit_other and outstanding_principal_other ← plafonEfektifLain and bakiDebetLain. To confirm with the SoT owner: the facility mappings above that §9.19 gives only as "Lembaga jasa keuangan", "Nomor rekening dan akad", "Jenis dan akad", "Nilai uang dan bunga", "Kondisi fasilitas".

Implementation notes (2026-10-23):

- slik_report.zip_document_id/pdf_document_id are UUID: they reference Case Document, whose key is a UUID (§9.7 says BIGINT/UUID); §9.18 says BIGINT.
- The request steps send the BMV codes stored on the case for tujuan_permintaan, pilihan_fasilitas, tipe_pencarian and segmentasi; whether SFS expects the codes or the descriptions is to be confirmed (asked the user).
- The ZIP request and the Rejected List record are made by the Team Lead who submitted the last Pre Screening (`lastReviewer`).
- A PBSResponse call that finds a SLIK result while the Spouse request is still running leaves the running executor to wait or go on; a Check Rejection stops either state.
- The guard that skips a SLIK request when its slik_report is no longer PENDING_REQUEST cannot be reached today (REQUESTED is written with the run's step); it stays as protection for a reopen (TBD-042).
