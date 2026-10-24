package app_test

import (
	"context"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"fos/internal/apptest"
)

// collateralData takes a case through QCA Data to its Collateral Data (Total
// Requested Amount 100.000.000), with test options for the lists the SoT
// does not seed.
func collateralData(t *testing.T) (*apptest.Harness, *apptest.Client, string, int64) {
	t.Helper()
	h, c, caseID, qid := qcaData(t)
	h.SeedBMV(t, "BENTUKTANAHBANGUNAN", "GD", "Gedung", 1, "Active")
	h.SeedBMV(t, "COLLATERAL_STATUS", "TS", "Tersedia", 1, "Active")
	h.SeedBMV(t, "COLLATERAL_JOINT_ACCOUNT", "JY", "Ya", 1, "Active")
	h.SeedBMV(t, "COLLATERAL_ASURANSI", "AY", "Ya", 1, "Active")
	return h, c, caseID, movedTo(t, h, caseID, submitQCA(t, c, caseID, qid, "Collateral Data", allAnswers()), "Collateral Data")
}

type card map[string]string

func submitCollateral(t *testing.T, c *apptest.Client, caseID string, id int64, to string, main, support []card) apptest.Response {
	t.Helper()
	if main == nil {
		main = []card{}
	}
	if support == nil {
		support = []card{}
	}
	return c.Do(t, http.MethodPost, prospectPath(caseID, id, "collateral", to), map[string]any{"main": main, "support": support})
}

// landBuilding is a filled Tanah dan Bangunan card with SHM.
func landBuilding() card {
	return card{"typeCode": "Tanah dan Bangunan", "buildingFormCode": "GD", "ownershipTypeCode": "1", "ownerName": "Budi", "shmNumber": "SHM-001",
		"physicalValue": "500000000", "location": "Jakarta", "collateralStatusCode": "TS", "collateralTypeText": "Rumah", "bindingType": "HT",
		"bindingDate": "2001-02-03", "njopValue": "400000000", "appraisalDate": "2099-01-01", "kjppValue": "450000000", "kjppAppraisalDate": "1990-05-06",
		"kjppAppraiserName": "KJPP A", "pariPassuPercent": "25", "jointFinancingCode": "JY", "pariPassuStatusCode": "Y", "insuredCode": "AY",
		"remarks": "Usaha 2 km"}
}

