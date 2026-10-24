package cases

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// The SLIK report PDF "Informasi Debitur" (FR-PSAPI-010 Laporan PDF,
// FR-PSAPI-012): one per received SLIK file, laid out as the legacy script
// did, from the same JSON the result's tables keep (summary_json,
// detail_json). Helvetica stands for Tahoma; a table row or a grid row is
// never split across pages (agreed for TBD-045).

// Page and colours (FR-PSAPI-012 Halaman dan Gaya); millimetres.
const (
	pdfMarginX, pdfMarginY = 15.0, 10.0
	pdfLine                = 4.2 // one line of 8 pt text
)

type rgb struct{ r, g, b int }

var (
	colRed       = rgb{255, 0, 0}
	colNavy      = rgb{0, 38, 87}
	colGrey      = rgb{217, 217, 217}
	colLightGrey = rgb{240, 240, 240}
	colBanner    = rgb{213, 215, 225}
	colWhite     = rgb{255, 255, 255}
	colBlack     = rgb{0, 0, 0}
)

// Month names for the date formats (FR-PSAPI-012 Format Nilai).
var monthsID = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

// slikPDF draws one report.
type slikPDF struct {
	pdf   *fpdf.Fpdf
	tr    func(string) string
	width float64 // printable width
}

// cell is one label and its value in a row of label cells.
type cell struct{ label, value string }

// renderSLIKPDF draws the report of one SLIK file.
func renderSLIKPDF(doc map[string]any) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A3", "")
	pdf.SetMargins(pdfMarginX, pdfMarginY, pdfMarginX)
	pdf.SetAutoPageBreak(false, pdfMarginY)
	pdf.SetCreator("FOS", false)
	pdf.AddPage()
	w, _ := pdf.GetPageSize()
	p := &slikPDF{pdf: pdf, tr: pdf.UnicodeTranslatorFromDescriptor(""), width: w - 2*pdfMarginX}

	header := obj(doc, "header")
	individual := obj(doc, "individual")
	p.head()
	p.keywords(header, individual)
	p.debtors(arr(individual, "dataPokokDebitur"))
	p.summary(obj(individual, "ringkasanFasilitas"))
	for _, f := range arr(obj(individual, "fasilitas"), "kreditPembiayan") {
		p.facility(f)
	}
	p.closing(header, individual)

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("SLIK report PDF: %w", err)
	}
	return out.Bytes(), nil
}

// text is a file value as shown: its text, or "".
func text(m map[string]any, key string) string {
	if v := field(m, key); v != nil {
		return *v
	}
	return ""
}

// dateID is yyyyMMdd[HHmmss] as "9 Mei 2023"; "" when it is not a date.
func dateID(s string) string {
	if len(s) < 8 {
		return ""
	}
	t, err := time.Parse("20060102", s[:8])
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d %s %d", t.Day(), monthsID[t.Month()-1], t.Year())
}

// monthOf reads yyyyMM.
func monthOf(s string) (time.Time, bool) {
	if len(s) < 6 {
		return time.Time{}, false
	}
	t, err := time.Parse("200601", s[:6])
	return t, err == nil
}

// monthID is yyyyMM as "September 2024"; monthEN as "February 2023"
// (legacy English, FR-PSAPI-012).
func monthID(s string) string {
	t, ok := monthOf(s)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s %d", monthsID[t.Month()-1], t.Year())
}

func monthEN(s string) string {
	t, ok := monthOf(s)
	if !ok {
		return ""
	}
	return t.Format("January 2006")
}

// amount is a decimal with thousands commas and two decimals
// (1,103,713,259.00); "" when it is not a number.
func amount(s string) string {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(s))
	if !ok {
		return ""
	}
	fixed := r.FloatString(2)
	sign := ""
	if strings.HasPrefix(fixed, "-") {
		sign, fixed = "-", fixed[1:]
	}
	whole, frac, _ := strings.Cut(fixed, ".")
	var b strings.Builder
	for i, d := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(d)
	}
	return sign + b.String() + "." + frac
}

