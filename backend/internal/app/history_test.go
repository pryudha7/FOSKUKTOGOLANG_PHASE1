package app_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"fos/internal/apptest"
)

type historyField struct {
	Label, Value string
	Indent       bool
}

type historyProcess struct {
	Name           string
	Rejected       bool
	RejectedOnly   bool
	Request        []historyField
	Response       []historyField
	Errors         []string
	Results        [][]historyField
	DocumentID     string
	DocumentAction string
}

type historyCard struct {
	Logs []struct {
		Name, SubmittedAt, Status, Request, Response string
	}
	TruncateChars, TruncateLines int
	Processes                    []historyProcess
}

// reviewAssignment is the case's first Pre Screening assignment and the
// Team Lead who holds it.
func reviewAssignment(t *testing.T, h *apptest.Harness, caseID string) (*apptest.Client, int64) {
	t.Helper()
	for _, a := range assignments(t, h, caseID) {
		if a.Name == "Pre Screening" {
			return relogin(t, h, "TL01"), a.ID
		}
	}
	t.Fatal("no Pre Screening assignment")
	return nil, 0
}

func history(t *testing.T, c *apptest.Client, caseID string, id any) (historyCard, apptest.Response) {
	t.Helper()
	r := c.Do(t, http.MethodGet, submitPath(caseID, id, "history"), nil)
	var card historyCard
	if r.Status == http.StatusOK {
		if err := json.Unmarshal([]byte(r.Raw), &card); err != nil {
			t.Fatalf("history: %v %s", err, r.Raw)
		}
	}
	return card, r
}

func process(t *testing.T, card historyCard, name string) historyProcess {
	t.Helper()
	for _, p := range card.Processes {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no %s in %+v", name, card.Processes)
	return historyProcess{}
}

func fieldValues(fs []historyField) string {
	var out []string
	for _, f := range fs {
		prefix := ""
		if f.Indent {
			prefix = "  "
		}
		out = append(out, prefix+f.Label+"="+f.Value)
	}
	return strings.Join(out, "|")
}

func processNames(card historyCard) string {
	var out []string
	for _, p := range card.Processes {
		out = append(out, p.Name)
	}
	return strings.Join(out, ",")
}

// AC-UI-020 (agreed for ticket 14): API Log History
// lists every row of the case, newest first, with Waktu Connect dd/MM/yy
// HH:mm and the cut limits from Dynamic System Settings; Pre-Screening API
// Data shows only processes that ran, in order: Dukcapil's request and
// response fields, DHN's identity, code, fixed message and blacklisted.
func TestHistoryDetailsAfterDukcapilAndDHN(t *testing.T) {
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, nil)
	waitRunEnd(t, h, caseID)
	tl, id := reviewAssignment(t, h, caseID)
	card, r := history(t, tl, caseID, id)
	if r.Status != http.StatusOK || card.TruncateChars != 200 || card.TruncateLines != 5 {
		t.Fatalf("history = %d %s", r.Status, r.Raw)
	}
	if len(card.Logs) < 2 || card.Logs[len(card.Logs)-1].Name != "Dukcapil" || len(card.Logs[0].SubmittedAt) != len("13/10/26 10:09") {
		t.Fatalf("logs = %+v", card.Logs)
	}
	if got := processNames(card); got != "Dukcapil,DHN" {
		t.Fatalf("processes = %s", got)
	}
	d := process(t, card, "Dukcapil")
	if got := fieldValues(d.Request); got != "NIK=3171011234567890|Nama Lengkap=Budi Santoso|Tanggal Lahir="+dmy(validPersonal(h).BirthDate) {
		t.Fatalf("dukcapil request = %s", got)
	}
	if got := fieldValues(d.Response); got != "Response NIK=Sesuai (100)|Response Nama Lengkap=Tidak Sesuai (0)|Response Tanggal Lahir=Sesuai" || d.Rejected {
		t.Fatalf("dukcapil response = %s", got)
	}
	dhn := process(t, card, "DHN")
	if got := fieldValues(dhn.Request) + "#" + fieldValues(dhn.Response); got != "Identity Number=3171011234567890#Code=DN-00|Message=Pengecekan DHN tidak dilakukan, mohon pastikan cabang memberikan bukti DHN|Blacklisted=false" {
		t.Fatalf("dhn = %s", got)
	}
	if _, r := history(t, relogin(t, h, "AO001"), caseID, id); r.Status != http.StatusForbidden {
		t.Fatalf("another operator = %d", r.Status)
	}
}

