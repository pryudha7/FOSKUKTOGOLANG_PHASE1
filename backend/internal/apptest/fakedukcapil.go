package apptest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// DukcapilAPIKey is the key the fake expects in x-Gateway-APIKey.
const DukcapilAPIKey = "dukcapil-test-key"

// FakeDukcapil is the Dukcapil gateway for tests. Each call takes the next
// scripted reply; when the script runs out the last reply repeats.
type FakeDukcapil struct {
	URL string

	mu       sync.Mutex
	replies  []DukcapilReply
	requests []DukcapilRequest
}

// DukcapilReply is one answer: Status with Body, or a hang longer than the
// caller's timeout.
type DukcapilReply struct {
	Status int
	Body   string
	Hang   bool
}

// DukcapilRequest is what the fake received.
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
	f := &FakeDukcapil{replies: replies}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	f.URL = srv.URL
	return f
}

// Script replaces the remaining replies.
func (f *FakeDukcapil) Script(replies ...DukcapilReply) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies = replies
}

// Requests returns what the fake received so far.
func (f *FakeDukcapil) Requests() []DukcapilRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]DukcapilRequest(nil), f.requests...)
}

func (f *FakeDukcapil) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, DukcapilRequest{r.URL.Path, r.Header.Get("x-Gateway-APIKey"), r.Header.Get("Content-Type"), string(body)})
	reply := DukcapilReply{Status: 500}
	if len(f.replies) > 0 {
		reply = f.replies[0]
		if len(f.replies) > 1 {
			f.replies = f.replies[1:]
		}
	}
	f.mu.Unlock()
	if reply.Hang {
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Set-Cookie", "gw=secret-cookie")
	w.WriteHeader(reply.Status)
	_, _ = io.WriteString(w, reply.Body)
}

// UseDukcapil points the Dukcapil settings at the fake, with fast retries
// and a short timeout, and provides the API key in the environment.
func (h *Harness) UseDukcapil(t *testing.T, f *FakeDukcapil) {
	t.Helper()
	t.Setenv("DUKCAPIL_APIKEY", DukcapilAPIKey)
	ctx := context.Background()
	for purpose, value := range map[string]string{"api/dukcapil/retry_gap_ms": "20", "api/dukcapil/timeout_ms": "300"} {
		if _, err := h.DB.Exec(ctx, `UPDATE fos.dynamic_system_settings SET value = $2 WHERE purpose = $1`, purpose, value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.DB.Exec(ctx, `
		INSERT INTO fos.dynamic_system_settings (app_scope, purpose, value, value_type, updated_at, updated_by)
		VALUES ('LOAN_APPLICATION', 'api/dukcapil/baseurl', $1, 'STRING', now(), 'test')`, f.URL); err != nil {
		t.Fatal(err)
	}
}
