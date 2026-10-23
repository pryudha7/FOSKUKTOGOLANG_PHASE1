package cases

import (
	"context"
	"log/slog"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
)

// listPreScreening is the list_source of the Underlying Document list on
// UI-011 (§9.11).
const listPreScreening = "PRESCREENING"

// UnderlyingPage is what UI-011 shows (FR-FILE-003, FR-FILE-005, FR-APP-008).
type UnderlyingPage struct {
	CustomerType string
	// MemoNumber is the Nomor Memo (FR-APP-008); empty until it can be formed.
	MemoNumber string
	// SLIK is SLIK Request Data, shown for Perorangan only.
	SLIK *SLIKSection
	// TotalLimitMB is the total upload limit of the Tipe Nasabah (FR-FILE-004).
	TotalLimitMB int
	Categories   []DocumentCategory
	// ListFailed: initialising the list failed (ERR-027); the page shows what
	// the case already had.
	ListFailed bool
	Editable   bool
}

// DocumentCategory is one accordion: Code is the codename stored in
// document_category, Label is shown (DB-RULE-087).
type DocumentCategory struct {
	Code      string
	Label     string
	Documents []RequiredDocument
}

// RequiredDocument is one row of a category; File is its stored file, if any.
type RequiredDocument struct {
	DocumentType string
	Mandatory    bool
	File         *StoredFile
}

// StoredFile identifies a Case Document row.
type StoredFile struct {
	FileID   string
	FileName string
}

// totalLimitMB is FR-FILE-004 rule 1; the SoT leaves the mechanism free
// (DEC-043).
var totalLimitMB = map[string]int{customerTypeIndividual: 10, customerTypeBusinessEntity: 100}

// categoryCode is the codename of a category label: the label without
// spaces or punctuation, e.g. "Dokumen Pre-Screening" → "DokumenPreScreening"
// (DB-RULE-087; agreed for TBD-052).
func categoryCode(label string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, label)
}

// underlyingPage builds UI-011. Only the owner's opening of an unfinished
// assignment initialises the list; any other look changes nothing.
func (s *Service) underlyingPage(ctx context.Context, userID, caseID string, editable bool) (UnderlyingPage, error) {
	page := UnderlyingPage{Editable: editable}
	var customerType, memo *string
	if err := s.db.QueryRow(ctx, `SELECT customer_type, memo_number FROM fos.work WHERE case_id = $1`, caseID).
		Scan(&customerType, &memo); err != nil {
		return UnderlyingPage{}, err
	}
	page.CustomerType = value(customerType)
	page.MemoNumber = value(memo)
	page.TotalLimitMB = totalLimitMB[page.CustomerType]
	if editable {
		if page.MemoNumber == "" {
			var err error
			if page.MemoNumber, err = s.formMemoNumber(ctx, userID, caseID); err != nil {
				return UnderlyingPage{}, err
			}
		}
		if err := s.initDocuments(ctx, caseID, page.CustomerType); err != nil {
			slog.Error("document list initialisation failed (ERR-027)", "case_id", caseID, "error", err)
			page.ListFailed = true
		}
	}
	categories, err := s.documentList(ctx, caseID)
	if err != nil {
		slog.Error("reading the document list failed (ERR-027)", "case_id", caseID, "error", err)
		page.ListFailed = true
		categories = []DocumentCategory{}
	}
	page.Categories = categories
	if page.CustomerType == customerTypeIndividual {
		sec, err := s.slikSection(ctx, caseID)
		if err != nil {
			return UnderlyingPage{}, err
		}
		page.SLIK = &sec
	}
	return page, nil
}

// formMemoNumber forms and stores the case's Nomor Memo,
// <Branch Code>/<UserID>/<yyyyMMddHHmmss in the operator's timezone>, unless
// another opening stored one first (FR-APP-008). Without a Branch Code none
// is formed; the next opening tries again (agreed for TBD-027).
func (s *Service) formMemoNumber(ctx context.Context, userID, caseID string) (string, error) {
	var branch *string
	var timezone string
	err := s.db.QueryRow(ctx, `SELECT branch_code, timezone FROM fos.operator WHERE user_id = $1`, userID).
		Scan(&branch, &timezone)
	if isNotFound(err) {
		slog.Error("Nomor Memo not formed: the operator is not in the operator data", "user_id", userID, "case_id", caseID)
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value(branch)) == "" {
		slog.Error("Nomor Memo not formed: the operator has no Branch Code", "user_id", userID, "case_id", caseID)
		return "", nil
	}
	loc, err := operatorLocation(timezone)
	if err != nil {
		return "", err
	}
	memo := *branch + "/" + userID + "/" + s.now().In(loc).Format("20060102150405")
	err = s.db.QueryRow(ctx, `
		UPDATE fos.work SET memo_number = COALESCE(memo_number, $2) WHERE case_id = $1 RETURNING memo_number`,
		caseID, memo).Scan(&memo)
	return memo, err
}