// AC-FR-PSAPI-005-02, -03, -04: a Dukcapil response without the contract's
// fields shows "--"; a reject shows the banner, with NAMA_LGKP as answered.
func TestHistoryDetailsDukcapilShapes(t *testing.T) {
	h, _, _, caseID := checksWith(t, []apptest.DukcapilReply{apptest.DukcapilNotFound}, dhnOK, nil)
	waitRunEnd(t, h, caseID)
	tl, id := reviewAssignment(t, h, caseID)
	card, _ := history(t, tl, caseID, id)
	if got := fieldValues(process(t, card, "Dukcapil").Response); got != "Response NIK=--|Response Nama Lengkap=--|Response Tanggal Lahir=--" {
		t.Fatalf("response = %s", got)
	}

	h2, _, _, other := checksWith(t, []apptest.DukcapilReply{apptest.DukcapilMismatch}, dhnOK, nil)
	waitRunEnd(t, h2, other)
	tl2, id2 := reviewAssignment(t, h2, other)
	card2, _ := history(t, tl2, other, id2)
	if p := process(t, card2, "Dukcapil"); !p.Rejected || processNames(card2) != "Dukcapil" ||
		!strings.Contains(fieldValues(p.Response), "Response Nama Lengkap=Sesuai (100)") {
		t.Fatalf("rejected dukcapil = %+v / %s", p, processNames(card2))
	}
}

// AC-FR-PSAPI-005-01, AC-FR-PSAPI-010-05: SLIK requested twice shows only
// its latest request, codes as BMV descriptions; View SLIK points at the
// SLIK PDF once there is one; the SLIKPBSResponse rows stay out.
func TestHistoryDetailsSLIK(t *testing.T) {
	h, _, caseID := slikCase(t, false, apptest.SLIKInvalid, apptest.SLIKAccepted)
	tl, id := reviewAssignment(t, h, caseID)
	card, _ := history(t, tl, caseID, id)
	s := process(t, card, "SLIK")
	kode := referenceCode(t, h, caseID, "CUSTOMER")
	if got := fieldValues(s.Request); !strings.HasPrefix(got, "Kode Reff="+kode+"|Tujuan Permintaan=") || strings.Contains(got, "=TP-1") ||
		fieldValues(s.Response) != "Code=1100|Message=success" || s.DocumentID != "" {
		t.Fatalf("slik = %s / %s / %q", got, fieldValues(s.Response), s.DocumentID)
	}
	callPBS(t, h, kode, slikZip(t, kode, "3171011234567890", facility("1")))
	card, _ = history(t, tl, caseID, id)
	pdf, _ := storedDocument(t, h, caseID, "PreScreeningResponse", "SLIK_PDF_CUSTOMER")
	if s := process(t, card, "SLIK"); s.DocumentID != pdf.FileID || s.DocumentAction != "View SLIK" || strings.Contains(processNames(card), "PBS") {
		t.Fatalf("after the result: %+v / %s", s, processNames(card))
	}
}

