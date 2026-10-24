# Generates migration 0014 and the Go column tables of SoT §9.35-9.39 from
# one transcription of §9, so the two cannot disagree (ticket 35).
# Usage: python tools/codegen/pefindo_details.py .   (from the repository root)
import sys

root = sys.argv[1]

# Column kinds: their SQL type and their Go colKind.
TEXT, INT, BIGINT, AMOUNT, DATE, TIMESTAMP, JSON = "text", "int", "bigint", "amount", "date", "timestamp", "json"
SQL = {TEXT: "VARCHAR", INT: "INTEGER", BIGINT: "BIGINT", AMOUNT: "NUMERIC(18,2)", DATE: "DATE", TIMESTAMP: "TIMESTAMP", JSON: "JSONB"}
GO = {TEXT: "colText", INT: "colInt", BIGINT: "colBigint", AMOUNT: "colAmount", DATE: "colDate", TIMESTAMP: "colTimestamp", JSON: "colJSON"}


def group(kind, cols, srcs):
    c, s = [x.strip() for x in cols.split(",")], [x.strip() for x in srcs.split(",")]
    assert len(c) == len(s), (cols, srcs)
    return [(a, b, kind) for a, b in zip(c, s)]


debtor = (
    group(TEXT, "npwp, email, phone, location_id, country_id, subdistrict", "npwp, email, phone, location_id, country_id, kecamatan")
    + group(TEXT, "kelurahan, occupation_id, birth_place, debtor_address, workplace, title_status_id",
            "kelurahan, occupation_id, birth_place, debtor_address, workplace, id_status_gelar")
    + group(TEXT, "identity_number, mobile_phone, gender_id, regency_city_id, economic_sector_id, debtor_group_id",
            "nomor_identitas, mobile_phone, gender_id, regency_city_id, economic_sector_id, id_golongan_debitur")
    + group(TEXT, "marital_status_id, debtor_full_name, workplace_address, name_per_identity, mother_maiden_name",
            "id_status_perkawinan, debtor_full_name, workplace_address, name_per_identity, nama_gadis_ibu_kandung")
    + group(INT, "collateral_count, guarantor_count, debtor_type_id, arrears_days, active_facility_count, closed_facility_count",
            "jml_agunan, guarantor_count, debtor_type_id, arrears_days, active_facility_count, jml_tutup_fasilitas")
    + group(INT, "multi_finance_arrears_days, multi_finance_active_facility_count, multi_finance_closed_facility_count, peer_to_peer_arrears_days, bank_arrears_days, worst_collectability",
            "mf_jml_hari_tunggakan, mf_jml_aktif_fasilitas, mf_jml_tutup_fasilitas, p2p_jml_hari_tunggakan, bank_jml_hari_tunggakan, kolektabilitas_terburuk")
    + group(INT, "non_bank_financial_arrears_days, peer_to_peer_active_facility_count, peer_to_peer_closed_facility_count, pawn_arrears_days, bank_active_facility_count, bank_closed_facility_count",
            "nbfi_jml_hari_tunggakan, p2p_jml_aktif_fasilitas, p2p_jml_tutup_fasilitas, pawn_jml_hari_tunggakan, bank_jml_aktif_fasilitas, bank_jml_tutup_fasilitas")
    + group(INT, "cooperative_arrears_days, overdue_facility_count, non_bank_financial_active_facility_count, non_bank_financial_closed_facility_count, pawn_active_facility_count, pawn_closed_facility_count",
            "coorp_jml_hari_tunggakan, jml_fasilitas_tertunggak, nbfi_jml_aktif_fasilitas, nbfi_jml_tutup_fasilitas, pawn_jml_aktif_fasilitas, pawn_jml_tutup_fasilitas")
    + group(INT, "cooperative_active_facility_count, cooperative_closed_facility_count, multi_finance_worst_collectability, worst_arrears_days, multi_finance_overdue_facility_count, peer_to_peer_worst_collectability",
            "coorp_jml_aktif_fasilitas, coorp_jml_tutup_fasilitas, mf_kolektabilitas_terburuk, jml_hari_tunggakan_terburuk, mf_jml_fasilitas_tertunggak, p2p_kolektabilitas_terburuk")
    + group(INT, "bank_worst_collectability, securities_facility_count, non_bank_financial_worst_collectability, peer_to_peer_overdue_facility_count, pawn_worst_collectability, bank_overdue_facility_count",
            "bank_kolektabilitas_terburuk, jml_fasilitas_surat_berharga, nbfi_kolektabilitas_terburuk, p2p_jml_fasilitas_tertunggak, pawn_kolektabilitas_terburuk, bank_jml_fasilitas_tertunggak")
    + group(INT, "cooperative_worst_collectability, non_bank_financial_overdue_facility_count, pawn_overdue_facility_count, cooperative_overdue_facility_count",
            "coorp_kolektabilitas_terburuk, nbfi_jml_fasilitas_tertunggak, pawn_jml_fasilitas_tertunggak, coorp_jml_fasilitas_tertunggak")
    + group(AMOUNT, "total_credit_limit, total_arrears, multi_finance_total_credit_limit, peer_to_peer_total_credit_limit, bank_total_credit_limit, non_bank_financial_total_credit_limit",
            "jml_plafon, jml_tunggakan, mf_jml_plafon, p2p_jml_plafon, bank_jml_plafon, nbfi_jml_plafon")
    + group(AMOUNT, "pawn_total_credit_limit, cooperative_total_credit_limit, total_collateral_value, multi_finance_total_arrears, peer_to_peer_total_arrears, bank_total_arrears",
            "pawn_jml_plafon, coorp_jml_plafon, jml_nilai_agunan, mf_jml_tunggakan, p2p_jml_tunggakan, bank_jml_tunggakan")
    + group(AMOUNT, "total_outstanding_balance, non_bank_financial_total_arrears, pawn_total_arrears, worst_arrears, cooperative_total_arrears, multi_finance_total_outstanding_balance",
            "jml_saldo_terutang, nbfi_jml_tunggakan, pawn_jml_tunggakan, tunggakan_terburuk, coorp_jml_tunggakan, mf_jml_saldo_terutang")
    + group(AMOUNT, "peer_to_peer_total_outstanding_balance, bank_total_outstanding_balance, non_bank_financial_total_outstanding_balance, pawn_total_outstanding_balance, cooperative_total_outstanding_balance",
            "p2p_jml_saldo_terutang, bank_jml_saldo_terutang, nbfi_jml_saldo_terutang, pawn_jml_saldo_terutang, coorp_jml_saldo_terutang")
    + group(DATE, "birth_date, last_arrears_date, last_facility_open_date, last_facility_close_date, multi_finance_last_facility_open_date, multi_finance_last_facility_close_date",
            "tanggal_lahir, tgl_tunggakan_terakhir, tgl_buka_fasilitas_terakhir, tgl_tutup_fasilitas_terakhir, mf_tgl_buka_fasilitas_terakhir, mf_tgl_tutup_fasilitas_terakhir")
    + group(DATE, "peer_to_peer_last_facility_open_date, bank_last_facility_open_date, non_bank_financial_last_facility_open_date, peer_to_peer_last_facility_close_date, pawn_last_facility_open_date, bank_last_facility_close_date",
            "p2p_tgl_buka_fasilitas_terakhir, bank_tgl_buka_fasilitas_terakhir, nbfi_tgl_buka_fasilitas_terakhir, p2p_tgl_tutup_fasilitas_terakhir, pawn_tgl_buka_fasilitas_terakhir, bank_tgl_tutup_fasilitas_terakhir")
    + group(DATE, "cooperative_last_facility_open_date, non_bank_financial_last_facility_close_date, pawn_last_facility_close_date, cooperative_last_facility_close_date",
            "coorp_tgl_buka_fasilitas_terakhir, nbfi_tgl_tutup_fasilitas_terakhir, pawn_tgl_tutup_fasilitas_terakhir, coorp_tgl_tutup_fasilitas_terakhir")
    + group(BIGINT, "golden_record_debtor_id", "id_debitur_golden_record")
)

