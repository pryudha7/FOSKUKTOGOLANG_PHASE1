package cases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// The Dukcapil check (FR-PSAPI-003) checks an Individual Customer's
// personal data, never the Spouse's or a Business Entity's.

const (
	apiDukcapil = "Dukcapil"
	// dukcapilKeyHeader carries the API key (DEC-060).
	dukcapilKeyHeader = "x-Gateway-APIKey"
)

// dukcapilRequest is the body, in the SoT's field order.
type dukcapilRequest struct {
	NIK          string `json:"nik"`
	TanggalLahir string `json:"tanggalLahir"` // dd-MM-yyyy
	NamaLengkap  string `json:"namaLengkap"`
}

func (e *Executor) dukcapilCheck() check {
	return check{apiName: apiDukcapil, try: e.tryDukcapil, retry: e.retrySettings("dukcapil")}
}

func (e *Executor) tryDukcapil(ctx context.Context, caseID string, attemptNumber int) (attempt, error) {
	var customerType *string
	if err := e.s.db.QueryRow(ctx, `SELECT customer_type FROM fos.work WHERE case_id = $1`, caseID).Scan(&customerType); err != nil {
		return attempt{}, err
	}
	if value(customerType) != customerTypeIndividual {
		return attempt{outcome: outcomeSkip}, nil
	}
	cfg, err := e.apiConfig(ctx, "dukcapil", dukcapilKeyHeader)
	if err != nil {
		return attempt{}, err
	}
	log := &apiLog{name: apiDukcapil, submittedAt: e.s.now(), method: http.MethodPost, attempt: attemptNumber}

	// FR-PSAPI-002 rule 6: the request is built from the case data each time;
	// missing data is a Fail of the attempt (FR-PSAPI-003, agreed).
	body, err := e.dukcapilBody(ctx, caseID)
	if err != nil {
		log.status, log.response, log.method = apiFail, err.Error(), ""
		return attempt{outcomeFail, log}, nil
	}
	log.request = string(body)

	if cfg.simulate {
		// No call; the configured response is logged as Success (FR-PSAPI-003).
		log.status, log.response, log.method = apiSuccess, cfg.simulateResponse, ""
		return attempt{outcomeSuccess, log}, nil
	}
	u, err := cfg.endpoint(cfg.path)
	if err != nil {
		return attempt{}, fmt.Errorf("Dukcapil: %w", err)
	}
	answered, err := e.call(ctx, cfg, log, u, body, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return attempt{}, err
	}
	o := outcomeFail
	if answered {
		o = evaluateDukcapil(log.response)
	}
	log.status = o.logStatus()
	return attempt{o, log}, nil
}

// dukcapilBody is the request for the case's Individual Customer.
func (e *Executor) dukcapilBody(ctx context.Context, caseID string) ([]byte, error) {
	var (
		nik, name *string
		birthDate *time.Time
	)
	err := e.s.db.QueryRow(ctx, `
		SELECT id_number, full_name, birth_date FROM fos.customer WHERE case_id = $1 AND customer_type = $2`,
		caseID, partyCustomer).Scan(&nik, &name, &birthDate)
	if isNotFound(err) || (err == nil && (value(nik) == "" || value(name) == "" || birthDate == nil)) {
		return nil, errors.New("Dukcapil request data missing: the customer's NIK, Nama or Tanggal Lahir is empty")
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(dukcapilRequest{value(nik), birthDate.Format("02-01-2006"), value(name)})
}

// evaluateDukcapil is FR-PSAPI-003 "Evaluasi hasil", judged on the body only.
func evaluateDukcapil(body string) outcome {
	var fields map[string]any
	if json.Unmarshal([]byte(body), &fields) != nil {
		return outcomeFail
	}
	if code, ok := fields["RESPONSE_CODE"]; ok {
		// Data not found passes (PROPOSED); the code may come as text or number.
		if fmt.Sprint(code) == "13" {
			return outcomeSuccess
		}
		return outcomeFail
	}
	for _, k := range []string{"NIK", "NAMA_LGKP"} {
		if _, ok := fields[k].(string); !ok {
			return outcomeFail
		}
	}
	birth, _ := fields["TGL_LHR"].(string)
	switch {
	case strings.HasPrefix(birth, "Tidak Sesuai"):
		return outcomeRejected
	case strings.HasPrefix(birth, "Sesuai"):
		return outcomeSuccess
	}
	return outcomeFail
}