// collateralRows are the case's §9.48 rows, section and sequence first, a
// column per field ("-" for NULL).
func collateralRows(t *testing.T, h *apptest.Harness, caseID string) []string {
	t.Helper()
	rows, err := h.DB.Query(context.Background(), `
		SELECT concat_ws('|', collateral_section, sequence_number, collateral_type_code, COALESCE(building_form_code, '-'),
		       COALESCE(ownership_type_code, '-'), COALESCE(owner_name, '-'), COALESCE(shm_number, '-'), COALESCE(shgb_number, '-'),
		       COALESCE(shgb_valid_until::text, '-'), COALESCE(bpkb_number, '-'), COALESCE(fiduciary_number, '-'), COALESCE(location, '-'),
		       COALESCE(land_area, '-'), COALESCE(due_date::text, '-'), COALESCE(physical_value::text, '-'), COALESCE(object_value::text, '-'),
		       COALESCE(deposit_amount::text, '-'), COALESCE(vehicle_brand, '-'), COALESCE(vehicle_year::text, '-'), COALESCE(remarks, '-'),
		       COALESCE(collateral_status_code, '-'), COALESCE(collateral_type_text, '-'), COALESCE(binding_type, '-'),
		       COALESCE(binding_date::text, '-'), COALESCE(appraisal_date::text, '-'), COALESCE(kjpp_appraisal_date::text, '-'),
		       COALESCE(njop_value::text, '-'), COALESCE(kjpp_value::text, '-'), COALESCE(kjpp_appraiser_name, '-'),
		       COALESCE(pari_passu_status_code, '-'), COALESCE(pari_passu_percent::text, '-'), COALESCE(joint_financing_code, '-'),
		       COALESCE(insured_code, '-'))
		FROM fos.case_collateral WHERE case_id = $1 ORDER BY collateral_section, sequence_number`, caseID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

// AC-FR-APP-021-01, -02: Collateral Data opens with one empty card per
// section; the type lists come from MAINCOLLATERAL and COLLATERALSUPPORT.
func TestCollateralDataPage(t *testing.T) {
	_, c, caseID, id := collateralData(t)
	p := pipelinePage(t, c, caseID, id)
	v := p["values"].(map[string]any)
	main, support := v["main"].([]any), v["support"].([]any)
	if len(main) != 1 || len(support) != 1 || main[0].(map[string]any)["typeCode"] != "" || p["maxCards"] != float64(10) || p["today"] == "" {
		t.Fatalf("page = %v", p)
	}
	opts := p["options"].(map[string]any)
	for key, want := range map[string][]string{
		"mainCollateral":    {"Tanah dan Bangunan=Tanah dan Bangunan", "Deposito Bank Nano Syariah=Deposito Bank Nano Syariah"},
		"supportCollateral": {"Tanah Kosong Produktif=Tanah Kosong Produktif", "Kendaraan=Kendaraan"},
		"ownershipType":     {"1=SHM", "2=SHGB", "3=BPKB", "4=SHMRS", "5=Akta Jaminan Fidusia"},
		"pariPassuStatus":   {"Y=Ya", "T=Tidak"},
		"buildingForm":      {"GD=Gedung"},
	} {
		if got := optionTexts(opts[key]); !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %v", key, got)
		}
	}
}

// AC-FR-APP-021-04, -09, -12, -15, -17, DB-RULE-127: each type stores its own
// columns, the others NULL; only the number field of the Tipe Bukti
// Kepemilikan; paripasu 0 when not Y and NULL on Deposito; any past or
// future date for the binding and appraisal dates; empty cards dropped;
// Next opens Document Upload; reopened, the cards come back in order.
func TestCollateralDataNext(t *testing.T) {
	h, c, caseID, id := collateralData(t)
	land := landBuilding()
	land["shgbNumber"], land["bpkbNumber"] = "ignored", "ignored"
	deposit := card{"typeCode": "Deposito Bank Nano Syariah", "depositAmount": "0", "ownerName": "ignored", "pariPassuPercent": "50"}
	shgbUntil := yearsAgo(h, -100, 0)
	vacant := card{"typeCode": "Tanah Kosong Produktif", "ownershipTypeCode": "2", "shgbNumber": "SHGB-9", "shgbValidUntil": shgbUntil, "shmNumber": "ignored",
		"ownerName": "Ani", "location": "Bogor", "landArea": "200 m2", "dueDate": "2000-01-01", "pariPassuStatusCode": "T", "pariPassuPercent": "30"}
	year := strconv.Itoa(h.Clock.Now().In(time.FixedZone("GMT+7", 7*3600)).Year() + 1)
	vehicle := card{"typeCode": "Kendaraan", "bpkbNumber": "BPKB-7", "ownerName": "Budi", "objectValue": "999999999999999", "vehicleBrand": "Toyota Avanza",
		"vehicleYear": year, "ownershipTypeCode": "1", "location": "ignored"}
	fiduciary := card{"typeCode": "Tanah Kosong Produktif", "ownershipTypeCode": "5", "fiduciaryNumber": "F-1"}
	movedTo(t, h, caseID, submitCollateral(t, c, caseID, id, "Document Upload", []card{{}, land, deposit}, []card{vacant, {"typeCode": ""}, vehicle, fiduciary}), "Document Upload")
	if !processStarted(t, h, caseID, "Document Upload") {
		t.Fatal("process Document Upload not started")
	}
	want := []string{
		"MAIN|1|Tanah dan Bangunan|GD|1|Budi|SHM-001|-|-|-|-|Jakarta|-|-|500000000|-|-|-|-|Usaha 2 km|TS|Rumah|HT|2001-02-03|2099-01-01|1990-05-06|400000000|450000000|KJPP A|Y|25|JY|AY",
		"MAIN|2|Deposito Bank Nano Syariah|-|-|-|-|-|-|-|-|-|-|-|-|-|0|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-",
		"SUPPORT|1|Tanah Kosong Produktif|-|2|Ani|-|SHGB-9|" + shgbUntil + "|-|-|Bogor|200 m2|2000-01-01|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|T|0|-|-",
		"SUPPORT|2|Kendaraan|-|-|Budi|-|-|-|BPKB-7|-|-|-|-|-|999999999999999|-|Toyota Avanza|" + year + "|-|-|-|-|-|-|-|-|-|-|-|0|-|-",
		"SUPPORT|3|Tanah Kosong Produktif|-|5|-|-|-|-|-|F-1|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|0|-|-",
	}
	if got := collateralRows(t, h, caseID); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows\n got %q\nwant %q", got, want)
	}
	exec(t, h, `UPDATE fos.assignments SET assignment_status = 'sudah dibuka', completed_at = NULL WHERE assignment_id = $1`, id)
	v := pipelinePage(t, c, caseID, id)["values"].(map[string]any)
	main, support := v["main"].([]any), v["support"].([]any)
	if len(main) != 2 || len(support) != 3 || main[0].(map[string]any)["shmNumber"] != "SHM-001" || main[0].(map[string]any)["bindingDate"] != "2001-02-03" ||
		support[1].(map[string]any)["vehicleYear"] != year || support[0].(map[string]any)["pariPassuPercent"] != "0" {
		t.Fatalf("reopened = %v", v)
	}
}

