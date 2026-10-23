package cases

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The SLIK request (FR-PSAPI-008) asks SFS for the Individual Customer's
// SLIK report, then the Spouse's when there is one. A request only passes
// or fails; the result arrives later through PBSResponse (pbs.go). Each
// subject has one reference code (kode_reff_pengguna), reused on retry.

const (
	stepSLIK       = "SLIK"
	stepSLIKSpouse = "SLIK Pasangan"

	subjectCustomer = "CUSTOMER"
	subjectSpouse   = "SPOUSE"

	// slik_report states (DB-RULE-058).
	slikPendingRequest = "PENDING_REQUEST"
	slikRequested      = "REQUESTED"
	slikReceived       = "RECEIVED"

	// SLIK document slots (DB-RULE-064).
	categoryZIPRequest = "PreScreeningZIPRequest"
	categoryResponse   = "PreScreeningResponse"
	docZIPRequest      = "SLIK_ZIP_REQUEST"
	docZIPCustomer     = "SLIK_ZIP_CUSTOMER"
	docZIPSpouse       = "SLIK_ZIP_SPOUSE"

	categoryPreScreening = "DokumenPreScreening"
)

// slikSubject is what differs between the Customer's SLIK and the Spouse's:
// the customer row, the API name and the ZIP result slot and name.
type slikSubject struct {
	code, party string
	api         outboundAPI
	zipDoc      string
	zipName     func(referenceCode string) string
}

var slikSubjects = map[string]slikSubject{
	subjectCustomer: {subjectCustomer, partyCustomer, outboundAPI{logName: "SLIK", settings: "slik", keyHeader: "x-api-key"},
		docZIPCustomer, func(code string) string { return code + ".zip" }},
	subjectSpouse: {subjectSpouse, partySpouse, outboundAPI{logName: "SLIKPasangan", settings: "slik", keyHeader: "x-api-key"},
		docZIPSpouse, func(code string) string { return code + " - Spouse.zip" }},
}

var (

	// jakarta is Asia/Jakarta, which has no daylight saving.
	jakarta = time.FixedZone("Asia/Jakarta", 7*3600)
)

func (e *Executor) slikCheck(subject slikSubject) check {
	return check{
		apiName: subject.api.logName,
		try: func(ctx context.Context, r runRef) (attempt, error) {
			return e.trySLIK(ctx, r.caseID, r.attempt, subject)
		},
		retry:    e.retrySettings(subject.api),
		bypassed: e.slikBypass,
	}
}

func (e *Executor) trySLIK(ctx context.Context, caseID string, attemptNumber int, subject slikSubject) (attempt, error) {
	customerType, err := caseCustomerType(ctx, e.s.db, caseID)
	if err != nil {
		return attempt{}, err
	}
	// A Business Entity uploads SLIK by hand (FR-FILE-004).
	if customerType != customerTypeIndividual {
		return attempt{outcome: outcomeSkip}, nil
	}
	var identity *string
	err = e.s.db.QueryRow(ctx, `SELECT id_number FROM fos.customer WHERE case_id = $1 AND customer_type = $2`, caseID, subject.party).Scan(&identity)
	if isNotFound(err) && subject.code == subjectSpouse {
		return attempt{outcome: outcomeSkip}, nil // no Spouse data: no SLIK Pasangan
	}
	if err != nil && !isNotFound(err) {
		return attempt{}, err
	}
	cfg, err := e.apiConfig(ctx, subject.api)
	if err != nil {
		return attempt{}, err
	}
	log := &apiLog{name: subject.api.logName, submittedAt: e.s.now(), method: http.MethodPost, attempt: attemptNumber}
	if value(identity) == "" {
		return failed(log, errors.New("SLIK request data missing: the NIK is empty")), nil
	}

	report, err := e.slikReport(ctx, caseID, subject.code, value(identity))
	if err != nil {
		if errors.Is(err, errBranchMissing) {
			return failed(log, err), nil
		}
		return attempt{}, err
	}
	if report.status != slikPendingRequest {
		return attempt{outcome: outcomeSkip}, nil // requested before an interruption
	}

	zipDoc, err := e.zipRequest(ctx, caseID)
	if err != nil {
		if errors.Is(err, ErrStorageFailed) {
			return attempt{}, err // GCS not configured: a system failure (agreed)
		}
		return failed(log, fmt.Errorf("SLIK ZIP request: %w", err)), nil
	}
	fields, err := e.slikFields(ctx, caseID, report.referenceCode, value(identity))
	if err != nil {
		return failed(log, err), nil
	}

	// The log keeps the eight text fields and a reference to the ZIP, never
	// its content (FR-PSAPI-008 rule 4, DB-RULE-086).
	logged := map[string]string{}
	for _, f := range fields {
		logged[f.name] = f.value
	}
	logged["underlying"] = fmt.Sprintf("[file %s, application/zip, %d bytes, document %s]", zipDoc.fileName, len(zipDoc.data), zipDoc.id)
	reqJSON, _ := json.Marshal(logged)
	log.request = string(reqJSON)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, f := range fields {
		_ = mw.WriteField(f.name, f.value)
	}
	part, err := mw.CreateFormFile("underlying", zipDoc.fileName)
	if err != nil {
		return attempt{}, err
	}
	_, _ = part.Write(zipDoc.data)
	_ = mw.Close()

	a, err := e.send(ctx, cfg, log, cfg.path, body.Bytes(), map[string]string{"Content-Type": mw.FormDataContentType()}, evaluateSLIKRequest)
	if err != nil {
		return attempt{}, err
	}
	if a.outcome == outcomeSuccess {
		id := report.id
		a.apply = func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `UPDATE fos.slik_report SET status = $2, requested_at = $3, updated_at = $3 WHERE slik_report_id = $1 AND status = $4`,
				id, slikRequested, e.s.now(), slikPendingRequest)
			return err
		}
	}
	return a, nil
}

