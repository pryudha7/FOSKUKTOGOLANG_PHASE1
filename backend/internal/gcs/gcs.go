// Package gcs stores case files in Google Cloud Storage through its JSON API
// (INT-004, FR-FILE-001), authenticating with a service-account key.
//
// Errors never carry the credential or its content (ENV-009).
package gcs

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"fos/internal/config"
)

// ErrNotConfigured: a GCS_* key is empty (ENV-010).
var ErrNotConfigured = errors.New("gcs configuration incomplete")

const scope = "https://www.googleapis.com/auth/devstorage.read_write"

type Client struct {
	cfg  config.GCS
	http *http.Client
	now  func() time.Time

	mu     sync.Mutex
	token  string
	expiry time.Time
}

func New(cfg config.GCS, now func() time.Time) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 60 * time.Second}, now: now}
}

// Bucket is GCS_BUCKET_NAME.
func (c *Client) Bucket() string { return c.cfg.BucketName }

// Missing names the empty GCS_* keys.
func (c *Client) Missing() []string { return c.cfg.Missing() }

// Put stores an object.
func (c *Client) Put(ctx context.Context, object, contentType string, body []byte) error {
	u := fmt.Sprintf("%s/upload/storage/v1/b/%s/o?uploadType=media&name=%s",
		c.cfg.APIBaseURL, url.PathEscape(c.cfg.BucketName), url.QueryEscape(object))
	resp, err := c.do(ctx, http.MethodPost, u, contentType, bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Get opens an object's content.
func (c *Client) Get(ctx context.Context, object string) (io.ReadCloser, error) {
	resp, err := c.do(ctx, http.MethodGet, c.objectURL(object)+"?alt=media", "", nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Delete removes an object.
func (c *Client) Delete(ctx context.Context, object string) error {
	resp, err := c.do(ctx, http.MethodDelete, c.objectURL(object), "", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) objectURL(object string) string {
	return fmt.Sprintf("%s/storage/v1/b/%s/o/%s", c.cfg.APIBaseURL, url.PathEscape(c.cfg.BucketName), url.PathEscape(object))
}

// do sends an authorised request; a non-2xx answer is an error naming only
// the method and status.
func (c *Client) do(ctx context.Context, method, u, contentType string, body io.Reader) (*http.Response, error) {
	if len(c.cfg.Missing()) > 0 {
		return nil, ErrNotConfigured
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gcs %s: %w", method, scrub(err))
	}
	if resp.StatusCode/100 != 2 {
		resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized {
			c.mu.Lock()
			c.token = ""
			c.mu.Unlock()
		}
		return nil, fmt.Errorf("gcs %s: status %d", method, resp.StatusCode)
	}
	return resp, nil
}

// scrub drops the URL from a transport error; it names only the object.
func scrub(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// serviceAccount is the part of a service-account key file used here.
type serviceAccount struct {
	ClientEmail  string `json:"client_email"`
	PrivateKey   string `json:"private_key"`
	PrivateKeyID string `json:"private_key_id"`
	TokenURI     string `json:"token_uri"`
}

// accessToken returns a cached OAuth token, or exchanges a signed JWT for a
// new one (RFC 7523).
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && c.now().Before(c.expiry) {
		return c.token, nil
	}
	raw, err := os.ReadFile(c.cfg.Credential)
	if err != nil {
		return "", errors.New("gcs credential file unreadable")
	}
	var sa serviceAccount
	if json.Unmarshal(raw, &sa) != nil || sa.ClientEmail == "" || sa.PrivateKey == "" || sa.TokenURI == "" {
		return "", errors.New("gcs credential file invalid")
	}
	key, err := parseKey(sa.PrivateKey)
	if err != nil {
		return "", err
	}
	now := c.now()
	assertion, err := signJWT(key, sa, now)
	if err != nil {
		return "", err
	}
	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sa.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errors.New("gcs token request invalid")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gcs token: %w", scrub(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gcs token: status %d", resp.StatusCode)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if json.NewDecoder(resp.Body).Decode(&tok) != nil || tok.AccessToken == "" {
		return "", errors.New("gcs token: invalid response")
	}
	c.token = tok.AccessToken
	c.expiry = now.Add(time.Duration(tok.ExpiresIn)*time.Second - time.Minute)
	return c.token, nil
}

func parseKey(p string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(p))
	if block == nil {
		return nil, errors.New("gcs credential key invalid")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rk, ok := k.(*rsa.PrivateKey); ok {
			return rk, nil
		}
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	return nil, errors.New("gcs credential key invalid")
}

func signJWT(key *rsa.PrivateKey, sa serviceAccount, now time.Time) (string, error) {
	enc := base64.RawURLEncoding
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": sa.PrivateKeyID})
	claims, _ := json.Marshal(map[string]any{
		"iss": sa.ClientEmail, "scope": scope, "aud": sa.TokenURI,
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})
	signing := enc.EncodeToString(header) + "." + enc.EncodeToString(claims)
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", errors.New("gcs credential signing failed")
	}
	return signing + "." + enc.EncodeToString(sig), nil
}