// FR-PSAPI-005 rule 10: a failing SLIK request shows its errors, one each.
func TestHistoryDetailsSLIKErrors(t *testing.T) {
	fs := apptest.StartFakeAPI(t, apptest.SLIKInvalid)
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) { h.UseSLIK(t, fs) })
	eventually(t, "SLIK to fail", func() bool { return run(t, h, caseID).State == "STOPPED_FAILED" })
	tl, id := reviewAssignment(t, h, caseID)
	card, _ := history(t, tl, caseID, id)
	s := process(t, card, "SLIK")
	if fieldValues(s.Response) != "Code=4000|Message=request invalid" || strings.Join(s.Errors, "|") != "pilihan fasilitas hanya boleh Seluruh atau Outstanding" {
		t.Fatalf("slik = %+v", s)
	}
}

// AC-FR-PSAPI-014-06, AC-FR-PSAPI-017-08, AC-FR-PSAPI-018-06: Pefindo and
// PefindoPasangan show their Search's fields, results highest Similarity
// Score first, Download PDF when the PDF exists, and the banner when any of
// the subject's Pefindo APIs rejected; the other Pefindo APIs are not
// processes of their own.
func TestHistoryDetailsPefindo(t *testing.T) {
	scripts := pefindoPassScripts()
	scripts[apptest.PefindoReport] = []apptest.FakeReply{reportReady("01", scoring("2025-07-15", "C2")), reportReady("01", scoring("2025-07-15", "E3"))}
	h, _, caseID := pefindoIndividual(t, true, scripts)
	tl, id := reviewAssignment(t, h, caseID)
	card, _ := history(t, tl, caseID, id)
	if got := processNames(card); got != "Dukcapil,DHN,SLIK,SLIKPasangan,Pefindo,PefindoPasangan" {
		t.Fatalf("processes = %s", got)
	}
	p := process(t, card, "Pefindo")
	ref := customerCode(t, h, caseID)
	if got := fieldValues(p.Request); got != "Type=PERSONAL|Product ID=1|Inquiry Reason=12|Reference Code="+ref+
		"|Params=|  ID Type=KTP|  ID No=3171011234567890|  Name=Budi Santoso|  Date Of Birth="+validPersonal(h).BirthDate+"|  Report Date=--" {
		t.Fatalf("pefindo request = %s", got)
	}
	if got := fieldValues(p.Response); got != "Code=01|API Status=Success|Message=Data ditemukan|Inquiry ID=1436899" {
		t.Fatalf("pefindo response = %s", got)
	}
	if len(p.Results) != 2 || p.Results[0][0].Value != "100" || p.Results[0][1].Value != "101100684000004" || p.Results[1][0].Value != "80" || p.Results[0][4].Value != "BUDI" {
		t.Fatalf("results = %+v", p.Results)
	}
	pdf, _ := storedDocument(t, h, caseID, "PreScreeningResponse", "PEFINDO_PDF_CUSTOMER")
	if p.DocumentID != pdf.FileID || p.DocumentAction != "Download PDF" || p.Rejected {
		t.Fatalf("pefindo = %+v", p)
	}
	if sp := process(t, card, "PefindoPasangan"); !sp.Rejected {
		t.Fatalf("spouse not rejected: %+v", sp)
	}
}

// FR-PSAPI-005 rule 6: Rejected List shows only after it rejected the case,
// as the banner alone.
func TestHistoryDetailsRejectedList(t *testing.T) {
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, nil)
	waitRunEnd(t, h, caseID)
	tl, id := reviewAssignment(t, h, caseID)
	if card, _ := history(t, tl, caseID, id); strings.Contains(processNames(card), "Rejected List") {
		t.Fatal("Rejected List shown without a reject")
	}
	exec(t, h, `UPDATE fos.prescreening_runs SET state = 'STOPPED_REJECTED', current_step = 'Rejected List' WHERE case_id = $1`, caseID)
	card, _ := history(t, tl, caseID, id)
	if p := process(t, card, "Rejected List"); !p.Rejected || !p.RejectedOnly || len(p.Request)+len(p.Response) != 0 || processNames(card) != "Dukcapil,DHN,Rejected List" {
		t.Fatalf("rejected list = %+v / %s", p, processNames(card))
	}
}