facility = (
    group(AMOUNT, "denda, plafon, outstanding_principal, market_value, initial_credit_limit, project_value",
          "denda, plafon, baki_debet, nilai_pasar, plafon_awal, nilai_proyek")
    + group(AMOUNT, "outstanding_balance, acquisition_value, principal_arrears, nominal_arrears, interest_or_return_rate, value_in_original_currency",
            "saldo_terutang, nilai_perolehan, tunggakan_pokok, nominal_tunggakan, suku_bunga_atau_imbalan, nilai_dalam_mata_uang_asal")
    + group(AMOUNT, "interest_or_return_arrears, current_month_realization_or_disbursement",
            "tunggakan_bunga_atau_imbalan, realisasi_atau_pencairan_bulan_berjalan")
    + group(TEXT, "listing, currency_id, keterangan, bad_debt_cause_id, guarantor_address, credit_type_id",
            "listing, id_valuta, keterangan, id_sebab_macet, alamat_penjamin, id_jenis_kredit")
    + group(TEXT, "credit_nature_id, start_contract_number, branch_office_id, end_contract_number, regency_city_id, collectability_id",
            "id_sifat_kredit, nomor_akad_awal, id_kantor_cabang, nomor_akad_akhir, id_kabupaten_kota, id_kolektabilitas")
    + group(TEXT, "economic_sector_id, facility_type_id, identity_type_id, financing_scheme_id, usage_type_id, guarantor_full_name",
            "id_sektor_ekonomi, id_jenis_fasilitas, id_jenis_identitas, id_skim_pembiayaan, id_jenis_penggunaan, nama_lengkap_penjamin")
    + group(TEXT, "bad_debt_cause_remarks, restructuring_method_id, usage_orientation_id, guarantor_identity_number, facility_account_number, securities_rating",
            "keterangan_sebab_macet, id_cara_restrukturisasi, id_orientasi_penggunaan, nomor_identitas_penjamin, nomor_rekening_fasilitas, peringkat_surat_berharga")
    + group(TEXT, "government_program_credit_id, guarantor_name_per_identity, interest_or_return_rate_type_id",
            "id_kredit_program_pemerintah, nama_penjamin_sesuai_identitas, id_jenis_suku_bunga_atau_imbalan")
    + group(INT, "condition_id, total_arrears, facility_phase, syndicated_loan", "id_kondisi, jml_tunggakan, fase_fasilitas, syndicated_loan")
    + group(BIGINT, "reporter_id, reporter_type_id", "id_pelapor, id_jenis_pelapor")
    + group(INT, "worst_arrears, arrears_frequency, arrears_days, worst_collectability_source_variant, worst_collectability, worst_arrears_12_months",
            "tunggakan_terburuk, frekuensi_tunggakan, jumlah_hari_tunggakan, koletabilitas_terburuk, kolektabilitas_terburuk, tunggakan_terburuk_12bln")
    + group(INT, "restructuring_frequency, worst_arrears_days, worst_collectability_source_variant_12_months, worst_collectability_12_months, facility_extension_frequency, worst_arrears_days_12_months",
            "frekuensi_restrukturisasi, jml_hari_tunggakan_terburuk, koletabilitas_terburuk_12bln, kolektabilitas_terburuk_12bln, frekuensi_perpanjangan_fasilitas, jml_hari_tunggakan_terburuk_12bln")
    + group(DATE, "end_date, bad_debt_date, start_date, exit_date, condition_date, data_year_month",
            "tanggal_akhir, tanggal_macet, tanggal_mulai, tanggal_keluar, tanggal_kondisi, tahun_bulan_data")
    + group(DATE, "disbursement_date, start_contract_date, purchase_date, end_contract_date, issuance_date, due_date",
            "disbursement_date, tanggal_akad_awal, tanggal_pembelian, tanggal_akad_akhir, tanggal_penerbitan, tanggal_jatuh_tempo")
    + group(DATE, "breach_date, last_arrears_date, restructuring_start_date, restructuring_end_date, credit_or_financing_start_date",
            "tanggal_wanprestasi, tgl_tunggakan_terakhir, tanggal_restrukturisasi_awal, tanggal_restrukturisasi_akhir, tanggal_awal_kredit_atau_pembiayaan")
    + group(JSON, "collateral_json, guarantor_json, facility_history_json", "agunan, penjamin, riwayat_fasilitas")
)