// FR-APP-021 Validation, AC-FR-APP-021-05, -06, -08, -13, -14, -19, -20:
// required fields, paripasu, Berlaku Sampai, Tahun Pembuatan, lengths and
// codes, each on its card's section; more than ten cards; a refused Next
// stores nothing.
func TestCollateralDataValidation(t *testing.T) {
	h, c, caseID, id := collateralData(t)
	thisYear := h.Clock.Now().In(time.FixedZone("GMT+7", 7*3600)).Year()
	for _, tc := range []struct {
		name          string
		main, support []card
		want          fieldError
	}{
		{"AC-13 building form", []card{{"typeCode": "Tanah dan Bangunan", "pariPassuStatusCode": "T"}}, nil,
			fieldError{"main-collateral-1", "Tanah dan Bangunan Yang Berbentuk", "ERR-010", "Tanah dan Bangunan Yang Berbentuk wajib diisi"}},
		{"status paripasu on land and building", []card{{"typeCode": "Tanah dan Bangunan", "buildingFormCode": "GD"}}, nil,
			fieldError{"main-collateral-1", "Status Paripasu", "ERR-010", "Status Paripasu wajib diisi"}},
		{"AC-08 paripasu empty", nil, []card{{"typeCode": "Kendaraan", "pariPassuStatusCode": "Y"}},
			fieldError{"support-collateral-1", "Presentase Paripasu", "ERR-010", "Presentase Paripasu wajib diisi"}},
		{"AC-08 paripasu 0", nil, []card{{}, {"typeCode": "Kendaraan", "pariPassuStatusCode": "Y", "pariPassuPercent": "0"}},
			fieldError{"support-collateral-2", "Presentase Paripasu", "ERR-037", "Harap masukkan Presentase Paripasu yang valid."}},
		{"AC-19 paripasu 12,5", nil, []card{{"typeCode": "Kendaraan", "pariPassuStatusCode": "Y", "pariPassuPercent": "12.5"}},
			fieldError{"support-collateral-1", "Presentase Paripasu", "ERR-037", "Harap masukkan Presentase Paripasu yang valid."}},
		{"paripasu 101", nil, []card{{"typeCode": "Kendaraan", "pariPassuStatusCode": "Y", "pariPassuPercent": "101"}},
			fieldError{"support-collateral-1", "Presentase Paripasu", "ERR-037", "Harap masukkan Presentase Paripasu yang valid."}},
		{"AC-05 Berlaku Sampai today", nil, []card{{"typeCode": "Tanah Kosong Produktif", "ownershipTypeCode": "2", "shgbValidUntil": yearsAgo(h, 0, 0)}},
			fieldError{"support-collateral-1", "Berlaku Sampai", "ERR-037", "Harap masukkan Berlaku Sampai yang valid."}},
		{"AC-05 Berlaku Sampai past 100 years", nil, []card{{"typeCode": "Tanah Kosong Produktif", "ownershipTypeCode": "2", "shgbValidUntil": yearsAgo(h, -100, 1)}},
			fieldError{"support-collateral-1", "Berlaku Sampai", "ERR-037", "Harap masukkan Berlaku Sampai yang valid."}},
		{"AC-14 year 1899", nil, []card{{"typeCode": "Kendaraan", "vehicleYear": "1899"}},
			fieldError{"support-collateral-1", "Tahun Pembuatan", "ERR-037", "Harap masukkan Tahun Pembuatan yang valid."}},
		{"AC-14 year after next", nil, []card{{"typeCode": "Kendaraan", "vehicleYear": strconv.Itoa(thisYear + 2)}},
			fieldError{"support-collateral-1", "Tahun Pembuatan", "ERR-037", "Harap masukkan Tahun Pembuatan yang valid."}},
		{"invalid date", nil, []card{{"typeCode": "Kendaraan", "bindingDate": "2023-02-30"}},
			fieldError{"support-collateral-1", "Tanggal Pengikatan", "ERR-037", "Harap masukkan Tanggal Pengikatan yang valid."}},
		{"money 16 digits", []card{{"typeCode": "Deposito Bank Nano Syariah", "depositAmount": "1000000000000000"}}, nil,
			fieldError{"main-collateral-1", "Jumlah Deposito Bank Nano Syariah", "ERR-037", "Harap masukkan Jumlah Deposito Bank Nano Syariah yang valid."}},
		{"land area 51", nil, []card{{"typeCode": "Tanah Kosong Produktif", "landArea": strings.Repeat("a", 51)}},
			fieldError{"support-collateral-1", "Luas Tanah", "ERR-040", "Luas Tanah maksimal 50 karakter"}},
		{"remarks 501", []card{{"typeCode": "Tanah dan Bangunan", "buildingFormCode": "GD", "pariPassuStatusCode": "T", "remarks": strings.Repeat("a", 501)}}, nil,
			fieldError{"main-collateral-1", "Agunan diperuntukan dan Jarak lokasi usaha dari kantor cabang Bank Nano Syariah", "ERR-040",
				"Agunan diperuntukan dan Jarak lokasi usaha dari kantor cabang Bank Nano Syariah maksimal 500 karakter"}},
		{"type of the other section", []card{{"typeCode": "Kendaraan"}}, nil,
			fieldError{"main-collateral-1", "Agunan Utama", "ERR-037", "Harap masukkan Agunan Utama yang valid."}},
		{"ownership code", nil, []card{{"typeCode": "Tanah Kosong Produktif", "ownershipTypeCode": "9"}},
			fieldError{"support-collateral-1", "Tipe Bukti Kepemilikan", "ERR-037", "Harap masukkan Tipe Bukti Kepemilikan yang valid."}},
	} {
		if got := fieldErrors(t, submitCollateral(t, c, caseID, id, "Document Upload", tc.main, tc.support)); !reflect.DeepEqual(got, []fieldError{tc.want}) {
			t.Errorf("%s = %v", tc.name, got)
		}
	}
	// AC-06: tomorrow and exactly 100 years ahead pass.
	for _, until := range []string{yearsAgo(h, 0, 1), yearsAgo(h, -100, 0)} {
		if r := submitCollateral(t, c, caseID, id, "save", nil, []card{{"typeCode": "Tanah Kosong Produktif", "ownershipTypeCode": "2", "shgbValidUntil": until}}); r.Status != http.StatusOK {
			t.Errorf("Berlaku Sampai %s = %d %s", until, r.Status, r.Raw)
		}
	}
	exec(t, h, `DELETE FROM fos.case_collateral WHERE case_id = $1`, caseID)
	// AC-20: eleven filled cards on one section.
	eleven := make([]card, 11)
	for i := range eleven {
		eleven[i] = card{"typeCode": "Deposito Bank Nano Syariah"}
	}
	if got := fieldErrors(t, submitCollateral(t, c, caseID, id, "Document Upload", eleven, nil)); !reflect.DeepEqual(got, []fieldError{{"form", "Agunan Utama", "ERR-037", "Harap masukkan Agunan Utama yang valid."}}) {
		t.Errorf("AC-20 = %v", got)
	}
	if got := collateralRows(t, h, caseID); len(got) != 0 {
		t.Fatalf("refused Next stored %v", got)
	}
	// Ten pass.
	movedTo(t, h, caseID, submitCollateral(t, c, caseID, id, "Document Upload", eleven[:10], nil), "Document Upload")
}

