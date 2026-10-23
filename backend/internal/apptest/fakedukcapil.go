package apptest

import (
	"testing"
)

// DukcapilAPIKey is the key the fake expects in x-Gateway-APIKey.
const DukcapilAPIKey = "dukcapil-test-key"

// DukcapilReply is a FakeReply of the Dukcapil gateway.
type DukcapilReply = FakeReply

// FakeDukcapil is the Dukcapil gateway for tests.
type FakeDukcapil struct{ *FakeAPI }

// DukcapilRequest is what the Dukcapil fake received.
type DukcapilRequest struct {
	Path, APIKey, ContentType, Body string
}

// Dukcapil replies (FR-PSAPI-003).
var (
	DukcapilMatch    = DukcapilReply{Status: 200, Body: `{"NIK":"Sesuai (100)","NAMA_LGKP":"Tidak Sesuai (0)","TGL_LHR":"Sesuai","ALAMAT":"Sesuai (100)"}`}
	DukcapilMismatch = DukcapilReply{Status: 200, Body: `{"NIK":"Sesuai (100)","NAMA_LGKP":"Sesuai (100)","TGL_LHR":"Tidak Sesuai"}`}
	DukcapilNotFound = DukcapilReply{Status: 200, Body: `{"RESPONSE_CODE":"13","RESPONSE_DESC":"Data Tidak Ditemukan, NIK tidak terdapat di database Kependudukan","RESPONSE":"Data Tidak Ditemukan, NIK tidak terdapat di database Kependudukan"}`}
	DukcapilDown     = DukcapilReply{Status: 503, Body: ""}
	DukcapilHang     = DukcapilReply{Hang: true}
)

// StartFakeDukcapil starts the fake with an initial script.
func StartFakeDukcapil(t *testing.T, replies ...DukcapilReply) *FakeDukcapil {
	t.Helper()
	return &FakeDukcapil{StartFakeAPI(t, replies...)}
}

// Requests returns what the fake received so far.
func (f *FakeDukcapil) Requests() []DukcapilRequest {
	var out []DukcapilRequest
	for _, r := range f.Calls() {
		out = append(out, DukcapilRequest{r.Path, r.Header.Get("x-Gateway-APIKey"), r.Header.Get("Content-Type"), r.Body})
	}
	return out
}

// UseDukcapil points the Dukcapil settings at the fake.
func (h *Harness) UseDukcapil(t *testing.T, f *FakeDukcapil) {
	t.Helper()
	h.useAPI(t, "dukcapil", f.URL, "DUKCAPIL_APIKEY", DukcapilAPIKey)
}
