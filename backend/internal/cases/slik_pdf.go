package cases

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// The SLIK report PDF "Informasi Debitur" (FR-PSAPI-010 Laporan PDF,
// FR-PSAPI-012): one per received SLIK file, laid out as the legacy script
// did, from the same JSON the result's tables keep (summary_json,
// detail_json). Helvetica stands for Tahoma; a table row or a grid row is
// never split across pages (agreed for TBD-045).

// Page and text sizes (FR-PSAPI-012 Halaman dan Gaya); millimetres.
const (
	pdfMarginX, pdfMarginY = 15.0, 10.0
	pdfLine                = 4.2 // one line of 8 pt text
	// pdfMaxLines bounds one value so a row always fits a page; a longer
	// value ends in "..." (agreed for the ticket 34 review).
	pdfMaxLines = 40
)

type pdfRGB struct{ r, g, b int }

var (
	colRed       = pdfRGB{255, 0, 0}
	colNavy      = pdfRGB{0, 38, 87}
	colGrey      = pdfRGB{217, 217, 217}
	colLightGrey = pdfRGB{240, 240, 240}
	colBanner    = pdfRGB{213, 215, 225}
	colWhite     = pdfRGB{255, 255, 255}
	colBlack     = pdfRGB{0, 0, 0}
)

// pdfStyle is a font style, size and text colour.
type pdfStyle struct {
	font  string // "", "B", "I", "IU", ...
	size  float64
	color pdfRGB
}

var (
	styleLabel = pdfStyle{"B", 8, colNavy}
	styleValue = pdfStyle{"", 8, colBlack}
)

// monthsIndonesian are the month names of the report's dates (FR-PSAPI-012
// Format Nilai).
var monthsIndonesian = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

// slikPDF draws one report.
type slikPDF struct {
	pdf   *fpdf.Fpdf
	tr    func(string) string
	width float64 // printable width
}

// pdfCell is one label and its value.
type pdfCell struct{ label, value string }

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

// pdfValue is a file value as shown: its text, or "".
func pdfValue(m map[string]any, key string) string {
	if v := field(m, key); v != nil {
		return *v
	}
	return ""
}

// indonesianDate is yyyyMMdd[HHmmss] as "9 Mei 2023"; "" when it is not a date.
func indonesianDate(s string) string {
	if len(s) < 8 {
		return ""
	}
	t, err := time.Parse("20060102", s[:8])
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d %s %d", t.Day(), monthsIndonesian[t.Month()-1], t.Year())
}

// dataMonth reads yyyyMM.
func dataMonth(s string) (time.Time, bool) {
	if len(s) < 6 {
		return time.Time{}, false
	}
	t, err := time.Parse("200601", s[:6])
	return t, err == nil
}

// indonesianMonth is yyyyMM as "September 2024"; englishMonth as
// "February 2023" (legacy English, FR-PSAPI-012).
func indonesianMonth(s string) string {
	t, ok := dataMonth(s)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s %d", monthsIndonesian[t.Month()-1], t.Year())
}

func englishMonth(s string) string {
	t, ok := dataMonth(s)
	if !ok {
		return ""
	}
	return t.Format("January 2006")
}

