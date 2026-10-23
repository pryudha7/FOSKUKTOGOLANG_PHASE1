package apptest

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Pefindo API routes of the fake (FR-PSAPI-014 to FR-PSAPI-018).
const (
	PefindoToken    = "/api/v1/getToken"
	PefindoSearch   = "/api/v1/product/search"
	PefindoGenerate = "/api/v1/product/generateReport"
	PefindoDownload = "/api/v1/product/downloadPdfReport/eventId/"
	PefindoReport   = "/api/v1/product/getReport/eventId/"
)

// FakePefindo is the Pefindo API for tests: one server, a script per route
// (matched by path prefix). Each call takes the route's next reply; the last
// reply repeats.
type FakePefindo struct {
	URL string

	mu       sync.Mutex
	scripts  map[string][]FakeReply
	requests []FakeRequest
}

// Pefindo test credentials and replies.
const (
	PefindoAPIKey        = "pefindo-test-key"
	PefindoAuthorization = "Basic cGVmaW5kbzpzZWNyZXQ="
	PefindoTokenValue    = "tok-1"
)

var (
	PefindoTokenOK     = FakeReply{Status: 200, Body: `{"code":"01","status":"Success","data":{"token":"tok-1"}}`}
	PefindoNotFound    = FakeReply{Status: 200, Body: `{"code":"31","status":"Success","message":"Data tidak ditemukan","data":[]}`}
	PefindoExpired     = FakeReply{Status: 200, Body: `{"code":"06","status":"Failed","message":"Token expired"}`}
	PefindoGenerated   = FakeReply{Status: 200, Body: `{"code":"01","status":"Success","message":"Proses membuat report sedang dikerjakan","event_id":"x"}`}
	PefindoScoring     = FakeReply{Status: 200, Body: `{"code":"32","status":"Success","message":"Laporan masih dalam proses scoring"}`}
	PefindoPDF         = FakeReply{Status: 200, Body: "%PDF-1.4 pefindo report", ContentType: "application/pdf"}
	PefindoNoPDFYet    = FakeReply{Status: 200, Body: `{"code":"31","status":"Success","message":"Proses membuat report sedang dikerjakan"}`}
	PefindoReportEmpty = FakeReply{Status: 200, Body: `{"code":"31","status":"Success","message":"Data tidak ditemukan"}`}
)

// PefindoFound is a Search answer with the given data elements (JSON).
func PefindoFound(data ...string) FakeReply {
	return FakeReply{Status: 200, Body: `{"code":"01","status":"Success","message":"Data ditemukan","data":[` + strings.Join(data, ",") + `],"inquiry_id":1436899,"response_status":"ALL_CORRECT"}`}
}

// StartFakePefindo starts the fake with a script per route.
func StartFakePefindo(t *testing.T, scripts map[string][]FakeReply) *FakePefindo {
	t.Helper()
	f := &FakePefindo{scripts: scripts}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	f.URL = srv.URL
	return f
}

// Script replaces a route's remaining replies.
func (f *FakePefindo) Script(route string, replies ...FakeReply) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scripts[route] = replies
}

// Calls returns what the fake received so far, in order.
func (f *FakePefindo) Calls() []FakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]FakeRequest(nil), f.requests...)
}

func (f *FakePefindo) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	path := r.URL.EscapedPath()
	f.mu.Lock()
	f.requests = append(f.requests, FakeRequest{r.Method, path, r.Header.Clone(), string(body)})
	reply := FakeReply{Status: 500}
	for route, script := range f.scripts {
		if strings.HasPrefix(path, route) && len(script) > 0 {
			reply = script[0]
			if len(script) > 1 {
				f.scripts[route] = script[1:]
			}
		}
	}
	f.mu.Unlock()
	writeReply(w, r, reply)
}

// UsePefindo points the Pefindo settings at the fake, with fast retries, no
// wait before Download PDF, and its credentials and token key in the
// environment.
func (h *Harness) UsePefindo(t *testing.T, f *FakePefindo) {
	t.Helper()
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	t.Setenv("PEFINDO_APIKEY", PefindoAPIKey)
	t.Setenv("PEFINDO_TOKEN_AUTHORIZATION", PefindoAuthorization)
	t.Setenv("INTEGRATION_TOKEN_KEY", base64.StdEncoding.EncodeToString(key))
	ctx := context.Background()
	for _, api := range []string{"pefindo", "pefindotoken", "pefindogeneratereport", "pefindodownloadpdf", "pefindogetreport"} {
		for purpose, value := range map[string]string{"api/" + api + "/retry_gap_ms": "20", "api/" + api + "/timeout_ms": "300"} {
			if _, err := h.DB.Exec(ctx, `UPDATE fos.dynamic_system_settings SET value = $2 WHERE purpose = $1`, purpose, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := h.DB.Exec(ctx, `UPDATE fos.dynamic_system_settings SET value = '0' WHERE purpose = 'process/pefindo/generate_to_download_gap_ms'`); err != nil {
		t.Fatal(err)
	}
	if _, err := h.DB.Exec(ctx, `
		INSERT INTO fos.dynamic_system_settings (app_scope, purpose, value, value_type, updated_at, updated_by)
		VALUES ('LOAN_APPLICATION', 'api/pefindo/baseurl', $1, 'STRING', now(), 'test'),
		       ('LOAN_APPLICATION', 'api/pefindo/product_id', '1', 'STRING', now(), 'test')`, f.URL); err != nil {
		t.Fatal(err)
	}
}
