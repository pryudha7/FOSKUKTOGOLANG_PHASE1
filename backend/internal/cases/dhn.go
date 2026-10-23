package cases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// The DHN check (FR-PSAPI-006) screens the Customer, Individual or Business
// Entity, never the Spouse. It does not reject yet: DN-01 passes too.

var dhnAPI = outboundAPI{logName: "DHN", settings: "dhn", keyHeader: "x-api-key"}

// dhnIdentityPlaceholder in api/dhn/path is replaced by the identity.
const dhnIdentityPlaceholder = "{IdentityNumber}"

func (e *Executor) dhnCheck() check {
	return check{apiName: dhnAPI.logName, try: e.tryDHN, retry: e.retrySettings(dhnAPI)}
}

func (e *Executor) tryDHN(ctx context.Context, caseID string, attemptNumber int) (attempt, error) {
	cfg, err := e.apiConfig(ctx, dhnAPI)
	if err != nil {
		return attempt{}, err
	}
	if !cfg.simulate && !strings.Contains(cfg.path, dhnIdentityPlaceholder) {
		return attempt{}, fmt.Errorf("api/dhn/path has no %s", dhnIdentityPlaceholder)
	}
	log := &apiLog{name: dhnAPI.logName, submittedAt: e.s.now(), method: http.MethodGet, attempt: attemptNumber}
	_, identity, err := customerIdentity(ctx, e.s.db, caseID)
	if errors.Is(err, errIdentityMissing) {
		// Missing request data fails the attempt, as for Dukcapil (agreed).
		return failed(log, err), nil
	}
	if err != nil {
		return attempt{}, err
	}
	// The request column holds the identity number only (PROPOSED, DEC-062).
	log.request = identity
	path := strings.Replace(cfg.path, dhnIdentityPlaceholder, url.PathEscape(identity), 1)
	return e.send(ctx, cfg, log, path, nil, map[string]string{"Accept": "application/json"}, evaluateDHN)
}

// evaluateDHN is FR-PSAPI-006 "Evaluasi hasil": DN-00 and DN-01 are Success
// and pass, since the reject criterion (DN-01) is not active; anything else
// fails.
func evaluateDHN(body string) outcome {
	var resp struct {
		Code *string `json:"code"`
	}
	if json.Unmarshal([]byte(body), &resp) != nil || resp.Code == nil {
		return outcomeFail
	}
	switch *resp.Code {
	case "DN-00", "DN-01":
		return outcomeSuccess
	}
	return outcomeFail
}