// initDocuments copies the master document list onto the case, adding only
// what is missing (FR-FILE-005, DB-RULE-034). It is safe to run again or
// concurrently (DB-RULE-033).
func (s *Service) initDocuments(ctx context.Context, caseID, customerType string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	now := s.now()

	// Query 1: the Active categories of the Tipe Nasabah (DB-RULE-092: blank is Active).
	type category struct {
		code, label string
		sortOrder   int
	}
	rows, err := tx.Query(ctx, `
		SELECT code, description, sort_order FROM fos.business_mapping_value
		WHERE type = 'DOCUMENTPRESCREENING' AND sub_type = $1 AND (status = 'Active' OR COALESCE(status, '') = '')
		ORDER BY sort_order, code`, customerType)
	if err != nil {
		return err
	}
	cats, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (category, error) {
		var c category
		return c, r.Scan(&c.code, &c.label, &c.sortOrder)
	})
	if err != nil {
		return err
	}
	for _, c := range cats {
		code := categoryCode(c.label)
		if _, err := tx.Exec(ctx, `
			INSERT INTO fos.case_document_category (case_id, document_category, sort_order, source_code, list_source, created_at)
			VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (case_id, document_category) DO NOTHING`,
			caseID, code, c.sortOrder, c.code, listPreScreening, now); err != nil {
			return err
		}
		// Query 2 runs only while the category has no documents on the case.
		if _, err := tx.Exec(ctx, `
			INSERT INTO fos.case_document_requirement (case_id, document_category, document_type, notes, sort_order,
				is_mandatory, list_source, source_row_uuid, created_at)
			SELECT $1::varchar, $2::varchar, m.description, m.notes, m.sort_order, m.is_mandatory, $5, m.row_uuid, $6
			FROM fos.document_mapping_value m
			WHERE m.category = $3 AND m.sub_category = $4
			  AND NOT EXISTS (SELECT 1 FROM fos.case_document_requirement r WHERE r.case_id = $1 AND r.document_category = $2)
			ON CONFLICT (case_id, document_category, document_type) DO NOTHING`,
			caseID, code, c.label, customerType, listPreScreening, now); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// documentList reads the case's Underlying Document list (UI-011) with the
// stored file of each row (FR-FILE-003 rule 1).
func (s *Service) documentList(ctx context.Context, caseID string) ([]DocumentCategory, error) {
	type row struct {
		code, label, docType string
		mandatory            *bool
		fileID, fileName     *string
	}
	rows, err := collect(ctx, s, func(r pgx.Rows) (row, error) {
		var x row
		return x, r.Scan(&x.code, &x.label, &x.docType, &x.mandatory, &x.fileID, &x.fileName)
	}, `
		SELECT c.document_category, COALESCE(b.description, c.document_category),
		       COALESCE(r.document_type, ''), r.is_mandatory, d.file_id::text, d.file_name
		FROM fos.case_document_category c
		JOIN fos.work w ON w.case_id = c.case_id
		LEFT JOIN fos.business_mapping_value b
		  ON b.type = 'DOCUMENTPRESCREENING' AND b.sub_type = w.customer_type AND b.code = c.source_code
		LEFT JOIN fos.case_document_requirement r
		  ON r.case_id = c.case_id AND r.document_category = c.document_category
		LEFT JOIN fos.case_document d
		  ON d.case_id = c.case_id AND d.document_category = r.document_category AND d.document_type = r.document_type
		WHERE c.case_id = $1 AND c.list_source = $2
		ORDER BY c.sort_order, c.document_category, r.sort_order, r.document_type`, caseID, listPreScreening)
	if err != nil {
		return nil, err
	}
	out := []DocumentCategory{}
	for _, x := range rows {
		if len(out) == 0 || out[len(out)-1].Code != x.code {
			out = append(out, DocumentCategory{Code: x.code, Label: x.label, Documents: []RequiredDocument{}})
		}
		if x.mandatory == nil {
			continue // a category without documents yet
		}
		doc := RequiredDocument{DocumentType: x.docType, Mandatory: *x.mandatory}
		if x.fileID != nil {
			doc.File = &StoredFile{FileID: *x.fileID, FileName: value(x.fileName)}
		}
		cat := &out[len(out)-1]
		cat.Documents = append(cat.Documents, doc)
	}
	return out, nil
}