// AC-FR-APP-021-10, -11, ERR-044: over 100.000.000 a Main Collateral is
// needed on every forward move (agreed comment 4); at 100.000.000 all cards
// may be empty and nothing is stored.
func TestCollateralDataMainRequired(t *testing.T) {
	h, c, caseID, id := collateralData(t)
	exec(t, h, `UPDATE fos.case_financing SET total_requested_amount = 100000001 WHERE case_id = $1`, caseID)
	need := []fieldError{{"form", "Agunan Utama", "ERR-044", "Agunan Utama minimal 1 untuk total nilai pengajuan lebih dari Rp100.000.000."}}
	if got := fieldErrors(t, submitCollateral(t, c, caseID, id, "Document Upload", []card{{}}, []card{{"typeCode": "Kendaraan"}})); !reflect.DeepEqual(got, need) {
		t.Fatalf("AC-10 = %v", got)
	}
	if got := collateralRows(t, h, caseID); len(got) != 0 {
		t.Fatalf("refused Next stored %v", got)
	}
	// Save and a move back do not need it.
	if r := submitCollateral(t, c, caseID, id, "save", nil, []card{{"typeCode": "Kendaraan"}}); r.Status != http.StatusOK {
		t.Fatalf("save = %d %s", r.Status, r.Raw)
	}
	exec(t, h, `UPDATE fos.case_financing SET total_requested_amount = 100000000 WHERE case_id = $1`, caseID)
	movedTo(t, h, caseID, submitCollateral(t, c, caseID, id, "Document Upload", []card{{}}, []card{{}}), "Document Upload")
	if got := collateralRows(t, h, caseID); len(got) != 0 {
		t.Fatalf("AC-11 stored %v", got)
	}
}

