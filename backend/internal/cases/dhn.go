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

const (
	apiDHN       = "DHN"
	dhnKeyHeader = "x-api-key"
	// dhnIdentityPlaceholder in api/dhn/path is replaced by the identity.
	dhnIdentityPlaceholder = "{IdentityNumber}"
)

func (e *Executor) dhnCheck() check {
	return check{apiName: apiDHN, try: e.tryDHN, retry: e.retrySettings("dhn")}
}

func (e *Executor) tryDHN(ctx context.Context, caseID string, attemptNumber int) (attempt, error) {
	cfg, err := e.apiConfig(ctx, "dhn", dhnKeyHeader)
	if err != nil {
		return attempt{}, err
	}
	log := &apiLog{name: apiDHN, submittedAt: e.s.now(), method: http.MethodGet, attempt: attemptNumber}
	_, identity, err := customerIdentity(ctx, e.s.db, caseID)
	if err != nil {
		if errors.Is(err, errIdentityMissing) {
			log.status, log.response, log.method = apiFail, err.Error(), ""
			return attempt{outcomeFail, log}, nil
		}
		return attempt{}, err
	}
	// The request column holds the identity number only (PROPOSED, DEC-062).
	log.request = identity

	if cfg.simulate {
		log.status, log.response, log.method = apiSuccess, cfg.simulateResponse, ""
		return attempt{outcomeSuccess, log}, nil
	}
	if !strings.Contains(cfg.path, dhnIdentityPlaceholder) {
		return attempt{}, fmt.Errorf("api/dhn/path has no %s", dhnIdentityPlaceholder)
	}
	u, err := cfg.endpoint(strings.Replace(cfg.path, dhnIdentityPlaceholder, url.PathEscape(identity), 1))
	if err != nil {
		return attempt{}, fmt.Errorf("DHN: %w", err)
	}
	answered, err := e.call(ctx, cfg, log, u, nil, map[string]string{"Accept": "application/json"})
	if err != nil {
		return attempt{}, err
	}
	o := outcomeFail
	if answered {
		o = evaluateDHN(log.response)
	}
	log.status = o.logStatus()
	return attempt{o, log}, nil
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

var errIdentityMissing = errors.New("customer identity missing: NIK or NPWP is empty")

// customerIdentity is the case's Tipe Nasabah and the Customer's identity
// number: the NIK of an Individual, the NPWP of a Business Entity.
func customerIdentity(ctx context.Context, q rowQuerier, caseID string) (string, string, error) {
	var customerType *string
	if err := q.QueryRow(ctx, `SELECT customer_type FROM fos.work WHERE case_id = $1`, caseID).Scan(&customerType); err != nil {
		return "", "", err
	}
	party, column := partyCustomer, "id_number"
	if value(customerType) == customerTypeBusinessEntity {
		party, column = customerTypeBusinessEntity, "npwp"
	}
	var id *string
	err := q.QueryRow(ctx, `SELECT `+column+` FROM fos.customer WHERE case_id = $1 AND customer_type = $2`, caseID, party).Scan(&id)
	if isNotFound(err) || (err == nil && strings.TrimSpace(value(id)) == "") {
		return value(customerType), "", errIdentityMissing
	}
	return value(customerType), value(id), err
}
