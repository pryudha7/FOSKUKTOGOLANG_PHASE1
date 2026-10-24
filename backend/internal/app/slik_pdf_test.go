package app_test

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"fos/internal/apptest"
)

var (
	pdfStream = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
	pdfShow   = regexp.MustCompile(`\(((?:\\.|[^\\)])*)\) ?Tj`)
)

// pdfContent is a PDF's decompressed content streams.
func pdfContent(data []byte) string {
	var out strings.Builder
	for _, m := range pdfStream.FindAllSubmatch(data, -1) {
		if r, err := zlib.NewReader(bytes.NewReader(m[1])); err == nil {
			content, _ := io.ReadAll(r)
			out.Write(content)
		}
	}
	return out.String()
}

// pdfText is the text a PDF shows, one shown string per line, read from its
// compressed content streams.
func pdfText(t *testing.T, data []byte) string {
	t.Helper()
	var out strings.Builder
	for _, m := range pdfStream.FindAllSubmatch(data, -1) {
		r, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue
		}
		content, _ := io.ReadAll(r)
		for _, s := range pdfShow.FindAllSubmatch(content, -1) {
			text := strings.NewReplacer(`\(`, "(", `\)`, ")", `\\`, `\`).Replace(string(s[1]))
			out.WriteString(text + "\n")
		}
	}
	return out.String()
}

// storedFile is a case document's row and its object in the fake GCS.
type storedFile struct {
	FileID, Name, Original, CreatedBy, Object string
	Size                                      int64
	Data                                      []byte
}

func storedDocument(t *testing.T, h *apptest.Harness, caseID, category, docType string) (storedFile, bool) {
	t.Helper()
	var f storedFile
	err := h.DB.QueryRow(context.Background(), `
		SELECT file_id::text, file_name, original_file_name, created_by, object_name, file_size_bytes FROM fos.case_document
		WHERE case_id = $1 AND document_category = $2 AND document_type = $3`, caseID, category, docType).
		Scan(&f.FileID, &f.Name, &f.Original, &f.CreatedBy, &f.Object, &f.Size)
	if err != nil {
		return f, false
	}
	for key, o := range fakeGCSOf[h].Objects() {
		if strings.HasSuffix(key, f.Object) {
			f.Data = o.Data
		}
	}
	return f, true
}

// reportFacility is a facility for the PDF checks: months from Sep 2024 to
// Aug 2026 with month 03 empty, Kol 0 / Ht 0 in month 04, a contract date
// and a plafon to format.
func reportFacility() map[string]any {
	f := facility("1")
	for m := 1; m <= 24; m++ {
		y, mo := 2024, 8+m
		for mo > 12 {
			y, mo = y+1, mo-12
		}
		f[fmt.Sprintf("tahunBulan%02d", m)] = fmt.Sprintf("%d%02d", y, mo)
	}
	f["tahunBulan03"], f["tahunBulan03Kol"], f["tahunBulan03Ht"] = "", "", ""
	f["tahunBulan04Kol"], f["tahunBulan04Ht"] = "0", "0"
	f["tanggalAkadAwal"], f["plafon"] = "20230509", "485000000"
	f["penjamin"] = []any{map[string]any{"namaPenjamin": "Ayah Budi", "nomorIdentitas": "3171000000000001", "keteranganJenisPenjamin": "Perorangan"}}
	return f
}

// AC-FR-PSAPI-012-01, -02, -03, AC-FR-PSAPI-010-07 (PDF): the Customer's
// result gets an A3 "Informasi Debitur" PDF with its sections in order, the
// 24-month grid labelled Sep 24 to Aug 26 with month 03 empty, dates as
// "9 Mei 2023", facility amounts as "Rp 485,000,000.00" and summary amounts
// without "Rp"; it is the SLIK_PDF_CUSTOMER document, made by the RPA and
// referenced by the report, stored with the result.
func TestSLIKReportPDF(t *testing.T) {
	h, _, caseID := slikCase(t, true)
	kc, ks := referenceCode(t, h, caseID, "CUSTOMER"), referenceCode(t, h, caseID, "SPOUSE")
	if r := callPBS(t, h, kc, slikZip(t, kc, "3171011234567890", reportFacility())); r.Body["Code"] != "0" {
		t.Fatalf("answer = %s", r.Raw)
	}
	pdf, ok := storedDocument(t, h, caseID, "PreScreeningResponse", "SLIK_PDF_CUSTOMER")
	if !ok || pdf.Name != "SLIK Report - "+customerCode(t, h, caseID)+".pdf" || pdf.CreatedBy != rpaUser || !bytes.HasPrefix(pdf.Data, []byte("%PDF-")) {
		t.Fatalf("pdf = %+v", pdf)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.slik_report WHERE case_id = $1 AND subject_type = 'CUSTOMER' AND pdf_document_id::text = $2`, caseID, pdf.FileID); n != 1 {
		t.Fatal("slik_report does not reference the PDF")
	}
	if !bytes.Contains(pdf.Data, []byte("/MediaBox [0 0 841.89 1190.55]")) {
		t.Fatal("not an A3 portrait page")
	}
	text := pdfText(t, pdf.Data)
	last := -1
	for _, part := range []string{"Informasi Debitur", "RAHASIA", "kata kunci pencarian", "BUDI SANTOSO", "Data Pokok Debitur",
		"Ringkasan Fasilitas", "Plafon Efektif", "Jumlah Kreditur", "Kredit/Pembiayaan", "Kolektabilitas/Jumlah",
		"Sep 24", "Oct 24", "Dec 24", "Aug 25", "Sep 25", "Aug 26", "Rp 485,000,000.00", "9 Mei 2023", "Agunan", "Tanah",
		"Penjamin", "Ayah Budi", "Nomor Laporan R000001", "Disclaimer: PDF BERIKUT MERUPAKAN DUPLIKASI DARI SLIK OJK"} {
		i := strings.Index(text, part)
		if i <= last {
			t.Fatalf("%q missing or out of order in:\n%s", part, text)
		}
		last = i
	}
	// AC-FR-PSAPI-012-02: month 04 (Kol 0, Ht 0) writes its zeros white.
	if !regexp.MustCompile(`1\.000 g BT [^\n]*\(0\) ?Tj`).MatchString(pdfContent(pdf.Data)) {
		t.Fatal("no white 0 in the grid")
	}
	if strings.Contains(text, "Nov 24") || !strings.Contains(text, "\n10,000,000.00\n") || strings.Contains(text, "Rp 10,000,000.00") {
		t.Fatalf("grid or summary amounts wrong:\n%s", text)
	}
	if os.Getenv("SLIK_PDF_SAMPLE") != "" {
		_ = os.WriteFile(os.Getenv("SLIK_PDF_SAMPLE"), pdf.Data, 0o644)
	}

	callPBS(t, h, ks, slikZip(t, ks, spouseNIK, facility("1")))
	if sp, ok := storedDocument(t, h, caseID, "PreScreeningResponse", "SLIK_PDF_SPOUSE"); !ok || sp.Name != "SLIK Report - "+customerCode(t, h, caseID)+" - Spouse.pdf" {
		t.Fatalf("spouse pdf = %+v", sp)
	}
}

// FR-PSAPI-012 Susunan Bagian 6: without Data Pokok Debitur the PDF says
// "Data Tidak Ditemukan".
func TestSLIKReportPDFWithoutDebtorData(t *testing.T) {
	h, _, caseID := slikCase(t, false)
	kc := referenceCode(t, h, caseID, "CUSTOMER")
	doc := slikDoc(kc, "3171011234567890")
	doc["individual"].(map[string]any)["dataPokokDebitur"] = []any{}
	if r := callPBS(t, h, kc, slikZipOf(t, "3171011234567890", doc)); r.Body["Code"] != "0" {
		t.Fatalf("answer = %s", r.Raw)
	}
	pdf, _ := storedDocument(t, h, caseID, "PreScreeningResponse", "SLIK_PDF_CUSTOMER")
	if !strings.Contains(pdfText(t, pdf.Data), "Data Tidak Ditemukan") {
		t.Fatal("no Data Tidak Ditemukan")
	}
}

// AC-FR-PSAPI-010-06, DB-RULE-097 (1): opening Document Underlying Upload
// with Dokumen Pre-Screening's SLIK row empty copies the Customer's
// automatic SLIK PDF into it, a file of its own made by the opener; an
// upload then replaces the copy and leaves the automatic PDF as it was.
func TestSLIKReportCopiedIntoDokumenPreScreening(t *testing.T) {
	h, _, caseID := slikCase(t, true)
	kc := referenceCode(t, h, caseID, "CUSTOMER")
	callPBS(t, h, kc, slikZip(t, kc, "3171011234567890", facility("1")))
	timedOut(t, h, caseID)
	auto, _ := storedDocument(t, h, caseID, "PreScreeningResponse", "SLIK_PDF_CUSTOMER")

	ao := relogin(t, h, "AO001")
	as := assignments(t, h, caseID)
	pss := as[len(as)-1].ID
	ao.OpenAssignment(t, caseID, pss)
	duu := toUnderlying(t, h, ao, caseID, pss)
	ao.OpenAssignment(t, caseID, duu)
	cp, ok := storedDocument(t, h, caseID, "DokumenPreScreening", "SLIK")
	if !ok || cp.FileID == auto.FileID || cp.Object == auto.Object || cp.Original != auto.Original || cp.Name != auto.Name ||
		cp.Size != auto.Size || cp.CreatedBy != "AO001" || !bytes.Equal(cp.Data, auto.Data) {
		t.Fatalf("copy %+v of %+v", cp, auto)
	}
	ao.OpenAssignment(t, caseID, duu)
	if n := count(t, h, `SELECT count(*) FROM fos.case_document WHERE case_id = $1 AND document_type = 'SLIK'`, caseID); n != 1 {
		t.Fatalf("SLIK rows after reopening = %d", n)
	}

	if r := upload(t, ao, caseID, duu, "SLIK", "SLIK", "slik-manual.pdf", []byte("%PDF manual")); r.Status != http.StatusCreated {
		t.Fatalf("upload = %d %s", r.Status, r.Raw)
	}
	up, _ := storedDocument(t, h, caseID, "DokumenPreScreening", "SLIK")
	still, _ := storedDocument(t, h, caseID, "PreScreeningResponse", "SLIK_PDF_CUSTOMER")
	if up.Original != "slik-manual.pdf" || still.FileID != auto.FileID || !bytes.Equal(still.Data, auto.Data) {
		t.Fatalf("after upload: %+v / %+v", up, still)
	}
}

// AC-FR-PSAPI-010-07: a new result for the same slot (e.g. after a reopen)
// replaces the PDF: one SLIK_PDF_CUSTOMER item on the attachments panel,
// referenced by the report; the old object is deleted.
func TestSLIKReportPDFSlotIsReplaced(t *testing.T) {
	h, _, caseID := slikCase(t, true)
	kc := referenceCode(t, h, caseID, "CUSTOMER")
	callPBS(t, h, kc, slikZip(t, kc, "3171011234567890", facility("1")))
	old, _ := storedDocument(t, h, caseID, "PreScreeningResponse", "SLIK_PDF_CUSTOMER")
	// As a reopen would (TBD-042): the report waits again without its parsed rows.
	for _, sql := range []string{
		`DELETE FROM fos.slik_facility_history WHERE facility_id IN (SELECT facility_id FROM fos.slik_facility f JOIN fos.slik_report r USING (slik_report_id) WHERE r.reference_code = $1)`,
		`DELETE FROM fos.slik_collateral WHERE facility_id IN (SELECT facility_id FROM fos.slik_facility f JOIN fos.slik_report r USING (slik_report_id) WHERE r.reference_code = $1)`,
		`DELETE FROM fos.slik_guarantor WHERE facility_id IN (SELECT facility_id FROM fos.slik_facility f JOIN fos.slik_report r USING (slik_report_id) WHERE r.reference_code = $1)`,
		`DELETE FROM fos.slik_facility WHERE slik_report_id IN (SELECT slik_report_id FROM fos.slik_report WHERE reference_code = $1)`,
		`DELETE FROM fos.slik_debtor WHERE slik_report_id IN (SELECT slik_report_id FROM fos.slik_report WHERE reference_code = $1)`,
		`UPDATE fos.slik_report SET status = 'REQUESTED' WHERE reference_code = $1`,
	} {
		exec(t, h, sql, kc)
	}
	if r := callPBS(t, h, kc, slikZip(t, kc, "3171011234567890", facility("2"))); r.Body["Code"] != "0" {
		t.Fatalf("answer = %s", r.Raw)
	}
	now, _ := storedDocument(t, h, caseID, "PreScreeningResponse", "SLIK_PDF_CUSTOMER")
	if now.FileID == old.FileID || count(t, h, `SELECT count(*) FROM fos.slik_report WHERE reference_code = $1 AND pdf_document_id::text = $2`, kc, now.FileID) != 1 {
		t.Fatalf("pdf %+v after %+v", now, old)
	}
	for key := range fakeGCSOf[h].Objects() {
		if strings.HasSuffix(key, old.Object) {
			t.Fatal("old PDF object kept")
		}
	}
	r := relogin(t, h, "AO001").Case(t, caseID)
	pdfs := 0
	for _, a := range r.Body["attachments"].([]any) {
		if a.(map[string]any)["fileName"] == now.Name {
			pdfs++
		}
	}
	if pdfs != 1 {
		t.Fatalf("attachments = %v", r.Body["attachments"])
	}
}

// Agreed decision 5 and the review: a PDF that cannot be stored, or named
// for want of a customer code, leaves the result unprocessed with Code 1;
// the RPA may send it again.
func TestSLIKResultWithoutItsPDFIsNotProcessed(t *testing.T) {
	h, _, caseID := slikCase(t, false)
	kc := referenceCode(t, h, caseID, "CUSTOMER")
	data := slikZip(t, kc, "3171011234567890", facility("1"))

	fakeGCSOf[h].SetDown(true)
	if r := callPBS(t, h, kc, data); r.Body["Code"] != "1" {
		t.Fatalf("GCS down = %s", r.Raw)
	}
	fakeGCSOf[h].SetDown(false)
	exec(t, h, `UPDATE fos.work SET customer_code = NULL WHERE case_id = $1`, caseID)
	if r := callPBS(t, h, kc, data); r.Body["Code"] != "1" {
		t.Fatalf("no customer code = %s", r.Raw)
	}
	if n := count(t, h, `SELECT count(*) FROM fos.slik_report WHERE reference_code = $1 AND status = 'REQUESTED'`, kc); n != 1 ||
		count(t, h, `SELECT count(*) FROM fos.case_document WHERE case_id = $1 AND document_category = 'PreScreeningResponse'`, caseID) != 0 {
		t.Fatal("result processed without its PDF")
	}
	exec(t, h, `UPDATE fos.work SET customer_code = 'SMSME2600000001' WHERE case_id = $1`, caseID)
	if r := callPBS(t, h, kc, data); r.Body["Code"] != "0" {
		t.Fatalf("resend = %s", r.Raw)
	}
}
