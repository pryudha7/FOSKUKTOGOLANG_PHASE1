package cases

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// A SLIK result file (FR-PSAPI-010) is parsed into slik_report (§9.18),
// slik_facility (§9.19) with slik_facility_history (§9.20), slik_debtor
// (§9.32), slik_collateral (§9.33) and slik_guarantor (§9.34). Every value in
// the file is a string; an empty one is NULL (DB-RULE-109). The original
// elements stay whole in summary_json and detail_json (DEC-138).

// slikFile is the chosen file of the ZIP.
type slikFile struct {
	name string
	doc  map[string]any
}

// pickSLIKFile chooses KTP_<number>.txt from the ZIP: the plainest name, else
// the smallest numeric suffix, else the first (FR-PSAPI-010 Pemilihan berkas).
// Recap_*.txt is never used.
func pickSLIKFile(data []byte) (slikFile, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return slikFile{}, errors.New("the file is not a ZIP")
	}
	type candidate struct {
		f      *zip.File
		plain  bool
		suffix int
		order  int
	}
	var cands []candidate
	for i, f := range zr.File {
		base := path.Base(f.Name)
		if !strings.HasPrefix(base, "KTP_") || !strings.HasSuffix(strings.ToLower(base), ".txt") {
			continue
		}
		stem := strings.TrimSuffix(strings.TrimPrefix(base, "KTP_"), path.Ext(base))
		c := candidate{f: f, order: i, suffix: 1 << 30}
		if _, err := strconv.ParseUint(stem, 10, 64); err == nil {
			c.plain = true
		} else if k := strings.LastIndexAny(stem, "_-( "); k >= 0 {
			if n, err := strconv.Atoi(strings.Trim(stem[k+1:], ")")); err == nil {
				c.suffix = n
			}
		}
		cands = append(cands, c)
	}
	if len(cands) == 0 {
		return slikFile{}, errors.New("the ZIP has no KTP_<nomor KTP>.txt file")
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].plain != cands[j].plain {
			return cands[i].plain
		}
		if cands[i].suffix != cands[j].suffix {
			return cands[i].suffix < cands[j].suffix
		}
		return cands[i].order < cands[j].order
	})
	rc, err := cands[0].f.Open()
	if err != nil {
		return slikFile{}, err
	}
	defer rc.Close()
	raw, err := io.ReadAll(io.LimitReader(rc, maxResponseBytes*5))
	if err != nil {
		return slikFile{}, err
	}
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")) // BOM tolerated
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return slikFile{}, errors.New("the SLIK file is not valid JSON")
	}
	return slikFile{name: path.Base(cands[0].f.Name), doc: doc}, nil
}

// obj and arr read nested JSON values.
func obj(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	if v == nil {
		return map[string]any{}
	}
	return v
}

func arr(m map[string]any, key string) []map[string]any {
	items, _ := m[key].([]any)
	var out []map[string]any
	for _, it := range items {
		if o, ok := it.(map[string]any); ok {
			out = append(out, o)
		}
	}
	return out
}

// field is a scalar value of the file as text, or nil when empty.
func field(m map[string]any, keys ...string) *string {
	for _, k := range keys {
		switch v := m[k].(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				return &s
			}
		case float64:
			s := strconv.FormatFloat(v, 'f', -1, 64)
			return &s
		}
	}
	return nil
}

// fieldDate reads yyyyMMdd (or a longer yyyyMMdd... stamp) as a date.
func fieldDate(m map[string]any, key string) *time.Time {
	s := field(m, key)
	if s == nil || len(*s) < 8 {
		return nil
	}
	t, err := time.Parse("20060102", (*s)[:8])
	if err != nil {
		return nil
	}
	return &t
}