// pdfAmount is a decimal with thousands commas and two decimals
// (1,103,713,259.00); "" when it is not a number.
func pdfAmount(s string) string {
	_, fixed, ok := decimal2(s)
	if !ok {
		return ""
	}
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

// pdfRupiah is an amount of a facility block: "Rp 485,000,000.00".
func pdfRupiah(s string) string {
	if a := pdfAmount(s); a != "" {
		return "Rp " + a
	}
	return ""
}

func pdfPercent(s string) string {
	if s == "" {
		return ""
	}
	return s + "%"
}

// pdfOrDash shows "-" for an empty Keterangan (FR-PSAPI-012).
func pdfOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// use sets a style for what is drawn next.
func (p *slikPDF) use(s pdfStyle) {
	p.pdf.SetFont("Helvetica", s.font, s.size)
	p.pdf.SetTextColor(s.color.r, s.color.g, s.color.b)
}

func (p *slikPDF) fill(c pdfRGB) { p.pdf.SetFillColor(c.r, c.g, c.b) }

// ensure starts a new page when h millimetres do not fit.
func (p *slikPDF) ensure(h float64) {
	_, pageH := p.pdf.GetPageSize()
	if p.pdf.GetY()+h > pageH-pdfMarginY {
		p.pdf.AddPage()
	}
}

// lines splits a value into the lines it takes in width w in style s, at
// most pdfMaxLines.
func (p *slikPDF) lines(v string, w float64, s pdfStyle) []string {
	if v == "" {
		return []string{""}
	}
	p.use(s)
	out := p.pdf.SplitText(p.tr(v), w-2)
	if len(out) > pdfMaxLines {
		out = append(out[:pdfMaxLines-1], strings.TrimRight(out[pdfMaxLines-1], " ")+"...")
	}
	return out
}

// draw writes lines from (x, y), one per pdfLine.
func (p *slikPDF) draw(x, y, w float64, lines []string, s pdfStyle, align string) {
	p.use(s)
	for i, l := range lines {
		p.pdf.SetXY(x, y+pdfLine*float64(i))
		p.pdf.CellFormat(w, pdfLine, l, "", 0, align, false, 0, "")
	}
}

// title is a section title: 14 pt, underlined in lineColor.
func (p *slikPDF) title(s string, textColor, lineColor pdfRGB) {
	p.ensure(12)
	p.pdf.Ln(3)
	p.use(pdfStyle{"B", 14, textColor})
	text := p.tr(s)
	x, y := p.pdf.GetX(), p.pdf.GetY()
	p.pdf.CellFormat(p.width, 7, text, "", 1, "L", false, 0, "")
	p.pdf.SetDrawColor(lineColor.r, lineColor.g, lineColor.b)
	p.pdf.SetLineWidth(0.4)
	p.pdf.Line(x, y+6.5, x+p.pdf.GetStringWidth(text), y+6.5)
	p.pdf.SetDrawColor(0, 0, 0)
	p.pdf.SetLineWidth(0.2)
	p.pdf.Ln(1)
}

// labelRow draws cells side by side from x across width: a label line
// (filled when fill is set) above the value lines.
func (p *slikPDF) labelRow(cells []pdfCell, fill *pdfRGB, x, width float64) {
	w := width / float64(len(cells))
	n := 1
	for _, c := range cells {
		n = max(n, len(p.lines(c.value, w, styleValue)))
	}
	p.ensure(pdfLine * float64(n+1))
	y := p.pdf.GetY()
	for i, c := range cells {
		cx := x + w*float64(i)
		p.use(styleLabel)
		if fill != nil {
			p.fill(*fill)
		}
		p.pdf.SetXY(cx, y)
		p.pdf.CellFormat(w, pdfLine, p.tr(c.label), "", 0, "L", fill != nil, 0, "")
		p.draw(cx, y+pdfLine, w, p.lines(c.value, w, styleValue), styleValue, "L")
	}
	p.pdf.SetXY(pdfMarginX, y+pdfLine*float64(n+1)+1)
}

// row draws a full-width labelRow.
func (p *slikPDF) row(cells []pdfCell, fill *pdfRGB) { p.labelRow(cells, fill, pdfMarginX, p.width) }

// pairs draws label/value pairs, two per line in bordered cells, labels
// filled (the detail tables of a facility and of a collateral).
func (p *slikPDF) pairs(items []pdfCell) {
	lw, vw := p.width*0.2, p.width*0.3
	for i := 0; i < len(items); i += 2 {
		row := items[i:min(i+2, len(items))]
		n := 1
		for _, c := range row {
			n = max(n, len(p.lines(c.label, lw, styleLabel)), len(p.lines(c.value, vw, styleValue)))
		}
		h := pdfLine * float64(n)
		p.ensure(h)
		x, y := pdfMarginX, p.pdf.GetY()
		for _, c := range row {
			p.fill(colLightGrey)
			p.pdf.Rect(x, y, lw, h, "FD")
			p.pdf.Rect(x+lw, y, vw, h, "D")
			p.draw(x, y, lw, p.lines(c.label, lw, styleLabel), styleLabel, "L")
			p.draw(x+lw, y, vw, p.lines(c.value, vw, styleValue), styleValue, "L")
			x += lw + vw
		}
		p.pdf.SetXY(pdfMarginX, y+h)
	}
}

func (p *slikPDF) head() {
	pdf := p.pdf
	p.use(pdfStyle{"B", 17.5, colRed})
	pdf.CellFormat(p.width, 8, p.tr("Informasi Debitur"), "", 1, "L", false, 0, "")
	p.use(pdfStyle{"", 8, colNavy})
	pdf.CellFormat(p.width, pdfLine, p.tr("Sistem Layanan Informasi Keuangan"), "", 1, "L", false, 0, "")
	pdf.Ln(2)
	boxW := 30.0
	y := pdf.GetY()
	p.fill(colBanner)
	p.use(pdfStyle{"I", 8, colBlack})
	pdf.MultiCell(p.width-boxW, pdfLine, p.tr("Informasi ini bersifat RAHASIA dan hanya digunakan untuk kepentingan pemohon informasi. Akibat yang timbul dari penggunaan informasi ini bukan merupakan tanggung jawab Otoritas Jasa Keuangan"), "", "L", true)
	h := pdf.GetY() - y
	pdf.SetXY(pdfMarginX+p.width-boxW, y)
	p.fill(colRed)
	p.use(pdfStyle{"B", 10, colWhite})
	pdf.CellFormat(boxW, h, "RAHASIA", "", 1, "C", true, 0, "")
	pdf.Ln(3)
}

func (p *slikPDF) keywords(header, individual map[string]any) {
	pdf := p.pdf
	search := obj(individual, "parameterPencarian")
	italic := pdfStyle{"I", 8, colBlack}
	p.use(italic)
	pdf.Write(pdfLine, p.tr("Informasi diberikan berdasarkan laporan yang dikirimkan oleh pelapor ke dalam Sistem Layanan Informasi Keuangan dengan "))
	p.use(pdfStyle{"IU", 8, colRed})
	pdf.Write(pdfLine, p.tr("kata kunci pencarian"))
	p.use(italic)
	pdf.Write(pdfLine, p.tr(" sebagai berikut:"))
	pdf.Ln(pdfLine + 2)

	// Left two thirds: the search parameters; right third: the request.
	left, right := p.width*2/3, p.width/3
	y := pdf.GetY()
	p.labelRow([]pdfCell{{"Nama", pdfValue(search, "namaDebitur")}, {"Jenis Kelamin", pdfValue(search, "jenisKelaminKet")}}, nil, pdfMarginX, left)
	p.labelRow([]pdfCell{{"No. Identitas", pdfValue(search, "noIdentitas")}, {"NPWP", pdfValue(search, "npwp")},
		{"Tempat Lahir", pdfValue(search, "tempatLahir")}, {"Tanggal Lahir", indonesianDate(pdfValue(search, "tanggalLahir"))}}, nil, pdfMarginX, left)
	bottom := pdf.GetY()
	pdf.SetY(y)
	for _, c := range []pdfCell{
		{"Kode Ref. Pengguna", pdfValue(header, "kodeReferensiPengguna")},
		{"Nomor Laporan", pdfValue(individual, "nomorLaporan")},
		{"Posisi Data Terakhir", indonesianMonth(pdfValue(individual, "posisiDataTerakhir"))},
		{"Tanggal Permintaan", indonesianDate(pdfValue(individual, "tanggalPermintaan"))},
	} {
		pdf.SetX(pdfMarginX + left)
		p.use(styleLabel)
		pdf.CellFormat(right*0.45, pdfLine, p.tr(c.label), "", 0, "L", false, 0, "")
		p.use(styleValue)
		pdf.CellFormat(right*0.55, pdfLine, p.tr(c.value), "", 1, "L", false, 0, "")
	}
	pdf.SetXY(pdfMarginX, max(bottom, pdf.GetY()))
}

func (p *slikPDF) debtors(items []map[string]any) {
	p.title("Data Pokok Debitur", colNavy, colNavy)
	p.use(styleValue)
	p.pdf.MultiCell(p.width, pdfLine, p.tr("Penyajian informasi debitur pada Sistem Layanan Informasi Keuangan dikelompokkan berdasarkan nomor identitas debitur. Pengguna informasi diharapkan dapat meneliti kembali kemungkinan adanya debitur berbeda yang dilaporkan menggunakan nomor identitas yang sama"), "", "L", false)
	p.pdf.Ln(2)
	if len(items) == 0 {
		p.use(pdfStyle{"B", 20, colNavy})
		p.pdf.CellFormat(p.width, 12, p.tr("Data Tidak Ditemukan"), "", 1, "C", false, 0, "")
		return
	}
	grey := colGrey
	for _, d := range items {
		p.row([]pdfCell{
			{"Nama Sesuai Identitas", pdfValue(d, "namaDebitur")},
			{"Identitas", "NIK / " + pdfValue(d, "noIdentitas")},
			{"Jenis Kelamin/NPWP", pdfValue(d, "jenisKelaminKet") + " / " + pdfValue(d, "npwp")},
			{"Tempat / Tgl Lahir", pdfValue(d, "tempatLahir") + " / " + indonesianDate(pdfValue(d, "tanggalLahir"))},
			{"Pelapor / Tanggal Update", pdfValue(d, "pelaporKet") + " / " + indonesianDate(pdfValue(d, "tanggalUpdate"))},
		}, nil)
		p.row([]pdfCell{
			{"Alamat", pdfValue(d, "alamat")}, {"Kelurahan", pdfValue(d, "kelurahan")}, {"Kecamatan", pdfValue(d, "kecamatan")},
			{"Kabupaten / Kota", pdfValue(d, "kabKotaKet")}, {"Kode Pos", pdfValue(d, "kodePos")}, {"Negara", pdfValue(d, "negaraKet")},
		}, &grey)
		p.row([]pdfCell{
			{"Pekerjaan", pdfValue(d, "pekerjaanKet")}, {"Tempat Bekerja", pdfValue(d, "tempatBekerja")}, {"", ""},
			{"Bidang Usaha", pdfValue(d, "bidangUsahaKet")}, {"", ""}, {"Status Gelar Debitur", pdfValue(d, "statusGelarDebitur")},
		}, &grey)
		p.pdf.Ln(2)
	}
}

func (p *slikPDF) summary(s map[string]any) {
	pdf := p.pdf
	p.title("Ringkasan Fasilitas", colNavy, colNavy)
	heads := []string{"Fasilitas", "Kredit (dalam IDR)", "Irrevocable L/C (dalam IDR)", "Garansi (dalam IDR)",
		"Fasilitas Lainnya (dalam IDR)", "Total (dalam IDR)", "Kolektabilitas Terburuk / Bulan Data"}
	w := p.width / float64(len(heads))
	worst := pdfValue(s, "kualitasTerburuk")
	if m := englishMonth(pdfValue(s, "kualitasBulanDataTerburuk")); m != "" {
		worst += " / " + m
	}
	// Fasilitas Lainnya is Sec, as legacy; Lain is not shown (TBD-045c, agreed).
	rows := [][]string{
		heads,
		{"Plafon Efektif", pdfAmount(pdfValue(s, "plafonEfektifKreditPembiayaan")), pdfAmount(pdfValue(s, "plafonEfektifLc")),
			pdfAmount(pdfValue(s, "plafonEfektifGyd")), pdfAmount(pdfValue(s, "plafonEfektifSec")), pdfAmount(pdfValue(s, "plafonEfektifTotal")), worst},
		{"Baki Debet", pdfAmount(pdfValue(s, "bakiDebetKreditPembiayaan")), pdfAmount(pdfValue(s, "bakiDebetLc")),
			pdfAmount(pdfValue(s, "bakiDebetGyd")), pdfAmount(pdfValue(s, "bakiDebetSec")), pdfAmount(pdfValue(s, "bakiDebetTotal")), ""},
	}
	for i, row := range rows {
		styleOf := func(j int) pdfStyle {
			if i == 0 || j == 0 {
				return styleLabel
			}
			return styleValue
		}
		// The row is as tall as its tallest cell, two lines at least.
		n := 2
		for j, v := range row {
			n = max(n, len(p.lines(v, w, styleOf(j))))
		}
		h := pdfLine * float64(n)
		p.ensure(h)
		y := pdf.GetY()
		for j, v := range row {
			x := pdfMarginX + w*float64(j)
			if i == 0 || j == 0 {
				p.fill(colGrey)
				pdf.Rect(x, y, w, h, "FD")
			} else {
				pdf.Rect(x, y, w, h, "D")
			}
			align := "R"
			if j == 0 || i == 0 || j == len(row)-1 {
				align = "L"
			}
			p.draw(x, y, w, p.lines(v, w, styleOf(j)), styleOf(j), align)
		}
		pdf.SetXY(pdfMarginX, y+h)
	}
	pdf.Ln(2)
	p.use(styleLabel)
	pdf.CellFormat(p.width, pdfLine, p.tr("Jumlah Kreditur"), "", 1, "L", false, 0, "")
	p.boxes([]pdfCell{
		{"Bank Umum", pdfValue(s, "krediturBankUmum")}, {"BPR/BPRS", pdfValue(s, "krediturBPR/S")},
		{"Lembaga Pembiayaan", pdfValue(s, "krediturLp")}, {"Lainnya", pdfValue(s, "krediturLainnya")},
	})
}

// boxes draws cells as bordered boxes side by side, a grey label line above
// the value (Jumlah Kreditur, FR-PSAPI-012 Susunan Bagian 4).
func (p *slikPDF) boxes(cells []pdfCell) {
	const gap = 3.0
	w := (p.width - gap*float64(len(cells)-1)) / float64(len(cells))
	h := pdfLine * 2
	p.ensure(h)
	y := p.pdf.GetY()
	for i, c := range cells {
		x := pdfMarginX + (w+gap)*float64(i)
		p.fill(colGrey)
		p.pdf.Rect(x, y, w, pdfLine, "FD")
		p.pdf.Rect(x, y+pdfLine, w, pdfLine, "D")
		p.draw(x, y, w, []string{p.tr(c.label)}, styleLabel, "L")
		p.draw(x, y+pdfLine, w, []string{p.tr(c.value)}, styleValue, "C")
	}
	p.pdf.SetXY(pdfMarginX, y+h+1)
}

// facility is one Kredit/Pembiayaan block (FR-PSAPI-012 Susunan Bagian 5).
func (p *slikPDF) facility(f map[string]any) {
	pdf := p.pdf
	p.ensure(20)
	pdf.Ln(3)
	p.fill(colNavy)
	p.use(pdfStyle{"B", 9, colWhite})
	pdf.CellFormat(p.width, 6, p.tr("Kredit/Pembiayaan"), "", 1, "L", true, 0, "")
	pdf.Ln(1)
	v := func(key string) string { return pdfValue(f, key) }
	p.row([]pdfCell{
		{"Pelapor", v("ljk") + "-" + v("ljkKet")}, {"Cabang", v("cabangKet")},
		{"Baki Debet", pdfRupiah(v("bakiDebet"))}, {"Tanggal Update", indonesianDate(v("tanggalUpdate"))},
	}, nil)
	p.months(f)
	p.pairs([]pdfCell{
		{"No Rekening", v("noRekening")}, {"Kualitas", v("kualitas")}, // TBD-045b, agreed
		{"Sifat Kredit/Pembiayaan", v("sifatKreditPembiayaanKet")}, {"Jumlah Hari Tunggakan", v("jumlahHariTunggakan")},
		{"Jenis Kredit/Pembiayaan", v("jenisKreditPembiayaanKet")}, {"Nilai Proyek", pdfRupiah(v("nilaiProyek"))},
		{"Akad Kredit/Pembiayaan", v("akadKreditPembiayaanKet")}, {"Plafon Awal", pdfRupiah(v("plafonAwal"))},
		{"Frekuensi Perpanjangan Kredit/Pembiayaan", v("frekPerpjganKreditPembiayaan")}, {"Plafon", pdfRupiah(v("plafon"))},
		{"No Akad Awal", v("noAkadAwal")}, {"Realisasi/Pencairan Bulan Berjalan", pdfRupiah(v("realisasiBulanBerjalan"))},
		{"Tanggal Akad Awal", indonesianDate(v("tanggalAkadAwal"))}, {"Nilai Dalam Mata Uang Asal", pdfRupiah(v("nilaiDalamMataUangAsal"))},
		{"No Akad Akhir", v("noAkadAkhir")}, {"Sebab Macet", v("sebabMacetKet")},
		{"Tanggal Akad Akhir", indonesianDate(v("tanggalAkadAkhir"))}, {"Tanggal Macet", indonesianDate(v("tanggalMacet"))},
		{"Tanggal Awal Kredit", indonesianDate(v("tanggalAwalKredit"))}, {"Tunggakan Pokok", pdfRupiah(v("tunggakanPokok"))},
		{"Tanggal Mulai", indonesianDate(v("tanggalMulai"))}, {"Tunggakan Bunga", pdfRupiah(v("tunggakanBunga"))},
		{"Tanggal Jatuh Tempo", indonesianDate(v("tanggalJatuhTempo"))}, {"Frekuensi Tunggakan", v("frekuensiTunggakan")},
		{"Kategori Debitur", v("kategoriDebiturKet")}, {"Denda", pdfRupiah(v("denda"))},
		{"Jenis Penggunaan", v("jenisPenggunaanKet")}, {"Frekuensi Restrukturisasi", v("frekuensiRestrukturisasi")},
		{"Sektor Ekonomi", v("sektorEkonomiKet")}, {"Tanggal Restrukturisasi Akhir", indonesianDate(v("tanggalRestrukturisasiAkhir"))},
		{"Kredit Program Pemerintah", v("kreditProgramPemerintahKet")}, {"Cara Restrukturisasi", v("restrukturisasiKet")},
		{"Kab/Kota Lokasi Proyek", v("lokasiProyekKet")}, {"Kondisi", v("kondisiKet")},
		{"Valuta", v("valutaKode")}, {"Tanggal Kondisi", indonesianDate(v("tanggalKondisi"))},
		{"Suku Bunga/Imbalan", pdfPercent(v("sukuBungaImbalan"))}, {"Jenis Suku Bunga Imbalan", v("jenisSukuBungaImbalanKet")},
		{"Keterangan", pdfOrDash(v("keterangan"))}, {"", ""},
	})
	p.collaterals(arr(f, "agunan"))
	p.guarantors(arr(f, "penjamin"))
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
	p.use(styleLabel)
	pdf.SetXY(pdfMarginX, top)
	pdf.MultiCell(labelW, pdfLine, p.tr("Kolektabilitas/Jumlah Hari Tunggakan"), "", "L", false)
	for row := 0; row < 2; row++ {
		y := top + rowH*float64(row)
		for i := 0; i < 12; i++ {
			n := row*12 + i + 1
			x := pdfMarginX + labelW + w*float64(i)
			label := ""
			if t, ok := dataMonth(pdfValue(f, fmt.Sprintf("tahunBulan%02d", n))); ok {
				label = t.Format("Jan 06")
			}
			collectibility := pdfValue(f, fmt.Sprintf("tahunBulan%02dKol", n))
			daysPastDue := pdfValue(f, fmt.Sprintf("tahunBulan%02dHt", n))
			p.draw(x, y, w, []string{label}, pdfStyle{"B", 8, colBlack}, "C")
			value := styleValue
			if collectibility == "0" && daysPastDue == "0" {
				value.color = colWhite
			}
			p.use(value)
			pdf.SetXY(x, y+pdfLine)
			pdf.CellFormat(box, pdfLine, collectibility, "1", 0, "C", false, 0, "")
			pdf.CellFormat(box, pdfLine, daysPastDue, "1", 0, "C", false, 0, "")
		}
	}
	pdf.SetXY(pdfMarginX, top+2*rowH+2)
}

func (p *slikPDF) collaterals(items []map[string]any) {
	p.title("Agunan", colBlack, colNavy)
	for _, a := range items {
		v := func(key string) string { return pdfValue(a, key) }
		// Collateral values and dates as the file has them (legacy).
		p.row([]pdfCell{
			{"Jenis Agunan", v("jenisAgunanKet")}, {"Nilai Agunan", v("nilaiAgunanMenurutLJK")},
			{"Paripasu", v("prosentaseParipasu")}, {"Tanggal Update", v("tanggalUpdate")},
		}, nil)
		p.pairs([]pdfCell{
			{"Nomor Agunan", v("nomorAgunan")}, {"Peringkat Agunan", v("peringkatAgunan")},
			{"Jenis Pengikatan", v("jenisPengikatanKet")}, {"Lembaga Pemeringkat", v("lembagaPemeringkat")},
			{"Tanggal Pengikatan", v("tanggalPengikatan")}, {"Bukti Kepemilikan", v("buktiKepemilikan")},
			{"Nama Pemilik Agunan", v("namaPemilikAgunan")}, {"Nilai Agunan (NJOP)/Nilai Wajar", v("nilaiAgunanNjop")},
			{"Alamat Agunan", v("alamatAgunan")}, {"Nilai Agunan Penilai Independen", v("nilaiAgunanIndep")},
			{"Dati II Lokasi Agunan", v("kabKotaLokasiAgunanKet")}, {"Nama Penilai Independen", v("namaPenilaiIndep")},
			{"Tanggal Penilaian LJK", v("tglPenilaianPelapor")}, {"Asuransi", v("asuransi")},
			{"Keterangan", pdfOrDash(v("keterangan"))}, {"Tgl Penilaian Penilai Independen", v("tanggalPenilaianPenilaiIndependen")},
		})
		p.pdf.Ln(2)
	}
}

// guarantors: field names as the legacy script (TBD-045f, agreed).
func (p *slikPDF) guarantors(items []map[string]any) {
	p.title("Penjamin", colBlack, colNavy)
	grey := colLightGrey
	for _, g := range items {
		v := func(key string) string { return pdfValue(g, key) }
		p.row([]pdfCell{
			{"Nama Penjamin", v("namaPenjamin")}, {"Nomor Identitas Penjamin", v("nomorIdentitas")},
			{"Tanggal Update", indonesianDate(v("tanggalUpdate"))},
		}, nil)
		p.row([]pdfCell{
			{"Golongan", v("keteranganJenisPenjamin")}, {"Alamat", v("alamatPenjamin")},
			{"Keterangan", pdfOrDash(v("keterangan"))},
		}, &grey)
	}
}

func (p *slikPDF) closing(header, individual map[string]any) {
	pdf := p.pdf
	p.ensure(30)
	pdf.Ln(4)
	p.use(pdfStyle{"I", 8, colBlack})
	for _, l := range []string{
		"Nomor Laporan " + pdfValue(individual, "nomorLaporan") + " / Operator " + pdfValue(header, "dibuatOleh") +
			" / Tanggal Permintaan " + indonesianDate(pdfValue(individual, "tanggalPermintaan")) + " / Kode Ref. Pengguna " + pdfValue(header, "kodeReferensiPengguna"),
		// Tanggal Dibentuk is header.tanggalHasil (TBD-045d, agreed).
		"Posisi Data Terakhir " + indonesianMonth(pdfValue(individual, "posisiDataTerakhir")) + " / LJK " + pdfValue(header, "kodeLJKPermintaan") + "-" +
			pdfValue(header, "kodeCabangPermintaan") + " / Peruntukan " + pdfValue(header, "kodeTujuanPermintaan") +
			" / Tanggal Dibentuk " + indonesianDate(pdfValue(header, "tanggalHasil")),
	} {
		pdf.MultiCell(p.width, pdfLine, p.tr(l), "", "L", false)
	}
	pdf.Ln(4)
	p.use(pdfStyle{"B", 15, colRed})
	pdf.CellFormat(p.width, 8, p.tr("Disclaimer: PDF BERIKUT MERUPAKAN DUPLIKASI DARI SLIK OJK"), "", 1, "C", false, 0, "")
}