// evaluateSLIKRequest passes a JSON answer with code "1100"; anything else
// fails. A request never rejects (FR-PSAPI-008 rule 3).
func evaluateSLIKRequest(body string) outcome {
	var resp struct {
		Code any `json:"code"`
	}
	if json.Unmarshal([]byte(body), &resp) != nil || fmt.Sprint(resp.Code) != "1100" {
		return outcomeFail
	}
	return outcomeSuccess
}

// slikReportRef is the case's slik_report row for one subject.
type slikReportRef struct {
	id                    int64
	referenceCode, status string
}

var errBranchMissing = errors.New("SLIK request data missing: the case has no branch code")

// slikReport returns the subject's row, creating it with its reference code
// the first time so every retry reuses the code (DB-RULE-057).
func (e *Executor) slikReport(ctx context.Context, caseID, subject, identity string) (slikReportRef, error) {
	var r slikReportRef
	err := e.s.db.QueryRow(ctx, `SELECT slik_report_id, reference_code, status FROM fos.slik_report WHERE case_id = $1 AND subject_type = $2`,
		caseID, subject).Scan(&r.id, &r.referenceCode, &r.status)
	if err == nil || !isNotFound(err) {
		return r, err
	}
	var branch *string
	if err := e.s.db.QueryRow(ctx, `SELECT branch_code FROM fos.work WHERE case_id = $1`, caseID).Scan(&branch); err != nil {
		return r, err
	}
	if value(branch) == "" {
		return r, errBranchMissing
	}
	var n int64
	if err := e.s.db.QueryRow(ctx, `SELECT count(*) FROM fos.slik_report`).Scan(&n); err != nil {
		return r, err
	}
	now := e.s.now()
	local := now.In(jakarta)
	// A clash on the unique code raises the number once (DB-RULE-057).
	for _, seq := range []int64{n + 1, n + 2} {
		code := fmt.Sprintf("SKL253/LOS%s/%02d/%04d/%d", value(branch), int(local.Month()), local.Year(), seq)
		err := e.s.db.QueryRow(ctx, `
			INSERT INTO fos.slik_report (case_id, subject_type, reference_code, identity_number, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $6) RETURNING slik_report_id`,
			caseID, subject, code, identity, slikPendingRequest, now).Scan(&r.id)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			continue
		}
		if err != nil {
			return r, err
		}
		r.referenceCode, r.status = code, slikPendingRequest
		return r, nil
	}
	return r, errors.New("SLIK kode reff could not be made unique")
}

// slikField is one form field of the request, in the SoT's order.
type slikField struct{ name, value string }

// slikFields are the eight text fields (FR-PSAPI-008 Request), read again
// on every attempt (FR-PSAPI-002 rule 6).
func (e *Executor) slikFields(ctx context.Context, caseID, referenceCode, identity string) ([]slikField, error) {
	var purpose, facility, memo, address, searchType, segmentation *string
	if err := e.s.db.QueryRow(ctx, `
		SELECT slik_request_purpose_code, slik_facility_option_code, memo_number, slik_identity_address,
		       slik_search_type_code, slik_segmentation_code
		FROM fos.work WHERE case_id = $1`, caseID).Scan(&purpose, &facility, &memo, &address, &searchType, &segmentation); err != nil {
		return nil, err
	}
	fields := []slikField{
		{"kode_reff_pengguna", referenceCode},
		{"tujuan_permintaan", value(purpose)},
		{"pilihan_fasilitas", value(facility)},
		{"nomor_memo", value(memo)},
		{"alamat_sesuai_identitas", value(address)},
		{"tipe_pencarian", value(searchType)},
		{"segmentasi", value(segmentation)},
		{"nomor_identitas", identity},
	}
	for _, f := range fields {
		if f.value == "" {
			return nil, fmt.Errorf("SLIK request data missing: %s is empty", f.name)
		}
	}
	return fields, nil
}