inquiry = (
    group(TEXT, "financial_institution_name, reporter_id, financial_institution_type_description", "nama_ljk, id_pelapor, deskripsi_jenis_ljk")
    + group(TIMESTAMP, "inquiry_date", "inquiry_date")
    + group(INT, "reporter_type_id, inquiry_purpose_id", "id_jenis_pelapor, id_tujuan_permintaan")
)

# "" source: §9 names none yet, stored NULL until a Get Report sample shows
# it (ticket 35 Comment 1, open domain question).
summary = group(INT, "reporter_count_12_months, inquiry_count_12_months", ", ")
history = (
    [("value", "", TEXT)]
    + group(BIGINT, "element_id, reporter_id", "id_elemen, id_pelapor")
    + group(TEXT, "id_element", "id_element")
    + [("data_year_month", "", DATE)]
)

tables = [
    # table, key column (None: keyed by pefindo_report_id, no sequence), columns, §, comment
    ("pefindo_debtor", None, debtor, "9.35", "report.debitur, one row per report (1:1)"),
    ("pefindo_facility", "facility_id", facility, "9.36", "one row per report.fasilitas element"),
    ("pefindo_inquiry", "inquiry_id", inquiry, "9.37", "one row per report.permintaan_data element"),
    ("pefindo_inquiry_summary", "summary_id", summary, "9.38", "one row per report.summary_permintaan_data element"),
    ("pefindo_identity_history", "history_id", history, "9.39", "one row per report.riwayat_identitas_debitur element"),
]

