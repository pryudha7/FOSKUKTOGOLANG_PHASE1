package cases

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// The Pefindo bearer token (FR-PSAPI-015) is requested when needed: when
// none is stored, or when a Pefindo API answers code 06. It is stored
// encrypted in integration_token (§9.24, DEC-099) so every instance shares
// it, and is decrypted only in memory (DB-RULE-081).

const providerPefindo = "PEFINDO"

var pefindoTokenAPI = outboundAPI{logName: "PefindoToken", settings: "pefindotoken", baseFrom: "pefindo"}

// bearer is a token in use; nonce tells it from a newer one (DB-RULE-080).
type bearer struct {
	value string
	nonce []byte
}

// tokenKey reads the AES-256-GCM key from the environment variable named by
// secret/encryption/integration_token_key_ref; the name is the key version
// (agreed for ticket 13). A missing or malformed key is a system failure.
func (e *Executor) tokenKey(ctx context.Context) (cipher.AEAD, string, error) {
	ref, err := e.s.settings.String(ctx, "secret/encryption/integration_token_key_ref")
	if err != nil {
		return nil, "", err
	}
	key, err := base64.StdEncoding.DecodeString(e.getenv(ref))
	if err != nil || len(key) != 32 {
		return nil, "", fmt.Errorf("integration token key: environment variable %s is not a base64 32-byte key (DEC-099)", ref)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, "", err
	}
	aead, err := cipher.NewGCM(block)
	return aead, ref, err
}

// readToken reads and decrypts the stored token (lock: "" or " FOR
// UPDATE"); ok is false when there is none usable (never requested, another
// key version, or not decryptable).
func readToken(ctx context.Context, q rowQuerier, lock string, aead cipher.AEAD, version string) (bearer, bool, error) {
	var ciphertext, nonce []byte
	var keyVersion *string
	if err := q.QueryRow(ctx, `SELECT token_ciphertext, token_nonce, key_version FROM fos.integration_token WHERE provider = $1`+lock,
		providerPefindo).Scan(&ciphertext, &nonce, &keyVersion); err != nil {
		return bearer{}, false, err
	}
	t, ok := storedToken(aead, version, ciphertext, nonce, keyVersion)
	return t, ok, nil
}

// storedToken decrypts a token row; ok is false when it is not usable.
func storedToken(aead cipher.AEAD, version string, ciphertext, nonce []byte, keyVersion *string) (bearer, bool) {
	if ciphertext == nil || value(keyVersion) != version || len(nonce) != aead.NonceSize() {
		return bearer{}, false
	}
	plain, err := aead.Open(nil, nonce, ciphertext, []byte(providerPefindo))
	if err != nil || len(plain) == 0 {
		return bearer{}, false
	}
	return bearer{string(plain), nonce}, true
}

// pefindoToken returns the stored token, requesting one when none is stored.
func (e *Executor) pefindoToken(ctx context.Context) (bearer, error) {
	aead, version, err := e.tokenKey(ctx)
	if err != nil {
		return bearer{}, err
	}
	t, ok, err := readToken(ctx, e.s.db, ``, aead, version)
	if err != nil || ok {
		return t, err
	}
	return e.renewPefindoToken(ctx, nil)
}

// renewPefindoToken requests a token from getToken, once, unless the stored
// token is no longer the one that got code 06 (used; nil when there was
// none): another executor renewed it while this one waited for the row
// lock, so concurrent requests merge into one (DB-RULE-080). A failed
// request is logged and returns no token; it is not repeated (FR-PSAPI-015).
func (e *Executor) renewPefindoToken(ctx context.Context, used []byte) (bearer, error) {
	aead, version, err := e.tokenKey(ctx)
	if err != nil {
		return bearer{}, err
	}
	cfg, authorization, err := e.tokenConfig(ctx)
	if err != nil {
		return bearer{}, err
	}
	tx, err := e.s.db.Begin(ctx)
	if err != nil {
		return bearer{}, err
	}
	defer tx.Rollback(ctx)
	// DB-RULE-080 compares obtained_at; the nonce tells the renewed token
	// apart the same way without depending on the clock (agreed for ticket 13).
	if t, ok, err := readToken(ctx, tx, ` FOR UPDATE`, aead, version); err != nil || ok && !bytes.Equal(t.nonce, used) {
		if err != nil {
			return bearer{}, err
		}
		return t, tx.Commit(ctx)
	}

	log := &apiLog{name: pefindoTokenAPI.logName, submittedAt: e.s.now(), method: http.MethodGet, attempt: 1}
	u, err := cfg.endpoint(cfg.path)
	if err != nil {
		return bearer{}, fmt.Errorf("%s: %w", log.name, err)
	}
	answer, err := e.call(ctx, cfg, log, u, nil, map[string]string{"Content-Type": "application/json", "Authorization": authorization})
	if err != nil {
		return bearer{}, err
	}
	var resp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	token := ""
	if answer != nil && json.Unmarshal(answer, &resp) == nil {
		token = resp.Data.Token
	}
	log.status = apiFail
	t := bearer{}
	if token != "" {
		log.status = apiSuccess
		log.response = strings.ReplaceAll(log.response, token, "***") // DB-RULE-073
		t = bearer{token, make([]byte, aead.NonceSize())}
		if _, err := rand.Read(t.nonce); err != nil {
			return bearer{}, err
		}
		now := e.s.now()
		if _, err := tx.Exec(ctx, `
			UPDATE fos.integration_token SET token_ciphertext = $2, token_nonce = $3, key_version = $4, obtained_at = $5, updated_at = $5
			WHERE provider = $1`, providerPefindo, aead.Seal(nil, t.nonce, []byte(token), []byte(providerPefindo)), t.nonce, version, now); err != nil {
			return bearer{}, err
		}
	}
	// The token row has no case (FR-PSAPI-015 Log API).
	if err := log.insert(ctx, tx, ""); err != nil {
		return bearer{}, err
	}
	return t, tx.Commit(ctx)
}

// tokenConfig reads getToken's settings: Pefindo's base URL, its path and
// timeout, and the Authorization value named by
// secret/pefindotoken/authorization_ref (DEC-060).
func (e *Executor) tokenConfig(ctx context.Context) (apiConfig, string, error) {
	st := e.s.settings
	c := apiConfig{}
	var err error
	if c.maskedHeaders, err = e.s.maskedHeaderNames(ctx); err != nil {
		return c, "", err
	}
	c.maskedHeaders = append(c.maskedHeaders, "Authorization")
	a := pefindoTokenAPI
	if c.base, err = st.String(ctx, "api/"+a.baseFrom+"/baseurl"); err != nil {
		return c, "", err
	}
	if c.path, err = st.String(ctx, "api/"+a.settings+"/path"); err != nil {
		return c, "", err
	}
	timeoutMS, err := st.Int(ctx, "api/"+a.settings+"/timeout_ms")
	if err != nil {
		return c, "", err
	}
	c.timeout = time.Duration(timeoutMS) * time.Millisecond
	ref, err := st.String(ctx, "secret/"+a.settings+"/authorization_ref")
	if err != nil {
		return c, "", err
	}
	authorization := e.getenv(ref)
	if authorization == "" {
		return c, "", errors.New("Pefindo getToken Authorization: environment variable " + ref + " is empty (DEC-060)")
	}
	return c, authorization, nil
}