// Agreed comments 3, 4, 10: Save checks formats but no required field and
// stays; Back to Previous Step validates and saves; a saved code now
// Inactive still shows and passes; the form takes only its fields.
func TestCollateralDataSaveAndMoves(t *testing.T) {
	h, c, caseID, id := collateralData(t)
	if r := submitCollateral(t, c, caseID, id, "save", []card{{"typeCode": "Tanah dan Bangunan"}}, nil); r.Status != http.StatusOK {
		t.Fatalf("save = %d %s", r.Status, r.Raw)
	}
	if got := collateralRows(t, h, caseID); len(got) != 1 || len(assignmentsNamed(t, h, caseID, "Document Upload")) != 0 {
		t.Fatalf("after save %v", got)
	}
	if got := fieldErrors(t, submitCollateral(t, c, caseID, id, "save", nil, []card{{"typeCode": "Kendaraan", "vehicleYear": "12"}})); len(got) != 1 || got[0].Field != "Tahun Pembuatan" {
		t.Fatalf("save formats = %v", got)
	}
	if got := fieldErrors(t, submitCollateral(t, c, caseID, id, "QCA Data", []card{{"typeCode": "Tanah dan Bangunan"}}, nil)); len(got) != 2 {
		t.Fatalf("back without required = %v", got)
	}
	if r := c.Do(t, http.MethodPost, prospectPath(caseID, id, "collateral", "save"), map[string]any{"main": []card{{"typeCode": "Kendaraan", "color": "x"}}, "support": []card{}}); r.Status != http.StatusBadRequest {
		t.Fatalf("another field = %d", r.Status)
	}
	land := landBuilding()
	movedTo(t, h, caseID, submitCollateral(t, c, caseID, id, "QCA Data", []card{land}, nil), "QCA Data")
	exec(t, h, `UPDATE fos.business_mapping_value SET status = 'Inactive' WHERE type = 'BENTUKTANAHBANGUNAN' AND code = 'GD'`)
	exec(t, h, `UPDATE fos.assignments SET assignment_status = 'sudah dibuka', completed_at = NULL WHERE assignment_id = $1`, id)
	if got := optionTexts(pipelinePage(t, c, caseID, id)["options"].(map[string]any)["buildingForm"]); !reflect.DeepEqual(got, []string{"GD=Gedung"}) {
		t.Fatalf("saved Inactive option = %v", got)
	}
	movedTo(t, h, caseID, submitCollateral(t, c, caseID, id, "Document Upload", []card{land}, nil), "Document Upload")
}