# Migration.
sql = ['''-- Ticket 35: the normalized parts of a Pefindo report (SoT §9.35-9.39),
-- written with pefindo_report (DB-RULE-075, DB-RULE-112). Generated with the
-- Go column tables (internal/cases/pefindo_details_columns.go) by
-- tools/codegen/pefindo_details.py from one transcription of §9.
''']
for name, key, cols, sec, comment in tables:
    lines = []
    if key is None:
        lines.append("    pefindo_report_id BIGINT PRIMARY KEY REFERENCES fos.pefindo_report (pefindo_report_id)")
    else:
        lines.append(f"    {key} BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY")
        lines.append("    pefindo_report_id BIGINT NOT NULL REFERENCES fos.pefindo_report (pefindo_report_id)")
        lines.append("    sequence_number SMALLINT NOT NULL")
    for c, _, k in cols:
        lines.append(f"    {c} {SQL[k]}")
    if key is not None:
        lines.append("    UNIQUE (pefindo_report_id, sequence_number) -- DB-RULE-112")
    sql.append(f"-- §{sec}: {comment}.\nCREATE TABLE fos.{name} (\n" + ",\n".join(lines) + "\n);\n")
open(root + "/backend/internal/migrate/migrations/0014_pefindo_details.sql", "w", encoding="utf-8", newline="\n").write("\n".join(sql))

# Go column tables.
go = ['''// Code generated by tools/codegen/pefindo_details.py from SoT §9.35-9.39,
// with migration 0014. DO NOT EDIT.

package cases

// The columns of the Pefindo report detail tables and the report fields
// they come from (§9.35-9.39); "" is a column §9 names no source for yet,
// stored NULL until a Get Report sample shows it (ticket 35 Comment 1).
''']
for name, key, cols, sec, comment in tables:
    var = "pefindo" + "".join(w.capitalize() for w in name.split("_")[1:]) + "Columns"
    go.append(f"// {var} are the columns of {name} (§{sec}).")
    go.append(f"var {var} = []reportColumn{{")
    for c, s, k in cols:
        go.append(f'\t{{"{c}", "{s}", {GO[k]}}},')
    go.append("}\n")
open(root + "/backend/internal/cases/pefindo_details_columns.go", "w", encoding="utf-8", newline="\n").write("\n".join(go))
print(len(debtor), len(facility), len(inquiry), len(summary), len(history))
