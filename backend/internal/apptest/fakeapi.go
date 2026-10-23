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

// FakeAPI is an outbound API of the Pre-Screening Check, for tests. Each call takes the
// next scripted reply; when the script runs out the last reply repeats.
type FakeAPI struct {
	URL string

	mu       sync.Mutex
	replies  []FakeReply
	requests []FakeRequest
}

// FakeReply is one answer: Status with Body (JSON unless ContentType says
// otherwise), or a hang longer than the caller's timeout.
type FakeReply struct {
	Status      int
	Body        string
	Hang        bool
	ContentType string
}

// FakeRequest is what the fake received.
type FakeRequest struct {
	Method, Path string
	Header       http.Header
	Body         string
}

// StartFakeAPI starts a fake with an initial script.
func StartFakeAPI(t *testing.T, replies ...FakeReply) *FakeAPI {
	t.Helper()
	f := &FakeAPI{replies: replies}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	f.URL = srv.URL
	return f
}

// Script replaces the remaining replies.
func (f *FakeAPI) Script(replies ...FakeReply) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies = replies
}

// Calls returns what the fake received so far.
func (f *FakeAPI) Calls() []FakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]FakeRequest(nil), f.requests...)
}

func (f *FakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, FakeRequest{r.Method, r.URL.EscapedPath(), r.Header.Clone(), string(body)})
	reply := FakeReply{Status: 500}
	if len(f.replies) > 0 {
		reply = f.replies[0]
		if len(f.replies) > 1 {
			f.replies = f.replies[1:]
		}
	}
	f.mu.Unlock()
	writeReply(w, r, reply)
}

func writeReply(w http.ResponseWriter, r *http.Request, reply FakeReply) {
	if reply.Hang {
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
		return
	}
	if reply.ContentType == "" {
		reply.ContentType = "application/json"
	}
	w.Header().Set("Content-Type", reply.ContentType)
	w.Header().Set("Set-Cookie", "gw=secret-cookie")
	w.WriteHeader(reply.Status)
	_, _ = io.WriteString(w, reply.Body)
}

// useAPI points an API's settings at a fake, with fast retries and a short
// timeout, and provides its key in the environment variable keyEnv.
func (h *Harness) useAPI(t *testing.T, api, url, keyEnv, key string) {
	t.Helper()
	t.Setenv(keyEnv, key)
	ctx := context.Background()
	for purpose, value := range map[string]string{"api/" + api + "/retry_gap_ms": "20", "api/" + api + "/timeout_ms": "300"} {
		if _, err := h.DB.Exec(ctx, `UPDATE fos.dynamic_system_settings SET value = $2 WHERE purpose = $1`, purpose, value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.DB.Exec(ctx, `
		INSERT INTO fos.dynamic_system_settings (app_scope, purpose, value, value_type, updated_at, updated_by)
		VALUES ('LOAN_APPLICATION', $1, $2, 'STRING', now(), 'test')`, "api/"+api+"/baseurl", url); err != nil {
		t.Fatal(err)
	}
}

// DHNAPIKey is the key the DHN fake is called with.
const DHNAPIKey = "dhn-test-key"

// DHN replies (FR-PSAPI-006).
var (
	DHNNotFound  = FakeReply{Status: 200, Body: `{"code":"DN-00","message":"Data tidak ditemukan","data":{"blacklisted":false}}`}
	DHNFound     = FakeReply{Status: 200, Body: `{"code":"DN-01","message":"Data ditemukan","data":{"blacklisted":true}}`}
	DHNOtherCode = FakeReply{Status: 200, Body: `{"code":"DN-99","message":"Gangguan","data":{}}`}
)

// UseDHN points the DHN settings at a fake.
func (h *Harness) UseDHN(t *testing.T, f *FakeAPI) {
	t.Helper()
	h.useAPI(t, "dhn", f.URL, "DHN_APIKEY", DHNAPIKey)
}

// SLIKAPIKey is the key the SLIK fake is called with.
const SLIKAPIKey = "slik-test-key"

// SLIK request replies (FR-PSAPI-008 rule 3).
var (
	SLIKAccepted = FakeReply{Status: 200, Body: `{"code":"1100","message":"success"}`}
	SLIKInvalid  = FakeReply{Status: 400, Body: `{"code":"4000","message":"request invalid","errors":["pilihan fasilitas hanya boleh Seluruh atau Outstanding"]}`}
)

// UseSLIK points the SLIK settings at a fake.
func (h *Harness) UseSLIK(t *testing.T, f *FakeAPI) {
	t.Helper()
	h.useAPI(t, "slik", f.URL, "SLIK_APIKEY", SLIKAPIKey)
}
