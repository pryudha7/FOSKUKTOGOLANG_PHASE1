package apptest

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"fos/internal/config"
)

// FakeGCS is Google Cloud Storage's JSON API and OAuth token endpoint for
// tests: it checks the service-account JWT signature and the bearer token,
// and keeps objects in memory.
type FakeGCS struct {
	URL string
	// PrivateKey is the key in the credential file; tests assert it never
	// reaches logs or responses.
	PrivateKey string

	pub     *rsa.PublicKey
	email   string
	mu      sync.Mutex
	objects map[string]FakeObject // "<bucket>/<object>"
	down    bool
	tokens  map[string]bool
}

// FakeObject is a stored object.
type FakeObject struct {
	ContentType string
	Data        []byte
}

// StartFakeGCS starts the fake and returns it with a configure function that
// points the app at it with bucket "fos-test" and a credential file.
func StartFakeGCS(t *testing.T) (*FakeGCS, func(*config.Config)) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	f := &FakeGCS{
		PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		pub:        &key.PublicKey, email: "fos@test.iam.gserviceaccount.com",
		objects: map[string]FakeObject{}, tokens: map[string]bool{},
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	f.URL = srv.URL
	cred, _ := json.Marshal(map[string]string{
		"type": "service_account", "client_email": f.email, "private_key": f.PrivateKey,
		"private_key_id": "test-key", "token_uri": srv.URL + "/token",
	})
	path := filepath.Join(t.TempDir(), "gcs.json")
	if err := os.WriteFile(path, cred, 0o600); err != nil {
		t.Fatal(err)
	}
	return f, func(c *config.Config) {
		c.GCS = config.GCS{ProjectID: "fos-test-project", BucketName: "fos-test", Credential: path, APIBaseURL: srv.URL}
	}
}

// SetDown makes every request fail with 503, as if GCS were unreachable.
func (f *FakeGCS) SetDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = down
}

// Objects returns a copy of the stored objects keyed "<bucket>/<object>".
func (f *FakeGCS) Objects() map[string]FakeObject {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]FakeObject{}
	for k, v := range f.objects {
		out[k] = v
	}
	return out
}

func (f *FakeGCS) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if r.URL.Path == "/token" {
		f.issueToken(w, r)
		return
	}
	if !f.tokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")] {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	path := r.URL.EscapedPath()
	switch {
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/upload/storage/v1/b/"):
		bucket, _ := url.PathUnescape(strings.TrimSuffix(strings.TrimPrefix(path, "/upload/storage/v1/b/"), "/o"))
		data, _ := io.ReadAll(r.Body)
		f.objects[bucket+"/"+r.URL.Query().Get("name")] = FakeObject{r.Header.Get("Content-Type"), data}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	case strings.HasPrefix(path, "/storage/v1/b/"):
		bucket, object, ok := strings.Cut(strings.TrimPrefix(path, "/storage/v1/b/"), "/o/")
		b, _ := url.PathUnescape(bucket)
		o, _ := url.PathUnescape(object)
		obj, exists := f.objects[b+"/"+o]
		switch {
		case !ok || !exists:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.Query().Get("alt") == "media":
			w.Header().Set("Content-Type", obj.ContentType)
			_, _ = w.Write(obj.Data)
		case r.Method == http.MethodDelete:
			delete(f.objects, b+"/"+o)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// issueToken accepts only a JWT bearer assertion signed by the credential key.
func (f *FakeGCS) issueToken(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	parts := strings.Split(r.FormValue("assertion"), ".")
	if len(parts) != 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	var claims struct {
		Iss string `json:"iss"`
	}
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	if rsa.VerifyPKCS1v15(f.pub, crypto.SHA256, sum[:], sig) != nil || json.Unmarshal(raw, &claims) != nil || claims.Iss != f.email {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	token := "fake-token-" + base64.RawURLEncoding.EncodeToString(sum[:8])
	f.tokens[token] = true
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"access_token":"` + token + `","expires_in":3600,"token_type":"Bearer"}`))
}