// fieldInt reads an integer, or nil.
func fieldInt(m map[string]any, key string) *int64 {
	s := field(m, key)
	if s == nil {
		return nil
	}
	n, err := strconv.ParseInt(*s, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

// fieldNum reads a number as text for a NUMERIC column, or nil.
func fieldNum(m map[string]any, key string) *string {
	s := field(m, key)
	if s == nil {
		return nil
	}
	if _, err := strconv.ParseFloat(*s, 64); err != nil {
		return nil
	}
	return s
}

// period reads yyyyMM.
func period(s *string) *string {
	if s == nil || len(*s) < 6 {
		return nil
	}
	p := (*s)[:6]
	return &p
}

// column is one column value of an insert.
type column struct {
	name  string
	value any
}

// insertRow inserts columns into table and returns the new key column.
func insertRow(ctx context.Context, tx pgx.Tx, table, key string, cols []column) (int64, error) {
	names := make([]string, len(cols))
	marks := make([]string, len(cols))
	args := make([]any, len(cols))
	for i, c := range cols {
		names[i] = c.name
		marks[i] = fmt.Sprintf("$%d", i+1)
		args[i] = c.value
	}
	sql := "INSERT INTO fos." + table + " (" + strings.Join(names, ", ") + ") VALUES (" + strings.Join(marks, ", ") + ")"
	if key == "" {
		_, err := tx.Exec(ctx, sql, args...)
		return 0, err
	}
	var id int64
	err := tx.QueryRow(ctx, sql+" RETURNING "+key, args...).Scan(&id)
	return id, err
}

// numCol is a NUMERIC column fed as text.
func numCol(name string, v *string) column {
	if v == nil {
		return column{name, nil}
	}
	return column{name, *v}
}

// storeSLIK writes the parsed file for one slik_report row and returns the
// worst Collectibility of its facilities (0 without facilities; FR-PSAPI-010
// rule 6). Columns without a clear source field stay NULL (agreed).
func storeSLIK(ctx context.Context, tx pgx.Tx, reportID int64, f slikFile, now time.Time) (int, error) {
	header := obj(f.doc, "header")
	individual := obj(f.doc, "individual")
	search := obj(individual, "parameterPencarian")
	summary := obj(individual, "ringkasanFasilitas")
	facilities := obj(individual, "fasilitas")

	summaryJSON, _ := json.Marshal(map[string]any{
		"header": f.doc["header"], "parameterPencarian": individual["parameterPencarian"],
		"dataPokokDebitur": individual["dataPokokDebitur"], "ringkasanFasilitas": individual["ringkasanFasilitas"],
		"nomorLaporan": individual["nomorLaporan"], "posisiDataTerakhir": individual["posisiDataTerakhir"],
		"tanggalPermintaan": individual["tanggalPermintaan"],
	})
	rawJSON := func(key string) []byte {
		v, ok := facilities[key]
		if !ok {
			return nil
		}
		b, _ := json.Marshal(v)
		return b
	}

	worst := 0
	creditFacilities := arr(facilities, "kreditPembiayan")
	for i, fac := range creditFacilities {
		if q := fieldInt(fac, "kualitas"); q != nil && int(*q) > worst {
			worst = int(*q)
		}
		detail, _ := json.Marshal(fac)
		data := field(fac, "tahun")
		if month := field(fac, "bulan"); data != nil && month != nil {
			p := *data + fmt.Sprintf("%02s", *month)
			data = &p
		} else {
			data = nil
		}
		facilityID, err := insertRow(ctx, tx, "slik_facility", "facility_id", []column{
			{"slik_report_id", reportID}, {"sequence_number", i + 1},
			{"financial_institution", field(fac, "ljk")}, {"financial_institution_description", field(fac, "ljkKet")},
			{"branch_description", field(fac, "cabangKet")},
			{"account_number", field(fac, "noRekening")}, {"start_contract_number", field(fac, "noAkadAwal")},
			{"end_contract_number", field(fac, "noAkadAkhir")},
			{"credit_financing_type", field(fac, "jenisKreditPembiayaan")}, {"credit_financing_type_description", field(fac, "jenisKreditPembiayaanKet")},
			{"contract_description", field(fac, "akadKreditPembiayaanKet")},
			{"start_contract_date", fieldDate(fac, "tanggalAkadAwal")}, {"end_contract_date", fieldDate(fac, "tanggalAkadAkhir")},
			{"start_date", fieldDate(fac, "tanggalMulai")}, {"due_date", fieldDate(fac, "tanggalJatuhTempo")},
			{"condition_date", fieldDate(fac, "tanggalKondisi")},
			{"currency_code", field(fac, "valutaKode")},
			numCol("initial_credit_limit", fieldNum(fac, "plafonAwal")), numCol("credit_limit", fieldNum(fac, "plafon")),
			numCol("outstanding_principal", fieldNum(fac, "bakiDebet")), numCol("interest_rate", fieldNum(fac, "sukuBungaImbalan")),
			numCol("principal_arrears", fieldNum(fac, "tunggakanPokok")), numCol("interest_arrears", fieldNum(fac, "tunggakanBunga")),
			numCol("penalty", fieldNum(fac, "denda")),
			{"quality", fieldInt(fac, "kualitas")},
			{"arrears_days", fieldInt(fac, "jumlahHariTunggakan")}, {"arrears_frequency", fieldInt(fac, "frekuensiTunggakan")},
			{"condition", field(fac, "kondisi")}, {"condition_description", field(fac, "kondisiKet")},
			{"branch", field(fac, "cabang")}, {"credit_financing_nature", field(fac, "sifatKreditPembiayaan")},
			{"credit_financing_contract", field(fac, "akadKreditPembiayaan")}, {"debtor_category_code", field(fac, "kategoriDebiturKode")},
			{"usage_type", field(fac, "jenisPenggunaan")}, {"economic_sector", field(fac, "sektorEkonomi")},
			{"government_program_credit", field(fac, "kreditProgramPemerintah")}, {"project_location", field(fac, "lokasiProyek")},
			{"interest_return_rate_type", field(fac, "jenisSukuBungaImbalan")}, {"bad_debt_cause_code", field(fac, "kodeSebabMacet")},
			{"restructuring_method_code", field(fac, "kodeCaraRestrukturisasi")},
			{"credit_financing_nature_description", field(fac, "sifatKreditPembiayaanKet")},
			{"debtor_category_description", field(fac, "kategoriDebiturKet")}, {"usage_type_description", field(fac, "jenisPenggunaanKet")},
			{"economic_sector_description", field(fac, "sektorEkonomiKet")},
			{"government_program_credit_description", field(fac, "kreditProgramPemerintahKet")},
			{"project_location_description", field(fac, "lokasiProyekKet")},
			{"interest_return_rate_type_description", field(fac, "jenisSukuBungaImbalanKet")},
			{"restructuring_description", field(fac, "restrukturisasiKet")}, {"bad_debt_cause_description", field(fac, "sebabMacetKet")},
			{"quality_description", field(fac, "kualitasKet")}, {"keterangan", field(fac, "keterangan")},
			{"extension_frequency", fieldInt(fac, "frekPerpjganKreditPembiayaan")}, {"restructuring_frequency", fieldInt(fac, "frekuensiRestrukturisasi")},
			numCol("project_value", fieldNum(fac, "nilaiProyek")), numCol("current_month_realization", fieldNum(fac, "realisasiBulanBerjalan")),
			numCol("value_in_original_currency", fieldNum(fac, "nilaiDalamMataUangAsal")),
			{"credit_start_date", fieldDate(fac, "tanggalAwalKredit")}, {"bad_debt_date", fieldDate(fac, "tanggalMacet")},
			{"restructuring_end_date", fieldDate(fac, "tanggalRestrukturisasiAkhir")}, {"formed_date", fieldDate(fac, "tanggalDibentuk")},
			{"updated_date", fieldDate(fac, "tanggalUpdate")},
			{"data_period", data}, {"detail_json", detail},
		})
		if err != nil {
			return 0, err
		}
		// DB-RULE-062: always 24 months, empty ones NULL.
		for m := 1; m <= 24; m++ {
			nn := fmt.Sprintf("tahunBulan%02d", m)
			if _, err := insertRow(ctx, tx, "slik_facility_history", "", []column{
				{"facility_id", facilityID}, {"month_index", m}, {"period", period(field(fac, nn))},
				{"kolektabilitas", fieldInt(fac, nn+"Kol")}, {"arrears_days", fieldInt(fac, nn+"Ht")},
			}); err != nil {
				return 0, err
			}
		}
		for j, ag := range arr(fac, "agunan") {
			if _, err := insertRow(ctx, tx, "slik_collateral", "", []column{
				{"facility_id", facilityID}, {"sequence_number", j + 1},
				{"collateral_type_description", field(ag, "jenisAgunanKet")}, {"collateral_number", field(ag, "nomorAgunan")},
				{"binding_type", field(ag, "jenisPengikatan")}, {"binding_type_description", field(ag, "jenisPengikatanKet")},
				numCol("collateral_value_per_financial_institution", fieldNum(ag, "nilaiAgunanMenurutLJK")),
				numCol("collateral_njop_value", fieldNum(ag, "nilaiAgunanNjop")), numCol("independent_collateral_value", fieldNum(ag, "nilaiAgunanIndep")),
				numCol("pari_passu_percentage", fieldNum(ag, "prosentaseParipasu")),
				{"updated_date", fieldDate(ag, "tanggalUpdate")}, {"binding_date", fieldDate(ag, "tanggalPengikatan")},
				{"reporter_appraisal_date", fieldDate(ag, "tglPenilaianPelapor")},
				{"independent_appraiser_appraisal_date", fieldDate(ag, "tanggalPenilaianPenilaiIndependen")},
				{"collateral_owner_name", field(ag, "namaPemilikAgunan")}, {"collateral_address", field(ag, "alamatAgunan")},
				{"collateral_location_regency_city", field(ag, "kabKotaLokasiAgunan")},
				{"collateral_location_regency_city_description", field(ag, "kabKotaLokasiAgunanKet")},
				{"collateral_rating", field(ag, "peringkatAgunan")}, {"rating_agency_code", field(ag, "kodeLembagaPemeringkat")},
				{"rating_agency", field(ag, "lembagaPemeringkat")}, {"ownership_proof", field(ag, "buktiKepemilikan")},
				{"independent_appraiser_name", field(ag, "namaPenilaiIndep")}, {"asuransi", field(ag, "asuransi")},
				{"keterangan", field(ag, "keterangan")},
			}); err != nil {
				return 0, err
			}
		}
		for j, pj := range arr(fac, "penjamin") {
			if _, err := insertRow(ctx, tx, "slik_guarantor", "", []column{
				{"facility_id", facilityID}, {"sequence_number", j + 1},
				{"guarantor_name", field(pj, "namaPenjamin")}, {"identity_number", field(pj, "nomorIdentitas")},
				{"guarantor_type_code", field(pj, "kodeJenisPenjamin")}, {"guarantor_type_description", field(pj, "keteranganJenisPenjamin")},
				{"guarantor_address", field(pj, "alamatPenjamin")}, {"keterangan", field(pj, "keterangan")},
				{"created_date", fieldDate(pj, "tanggalBuat")}, {"updated_date", fieldDate(pj, "tanggalUpdate")},
			}); err != nil {
				return 0, err
			}
		}
	}

	for i, d := range arr(individual, "dataPokokDebitur") {
		// §9.32 names English source fields where FR-PSAPI-012 names Indonesian
		// ones; either is read.
		if _, err := insertRow(ctx, tx, "slik_debtor", "", []column{
			{"slik_report_id", reportID}, {"sequence_number", i + 1},
			{"debtor_name", field(d, "namaDebitur")}, {"identity", field(d, "identity", "identitas")},
			{"identity_number", field(d, "noIdentitas")}, {"npwp", field(d, "npwp")},
			{"gender", field(d, "jenisKelamin")}, {"gender_description", field(d, "jenisKelaminKet")}, {"birth_place", field(d, "tempatLahir")},
			{"birth_date", fieldDate(d, "tanggalLahir")}, {"formed_date", fieldDate(d, "tanggalDibentuk")}, {"updated_date", fieldDate(d, "tanggalUpdate")},
			{"pelapor", field(d, "pelapor")}, {"reporter_description", field(d, "pelaporKet")},
			{"address", field(d, "alamat", "address")}, {"village", field(d, "kelurahan", "village")},
			{"subdistrict", field(d, "kecamatan", "subdistrict")}, {"regency_city", field(d, "kabKota")},
			{"regency_city_description", field(d, "kabKotaKet")}, {"postal_code", field(d, "kodePos")},
			{"country", field(d, "negara", "country")}, {"country_description", field(d, "negaraKet")},
			{"pekerjaan", field(d, "pekerjaan")}, {"occupation_description", field(d, "pekerjaanKet")},
			{"workplace", field(d, "tempatBekerja")}, {"business_field", field(d, "bidangUsaha")},
			{"business_field_description", field(d, "bidangUsahaKet")},
			{"debtor_title_code", field(d, "kodeGelarDebitur")}, {"debtor_title_status", field(d, "statusGelarDebitur")},
		}); err != nil {
			return 0, err
		}
	}

	_, err := tx.Exec(ctx, `
		UPDATE fos.slik_report SET
			source_file_name = $2, inquiry_id = $3, report_number = $4, latest_data_position = $5, result_date = $6,
			worst_quality = $7, worst_quality_data_month = $8, worst_collect = $9,
			total_effective_credit_limit = $10::numeric, total_outstanding_principal = $11::numeric,
			inquiry_date = $12, header_inquiry_date = $13, search_parameter_birth_date = $14,
			search_parameter_debtor_name = $15, search_parameter_gender = $16, search_parameter_gender_description = $17,
			search_parameter_npwp = $18, search_parameter_birth_place = $19,
			effective_credit_limit_credit_financing = $20::numeric, effective_credit_limit_letter_of_credit = $21::numeric,
			effective_credit_limit_guarantee_given = $22::numeric, effective_credit_limit_securities = $23::numeric,
			effective_credit_limit_other = $24::numeric,
			outstanding_principal_credit_financing = $25::numeric, outstanding_principal_letter_of_credit = $26::numeric,
			outstanding_principal_guarantee_given = $27::numeric, outstanding_principal_securities = $28::numeric,
			outstanding_principal_other = $29::numeric,
			creditor_public_bank = $30, creditor_rural_bank = $31, creditor_financing_institution = $32, creditor_others = $33,
			inquiry_usage_id = $34, header_created_by = $35, inquiry_financial_institution_code = $36, inquiry_branch_code = $37,
			inquiry_purpose_code = $38, total_parts = $39, part_number = $40,
			summary_json = $41, letter_of_credit_json = $42, guarantee_given_json = $43, other_facility_json = $44,
			updated_at = $45
		WHERE slik_report_id = $1`,
		reportID, f.name, field(header, "idPermintaan"), field(individual, "nomorLaporan"), field(individual, "posisiDataTerakhir"),
		field(header, "tanggalHasil"),
		fieldInt(summary, "kualitasTerburuk"), period(field(summary, "kualitasBulanDataTerburuk")), worst,
		fieldNum(summary, "plafonEfektifTotal"), fieldNum(summary, "bakiDebetTotal"),
		fieldDate(individual, "tanggalPermintaan"), fieldDate(header, "tanggalPermintaan"), fieldDate(search, "tanggalLahir"),
		field(search, "namaDebitur"), field(search, "jenisKelamin"), field(search, "jenisKelaminKet"),
		field(search, "npwp"), field(search, "tempatLahir"),
		fieldNum(summary, "plafonEfektifKreditPembiayaan"), fieldNum(summary, "plafonEfektifLc"), fieldNum(summary, "plafonEfektifGyd"),
		fieldNum(summary, "plafonEfektifSec"), fieldNum(summary, "plafonEfektifLain"),
		fieldNum(summary, "bakiDebetKreditPembiayaan"), fieldNum(summary, "bakiDebetLc"), fieldNum(summary, "bakiDebetGyd"),
		fieldNum(summary, "bakiDebetSec"), fieldNum(summary, "bakiDebetLain"),
		fieldInt(summary, "krediturBankUmum"), fieldInt(summary, "krediturBPR/S"), fieldInt(summary, "krediturLp"), fieldInt(summary, "krediturLainnya"),
		field(header, "idPenggunaPermintaan"), field(header, "dibuatOleh"), field(header, "kodeLJKPermintaan"),
		field(header, "kodeCabangPermintaan"), field(header, "kodeTujuanPermintaan"),
		fieldInt(header, "totalBagian"), fieldInt(header, "nomorBagian"),
		summaryJSON, rawJSON("lc"), rawJSON("garansiYgDiberikan"), rawJSON("fasilitasLain"), now)
	return worst, err
}

// slikRejects is the SLIK Check Rejection (FR-PSAPI-010 rule 1, DB-RULE-063):
// a facility with Collectibility >= 2 and 1-15 Days Past Due in 7 or more of
// its 24 months, or Collectibility >= 2 and more than 15 days in 2 or more.
func slikRejects(ctx context.Context, tx pgx.Tx, reportID int64) (bool, error) {
	var rejects bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM fos.slik_facility_history h
			JOIN fos.slik_facility f ON f.facility_id = h.facility_id
			WHERE f.slik_report_id = $1 AND h.month_index BETWEEN 1 AND 24
			GROUP BY h.facility_id
			HAVING count(*) FILTER (WHERE h.kolektabilitas >= 2 AND h.arrears_days BETWEEN 1 AND 15) >= 7
			    OR count(*) FILTER (WHERE h.kolektabilitas >= 2 AND h.arrears_days > 15) >= 2)`, reportID).Scan(&rejects)
	return rejects, err
}