// rupiah is an amount of a facility block: "Rp 485,000,000.00".
func rupiah(s string) string {
	if a := amount(s); a != "" {
		return "Rp " + a
	}
	return ""
}

// orDash shows "-" for an empty Keterangan (FR-PSAPI-012).
func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func (p *slikPDF) color(c rgb) { p.pdf.SetTextColor(c.r, c.g, c.b) }
func (p *slikPDF) fill(c rgb)  { p.pdf.SetFillColor(c.r, c.g, c.b) }

// ensure starts a new page when h millimetres do not fit.
func (p *slikPDF) ensure(h float64) {
	_, pageH := p.pdf.GetPageSize()
	if p.pdf.GetY()+h > pageH-pdfMarginY {
		p.pdf.AddPage()
	}
}

// lines splits a value into the lines it takes in width w.
func (p *slikPDF) lines(s string, w float64) []string {
	if s == "" {
		return []string{""}
	}
	return p.pdf.SplitText(p.tr(s), w-2)
}

// title is a section title: 14 pt, underlined.
func (p *slikPDF) title(s string, c rgb) {
	p.ensure(12)
	p.pdf.Ln(3)
	p.pdf.SetFont("Helvetica", "BU", 14)
	p.color(c)
	p.pdf.CellFormat(p.width, 7, p.tr(s), "", 1, "L", false, 0, "")
	p.pdf.Ln(1)
}

// labelRow draws cells side by side: a label line (filled when fill is
// set) above the value lines; widths are equal unless given.
func (p *slikPDF) labelRow(cells []cell, labelColor rgb, fill *rgb, widths ...float64) {
	if len(widths) == 0 {
		for range cells {
			widths = append(widths, p.width/float64(len(cells)))
		}
	}
	p.pdf.SetFont("Helvetica", "", 8)
	n := 1
	for i, c := range cells {
		n = max(n, len(p.lines(c.value, widths[i])))
	}
	p.ensure(pdfLine * float64(n+1))
	x, y := p.pdf.GetX(), p.pdf.GetY()
	for i, c := range cells {
		p.pdf.SetXY(x, y)
		p.pdf.SetFont("Helvetica", "B", 8)
		p.color(labelColor)
		if fill != nil {
			p.fill(*fill)
		}
		p.pdf.CellFormat(widths[i], pdfLine, p.tr(c.label), "", 0, "L", fill != nil, 0, "")
		p.pdf.SetFont("Helvetica", "", 8)
		p.color(colBlack)
		for j, l := range p.lines(c.value, widths[i]) {
			p.pdf.SetXY(x, y+pdfLine*float64(j+1))
			p.pdf.CellFormat(widths[i], pdfLine, l, "", 0, "L", false, 0, "")
		}
		x += widths[i]
	}
	p.pdf.SetXY(pdfMarginX, y+pdfLine*float64(n+1)+1)
}

// pairs draws label/value pairs, two per line, labels filled (the detail
// tables of a facility and of a collateral).
func (p *slikPDF) pairs(items []cell) {
	lw, vw := p.width*0.2, p.width*0.3
	for i := 0; i < len(items); i += 2 {
		row := items[i:min(i+2, len(items))]
		p.pdf.SetFont("Helvetica", "", 8)
		n := 1
		for _, c := range row {
			n = max(n, len(p.lines(c.value, vw)), len(p.lines(c.label, lw)))
		}
		h := pdfLine * float64(n)
		p.ensure(h)
		x, y := pdfMarginX, p.pdf.GetY()
		for _, c := range row {
			p.fill(colLightGrey)
			p.pdf.SetXY(x, y)
			p.pdf.SetFont("Helvetica", "B", 8)
			p.color(colNavy)
			p.pdf.MultiCell(lw, h/float64(len(p.lines(c.label, lw))), p.tr(c.label), "1", "L", true)
			p.pdf.SetXY(x+lw, y)
			p.pdf.SetFont("Helvetica", "", 8)
			p.color(colBlack)
			p.pdf.MultiCell(vw, h/float64(len(p.lines(c.value, vw))), p.tr(c.value), "1", "L", false)
			x += lw + vw
		}
		p.pdf.SetXY(pdfMarginX, y+h)
	}
}