// zipFile is the ZIP request being sent.
type zipFile struct {
	id       uuid.UUID
	fileName string
	data     []byte
}

// zipRequest returns the case's ZIP of its Pre-Screening documents, making
// it once; SLIK and SLIK Pasangan send the same ZIP (FR-PSAPI-008 rule 2).
func (e *Executor) zipRequest(ctx context.Context, caseID string) (zipFile, error) {
	s := e.s
	var (
		z      zipFile
		stored storedObject
	)
	err := s.db.QueryRow(ctx, `
		SELECT file_id, file_name, bucket_name, object_name FROM fos.case_document
		WHERE case_id = $1 AND document_category = $2 AND document_type = $3`, caseID, categoryZIPRequest, docZIPRequest).
		Scan(&z.id, &z.fileName, &stored.bucket, &stored.object)
	if err == nil {
		if err := s.storageConfigured(); err != nil {
			return zipFile{}, err
		}
		z.data, err = s.readObject(ctx, stored)
		return z, err
	}
	if !isNotFound(err) {
		return zipFile{}, err
	}

	type doc struct {
		name string
		obj  storedObject
	}
	docs, err := collect(ctx, s, func(r pgx.Rows) (doc, error) {
		var d doc
		return d, r.Scan(&d.name, &d.obj.bucket, &d.obj.object)
	}, `SELECT file_name, bucket_name, object_name FROM fos.case_document WHERE case_id = $1 AND document_category = $2 ORDER BY created_at, file_name`,
		caseID, categoryPreScreening)
	if err != nil {
		return zipFile{}, err
	}
	if len(docs) == 0 {
		return zipFile{}, errors.New("the case has no Pre-Screening documents")
	}
	if err := s.storageConfigured(); err != nil {
		return zipFile{}, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	used := map[string]int{}
	for _, d := range docs {
		data, err := s.readObject(ctx, d.obj)
		if err != nil {
			return zipFile{}, err
		}
		name := d.name
		if used[name]++; used[name] > 1 {
			name = strconv.Itoa(used[name]) + "-" + name
		}
		w, err := zw.Create(name)
		if err != nil {
			return zipFile{}, err
		}
		if _, err := w.Write(data); err != nil {
			return zipFile{}, err
		}
	}
	if err := zw.Close(); err != nil {
		return zipFile{}, err
	}
	_, nik, err := customerIdentity(ctx, s.db, caseID)
	if err != nil {
		return zipFile{}, err
	}
	by, err := lastReviewer(ctx, s.db, caseID)
	if err != nil {
		return zipFile{}, err
	}
	z.fileName = fmt.Sprintf("Pre-Screening-%s-%s-%s.zip", caseID, nik, s.now().In(jakarta).Format("20060102"))
	z.data = buf.Bytes()
	z.id, err = s.storeGenerated(ctx, caseID, generatedDoc{
		category: categoryZIPRequest, docType: docZIPRequest, fileName: z.fileName,
		contentType: "application/zip", data: z.data, createdBy: by,
	})
	return z, err
}

// readObject reads a whole stored object.
func (s *Service) readObject(ctx context.Context, o storedObject) ([]byte, error) {
	body, err := s.storage.Get(ctx, o.bucket, o.object)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	const limit = maxResponseBytes * 10
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err == nil && len(data) > limit {
		err = fmt.Errorf("a stored document is larger than %d bytes", limit)
	}
	return data, err
}

// lastReviewer is the Team Lead who submitted the case's last Pre-Screening
// Review (Sesuai or Retry): the recorder of what the Pre-Screening Check
// makes (agreed for ticket 10).
func lastReviewer(ctx context.Context, q rowQuerier, caseID string) (string, error) {
	var by *string
	if err := q.QueryRow(ctx, `
		SELECT worked_by_user_id FROM fos.assignments
		WHERE case_id = $1 AND assignment_name = $2 AND assignment_status = $3
		ORDER BY completed_at DESC NULLS LAST, assignment_id DESC LIMIT 1`,
		caseID, AssignmentPreScreeningReview, string(AssignmentCompleted)).Scan(&by); err != nil {
		return "", fmt.Errorf("Pre-Screening Check recorder: %w", err)
	}
	if value(by) == "" {
		return "", errors.New("Pre-Screening Check recorder: the last Pre Screening assignment has no worker")
	}
	return value(by), nil
}