func (p *slikPDF) head() {
	pdf := p.pdf
	pdf.SetFont("Helvetica", "B", 17.5)
	p.color(colRed)
	pdf.CellFormat(p.width, 8, p.tr("Informasi Debitur"), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8)
	p.color(colNavy)
	pdf.CellFormat(p.width, pdfLine, p.tr("Sistem Layanan Informasi Keuangan"), "", 1, "L", false, 0, "")
	pdf.Ln(2)
	boxW := 30.0
	y := pdf.GetY()
	p.fill(colBanner)
	pdf.SetFont("Helvetica", "I", 8)
	p.color(colBlack)
	pdf.MultiCell(p.width-boxW, pdfLine, p.tr("Informasi ini bersifat RAHASIA dan hanya digunakan untuk kepentingan pemohon informasi. Akibat yang timbul dari penggunaan informasi ini bukan merupakan tanggung jawab Otoritas Jasa Keuangan"), "", "L", true)
	h := pdf.GetY() - y
	pdf.SetXY(pdfMarginX+p.width-boxW, y)
	p.fill(colRed)
	p.color(colWhite)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(boxW, h, "RAHASIA", "", 1, "C", true, 0, "")
	pdf.Ln(3)
}

func (p *slikPDF) keywords(header, individual map[string]any) {
	pdf := p.pdf
	search := obj(individual, "parameterPencarian")
	pdf.SetFont("Helvetica", "I", 8)
	p.color(colBlack)
	pdf.Write(pdfLine, p.tr("Informasi diberikan berdasarkan laporan yang dikirimkan oleh pelapor ke dalam Sistem Layanan Informasi Keuangan dengan "))
	pdf.SetFont("Helvetica", "IU", 8)
	p.color(colRed)
	pdf.Write(pdfLine, p.tr("kata kunci pencarian"))
	pdf.SetFont("Helvetica", "I", 8)
	p.color(colBlack)
	pdf.Write(pdfLine, p.tr(" sebagai berikut:"))
	pdf.Ln(pdfLine + 2)

	// Left two thirds: the search parameters; right third: the request.
	left, right := p.width*2/3, p.width/3
	y := pdf.GetY()
	saved := p.width
	p.width = left
	p.labelRow([]cell{{"Nama", text(search, "namaDebitur")}, {"Jenis Kelamin", text(search, "jenisKelaminKet")}}, colNavy, nil)
	p.labelRow([]cell{{"No. Identitas", text(search, "noIdentitas")}, {"NPWP", text(search, "npwp")},
		{"Tempat Lahir", text(search, "tempatLahir")}, {"Tanggal Lahir", dateID(text(search, "tanggalLahir"))}}, colNavy, nil)
	bottom := pdf.GetY()
	p.width = right
	pdf.SetXY(pdfMarginX+left, y)
	for _, c := range []cell{
		{"Kode Ref. Pengguna", text(header, "kodeReferensiPengguna")},
		{"Nomor Laporan", text(individual, "nomorLaporan")},
		{"Posisi Data Terakhir", monthID(text(individual, "posisiDataTerakhir"))},
		{"Tanggal Permintaan", dateID(text(individual, "tanggalPermintaan"))},
	} {
		pdf.SetX(pdfMarginX + left)
		pdf.SetFont("Helvetica", "B", 8)
		p.color(colNavy)
		pdf.CellFormat(right*0.45, pdfLine, p.tr(c.label), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		p.color(colBlack)
		pdf.CellFormat(right*0.55, pdfLine, p.tr(c.value), "", 1, "L", false, 0, "")
	}
	p.width = saved
	pdf.SetXY(pdfMarginX, max(bottom, pdf.GetY()))
}

func (p *slikPDF) debtors(items []map[string]any) {
	p.title("Data Pokok Debitur", colNavy)
	p.pdf.SetFont("Helvetica", "", 8)
	p.color(colBlack)
	p.pdf.MultiCell(p.width, pdfLine, p.tr("Penyajian informasi debitur pada Sistem Layanan Informasi Keuangan dikelompokkan berdasarkan nomor identitas debitur. Pengguna informasi diharapkan dapat meneliti kembali kemungkinan adanya debitur berbeda yang dilaporkan menggunakan nomor identitas yang sama"), "", "L", false)
	p.pdf.Ln(2)
	if len(items) == 0 {
		p.pdf.SetFont("Helvetica", "B", 20)
		p.color(colNavy)
		p.pdf.CellFormat(p.width, 12, p.tr("Data Tidak Ditemukan"), "", 1, "C", false, 0, "")
		return
	}
	grey := colGrey
	for _, d := range items {
		p.labelRow([]cell{
			{"Nama Sesuai Identitas", text(d, "namaDebitur")},
			{"Identitas", "NIK / " + text(d, "noIdentitas")},
			{"Jenis Kelamin/NPWP", text(d, "jenisKelaminKet") + " / " + text(d, "npwp")},
			{"Tempat / Tgl Lahir", text(d, "tempatLahir") + " / " + dateID(text(d, "tanggalLahir"))},
			{"Pelapor / Tanggal Update", text(d, "pelaporKet") + " / " + dateID(text(d, "tanggalUpdate"))},
		}, colNavy, nil)
		p.labelRow([]cell{
			{"Alamat", text(d, "alamat")}, {"Kelurahan", text(d, "kelurahan")}, {"Kecamatan", text(d, "kecamatan")},
			{"Kabupaten / Kota", text(d, "kabKotaKet")}, {"Kode Pos", text(d, "kodePos")}, {"Negara", text(d, "negaraKet")},
		}, colNavy, &grey)
		p.labelRow([]cell{
			{"Pekerjaan", text(d, "pekerjaanKet")}, {"Tempat Bekerja", text(d, "tempatBekerja")}, {"", ""},
			{"Bidang Usaha", text(d, "bidangUsahaKet")}, {"", ""}, {"Status Gelar Debitur", text(d, "statusGelarDebitur")},
		}, colNavy, &grey)
		p.pdf.Ln(2)
	}
}

func (p *slikPDF) summary(s map[string]any) {
	pdf := p.pdf
	p.title("Ringkasan Fasilitas", colNavy)
	heads := []string{"Fasilitas", "Kredit (dalam IDR)", "Irrevocable L/C (dalam IDR)", "Garansi (dalam IDR)",
		"Fasilitas Lainnya (dalam IDR)", "Total (dalam IDR)", "Kolektabilitas Terburuk / Bulan Data"}
	w := p.width / float64(len(heads))
	worst := text(s, "kualitasTerburuk")
	if m := monthEN(text(s, "kualitasBulanDataTerburuk")); m != "" {
		worst += " / " + m
	}
	// Fasilitas Lainnya is Sec, as legacy; Lain is not shown (TBD-045c, agreed).
	rows := [][]string{
		heads,
		{"Plafon Efektif", amount(text(s, "plafonEfektifKreditPembiayaan")), amount(text(s, "plafonEfektifLc")), amount(text(s, "plafonEfektifGyd")),
			amount(text(s, "plafonEfektifSec")), amount(text(s, "plafonEfektifTotal")), worst},
		{"Baki Debet", amount(text(s, "bakiDebetKreditPembiayaan")), amount(text(s, "bakiDebetLc")), amount(text(s, "bakiDebetGyd")),
			amount(text(s, "bakiDebetSec")), amount(text(s, "bakiDebetTotal")), ""},
	}
	for i, row := range rows {
		h := pdfLine * 2
		p.ensure(h)
		y := pdf.GetY()
		for j, v := range row {
			pdf.SetXY(pdfMarginX+w*float64(j), y)
			style, fill := "", false
			if i == 0 || j == 0 {
				style, fill = "B", true
				p.fill(colGrey)
				p.color(colNavy)
			} else {
				p.color(colBlack)
			}
			pdf.SetFont("Helvetica", style, 8)
			align := "R"
			if j == 0 || i == 0 || j == len(row)-1 {
				align = "L"
			}
			n := len(p.lines(v, w))
			pdf.MultiCell(w, h/float64(max(n, 1)), p.tr(v), "1", align, fill)
		}
		pdf.SetXY(pdfMarginX, y+h)
	}
	pdf.Ln(2)
	pdf.SetFont("Helvetica", "B", 8)
	p.color(colNavy)
	pdf.CellFormat(p.width, pdfLine, p.tr("Jumlah Kreditur"), "", 1, "L", false, 0, "")
	grey := colGrey
	p.labelRow([]cell{
		{"Bank Umum", text(s, "krediturBankUmum")}, {"BPR/BPRS", text(s, "krediturBPR/S")},
		{"Lembaga Pembiayaan", text(s, "krediturLp")}, {"Lainnya", text(s, "krediturLainnya")},
	}, colNavy, &grey)
}

// facility is one Kredit/Pembiayaan block (FR-PSAPI-012 Susunan Bagian 5).
func (p *slikPDF) facility(f map[string]any) {
	pdf := p.pdf
	p.ensure(20)
	pdf.Ln(3)
	p.fill(colNavy)
	p.color(colWhite)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.CellFormat(p.width, 6, p.tr("Kredit/Pembiayaan"), "", 1, "L", true, 0, "")
	pdf.Ln(1)
	p.labelRow([]cell{
		{"Pelapor", text(f, "ljk") + "-" + text(f, "ljkKet")}, {"Cabang", text(f, "cabangKet")},
		{"Baki Debet", rupiah(text(f, "bakiDebet"))}, {"Tanggal Update", dateID(text(f, "tanggalUpdate"))},
	}, colNavy, nil)
	p.months(f)
	p.pairs([]cell{
		{"No Rekening", text(f, "noRekening")}, {"Kualitas", text(f, "kualitas")}, // TBD-045b, agreed
		{"Sifat Kredit/Pembiayaan", text(f, "sifatKreditPembiayaanKet")}, {"Jumlah Hari Tunggakan", text(f, "jumlahHariTunggakan")},
		{"Jenis Kredit/Pembiayaan", text(f, "jenisKreditPembiayaanKet")}, {"Nilai Proyek", rupiah(text(f, "nilaiProyek"))},
		{"Akad Kredit/Pembiayaan", text(f, "akadKreditPembiayaanKet")}, {"Plafon Awal", rupiah(text(f, "plafonAwal"))},
		{"Frekuensi Perpanjangan Kredit/Pembiayaan", text(f, "frekPerpjganKreditPembiayaan")}, {"Plafon", rupiah(text(f, "plafon"))},
		{"No Akad Awal", text(f, "noAkadAwal")}, {"Realisasi/Pencairan Bulan Berjalan", rupiah(text(f, "realisasiBulanBerjalan"))},
		{"Tanggal Akad Awal", dateID(text(f, "tanggalAkadAwal"))}, {"Nilai Dalam Mata Uang Asal", rupiah(text(f, "nilaiDalamMataUangAsal"))},
		{"No Akad Akhir", text(f, "noAkadAkhir")}, {"Sebab Macet", text(f, "sebabMacetKet")},
		{"Tanggal Akad Akhir", dateID(text(f, "tanggalAkadAkhir"))}, {"Tanggal Macet", dateID(text(f, "tanggalMacet"))},
		{"Tanggal Awal Kredit", dateID(text(f, "tanggalAwalKredit"))}, {"Tunggakan Pokok", rupiah(text(f, "tunggakanPokok"))},
		{"Tanggal Mulai", dateID(text(f, "tanggalMulai"))}, {"Tunggakan Bunga", rupiah(text(f, "tunggakanBunga"))},
		{"Tanggal Jatuh Tempo", dateID(text(f, "tanggalJatuhTempo"))}, {"Frekuensi Tunggakan", text(f, "frekuensiTunggakan")},
		{"Kategori Debitur", text(f, "kategoriDebiturKet")}, {"Denda", rupiah(text(f, "denda"))},
		{"Jenis Penggunaan", text(f, "jenisPenggunaanKet")}, {"Frekuensi Restrukturisasi", text(f, "frekuensiRestrukturisasi")},
		{"Sektor Ekonomi", text(f, "sektorEkonomiKet")}, {"Tanggal Restrukturisasi Akhir", dateID(text(f, "tanggalRestrukturisasiAkhir"))},
		{"Kredit Program Pemerintah", text(f, "kreditProgramPemerintahKet")}, {"Cara Restrukturisasi", text(f, "restrukturisasiKet")},
		{"Kab/Kota Lokasi Proyek", text(f, "lokasiProyekKet")}, {"Kondisi", text(f, "kondisiKet")},
		{"Valuta", text(f, "valutaKode")}, {"Tanggal Kondisi", dateID(text(f, "tanggalKondisi"))},
		{"Suku Bunga/Imbalan", percent(text(f, "sukuBungaImbalan"))}, {"Jenis Suku Bunga Imbalan", text(f, "jenisSukuBungaImbalanKet")},
		{"Keterangan", orDash(text(f, "keterangan"))}, {"", ""},
	})
	p.collaterals(arr(f, "agunan"))
	p.guarantors(arr(f, "penjamin"))
}

func percent(s string) string {
	if s == "" {
		return ""
	}
	return s + "%"
}

// months is the 24-month grid: months 01-12, then 13-24, each a bold
// "MMM yy" label over the Collectibility and Days Past Due boxes; 0 and 0
// are written white (FR-PSAPI-012 Grid 24 Bulan).
func (p *slikPDF) months(f map[string]any) {
	pdf := p.pdf
	labelW := 40.0 // fits "Kolektabilitas/Jumlah" on one line
	w := (p.width - labelW) / 12
	box := w / 2
	rowH := pdfLine * 2
	p.ensure(2*rowH + 2)
	top := pdf.GetY()
	pdf.SetFont("Helvetica", "B", 8)
	p.color(colNavy)
	pdf.SetXY(pdfMarginX, top)
	pdf.MultiCell(labelW, pdfLine, p.tr("Kolektabilitas/Jumlah Hari Tunggakan"), "", "L", false)
	for row := 0; row < 2; row++ {
		y := top + rowH*float64(row)
		for i := 0; i < 12; i++ {
			n := row*12 + i + 1
			x := pdfMarginX + labelW + w*float64(i)
			label := ""
			if t, ok := monthOf(text(f, fmt.Sprintf("tahunBulan%02d", n))); ok {
				label = t.Format("Jan 06")
			}
			kol, ht := text(f, fmt.Sprintf("tahunBulan%02dKol", n)), text(f, fmt.Sprintf("tahunBulan%02dHt", n))
			pdf.SetXY(x, y)
			pdf.SetFont("Helvetica", "B", 8)
			p.color(colBlack)
			pdf.CellFormat(w, pdfLine, label, "", 0, "C", false, 0, "")
			value := colBlack
			if kol == "0" && ht == "0" {
				value = colWhite
			}
			pdf.SetFont("Helvetica", "", 8)
			p.color(value)
			pdf.SetXY(x, y+pdfLine)
			pdf.CellFormat(box, pdfLine, kol, "1", 0, "C", false, 0, "")
			pdf.CellFormat(box, pdfLine, ht, "1", 0, "C", false, 0, "")
		}
	}
	pdf.SetXY(pdfMarginX, top+2*rowH+2)
}

func (p *slikPDF) collaterals(items []map[string]any) {
	p.title("Agunan", colBlack)
	for _, a := range items {
		// Collateral values and dates as the file has them (legacy).
		p.labelRow([]cell{
			{"Jenis Agunan", text(a, "jenisAgunanKet")}, {"Nilai Agunan", text(a, "nilaiAgunanMenurutLJK")},
			{"Paripasu", text(a, "prosentaseParipasu")}, {"Tanggal Update", text(a, "tanggalUpdate")},
		}, colNavy, nil)
		p.pairs([]cell{
			{"Nomor Agunan", text(a, "nomorAgunan")}, {"Peringkat Agunan", text(a, "peringkatAgunan")},
			{"Jenis Pengikatan", text(a, "jenisPengikatanKet")}, {"Lembaga Pemeringkat", text(a, "lembagaPemeringkat")},
			{"Tanggal Pengikatan", text(a, "tanggalPengikatan")}, {"Bukti Kepemilikan", text(a, "buktiKepemilikan")},
			{"Nama Pemilik Agunan", text(a, "namaPemilikAgunan")}, {"Nilai Agunan (NJOP)/Nilai Wajar", text(a, "nilaiAgunanNjop")},
			{"Alamat Agunan", text(a, "alamatAgunan")}, {"Nilai Agunan Penilai Independen", text(a, "nilaiAgunanIndep")},
			{"Dati II Lokasi Agunan", text(a, "kabKotaLokasiAgunanKet")}, {"Nama Penilai Independen", text(a, "namaPenilaiIndep")},
			{"Tanggal Penilaian LJK", text(a, "tglPenilaianPelapor")}, {"Asuransi", text(a, "asuransi")},
			{"Keterangan", orDash(text(a, "keterangan"))}, {"Tgl Penilaian Penilai Independen", text(a, "tanggalPenilaianPenilaiIndependen")},
		})
		p.pdf.Ln(2)
	}
}

// guarantors: field names as the legacy script (TBD-045f, agreed).
func (p *slikPDF) guarantors(items []map[string]any) {
	p.title("Penjamin", colBlack)
	for _, g := range items {
		p.labelRow([]cell{
			{"Nama Penjamin", text(g, "namaPenjamin")}, {"Nomor Identitas Penjamin", text(g, "nomorIdentitas")},
			{"Tanggal Update", dateID(text(g, "tanggalUpdate"))},
		}, colNavy, nil)
		grey := colLightGrey
		p.labelRow([]cell{
			{"Golongan", text(g, "keteranganJenisPenjamin")}, {"Alamat", text(g, "alamatPenjamin")},
			{"Keterangan", orDash(text(g, "keterangan"))},
		}, colNavy, &grey)
	}
}

func (p *slikPDF) closing(header, individual map[string]any) {
	pdf := p.pdf
	p.ensure(30)
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "I", 8)
	p.color(colBlack)
	for _, l := range []string{
		"Nomor Laporan " + text(individual, "nomorLaporan") + " / Operator " + text(header, "dibuatOleh") +
			" / Tanggal Permintaan " + dateID(text(individual, "tanggalPermintaan")) + " / Kode Ref. Pengguna " + text(header, "kodeReferensiPengguna"),
		// Tanggal Dibentuk is header.tanggalHasil (TBD-045d, agreed).
		"Posisi Data Terakhir " + monthID(text(individual, "posisiDataTerakhir")) + " / LJK " + text(header, "kodeLJKPermintaan") + "-" +
			text(header, "kodeCabangPermintaan") + " / Peruntukan " + text(header, "kodeTujuanPermintaan") + " / Tanggal Dibentuk " + dateID(text(header, "tanggalHasil")),
	} {
		pdf.MultiCell(p.width, pdfLine, p.tr(l), "", "L", false)
	}
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "B", 15)
	p.color(colRed)
	pdf.CellFormat(p.width, 8, p.tr("Disclaimer: PDF BERIKUT MERUPAKAN DUPLIKASI DARI SLIK OJK"), "", 1, "C", false, 0, "")
}
