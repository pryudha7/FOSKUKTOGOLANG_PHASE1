package cases

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"fos/internal/auth"
)

// AssignmentPreScreeningReview is the Team Lead's assignment after Create,
// named "Pre Screening" in the SoT (FR-APP-009; GLOSSARY Pre-Screening Review).
const AssignmentPreScreeningReview = "Pre Screening"

// UI-011 sections and keys of a FieldError on Create.
const (
	SectionSLIK      = "slik"
	SectionDocuments = "documents" // key "<category>/<document type>"

	KeyRequestPurpose = "requestPurpose"
	KeyFacilityOption = "facilityOption"
	KeyAddress        = "address"
	KeySearchType     = "searchType"
	KeySegmentation   = "segmentation"
	KeyTotalSize      = "totalSize"
	KeyTeamLead       = "teamLead"
)

// Create messages (§12). ERR-029 and the missing Team Lead text are agreed
// interim texts (TBD-039, TBD-029).
const (
	msgMandatoryDocument = "Harap upload dokumen wajib"
	msgAddressLength     = "Alamat Nasabah maksimal 350 karakter"
	msgTeamLeadMissing   = "Team Lead case tidak ditemukan"
)

// maxAddressRunes bounds Alamat Nasabah, counted in characters after CRLF
// becomes LF, as the browser counts (UI-011, DEC-050).
const maxAddressRunes = 350

// bytesPerMB is FR-FILE-004's MB: 1024 x 1024 bytes.
const bytesPerMB = 1024 * 1024

// totalMB shows a byte total in MB with two decimals, rounded up so a total
// over the limit never reads as the limit (agreed for TBD-027).
func totalMB(bytes int64) string {
	hundredths := (bytes*100 + bytesPerMB - 1) / bytesPerMB
	return fmt.Sprintf("%d.%02d", hundredths/100, hundredths%100)
}

// CreateUnderlying is Create on Document Underlying Upload (Perorangan): it
// validates SLIK Request Data, the total upload size and the mandatory
// documents (FR-FILE-004), then stores the SLIK Request Data, completes the
// assignment and creates the Team Lead's Pre Screening assignment
// (FR-APP-009), returning that assignment.
func (s *Service) CreateUnderlying(ctx context.Context, sess auth.Session, caseID string, assignmentID int64, req SLIKRequest) (int64, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	a, err := ownOpenAssignment(ctx, tx, sess, caseID, assignmentID)
	if err != nil {
		return 0, err
	}
	if a.name != AssignmentUnderlyingDocumentUpload {
		return 0, ErrUnsupported
	}
	var customerType, teamLead *string
	if err := tx.QueryRow(ctx, `SELECT customer_type, branch_officer_team_lead FROM fos.work WHERE case_id = $1`, caseID).
		Scan(&customerType, &teamLead); err != nil {
		return 0, err
	}
	// Badan Usaha: ticket 31.
	if value(customerType) != customerTypeIndividual {
		return 0, ErrUnsupported
	}

	req = SLIKRequest{
		RequestPurpose: strings.TrimSpace(req.RequestPurpose),
		FacilityOption: strings.TrimSpace(req.FacilityOption),
		Address:        strings.ReplaceAll(strings.TrimSpace(req.Address), "\r\n", "\n"),
		SearchType:     strings.TrimSpace(req.SearchType),
		Segmentation:   strings.TrimSpace(req.Segmentation),
	}
	var errs []FieldError
	for _, f := range []struct {
		key, label, bmvType, code string
	}{
		{KeyRequestPurpose, "Tujuan Permintaan", bmvRequestPurpose, req.RequestPurpose},
		{KeyFacilityOption, "Pilihan Fasilitas", bmvFacilityOption, req.FacilityOption},
		{KeyAddress, "Alamat Nasabah", "", req.Address},
		{KeySearchType, "Tipe Pencarian", bmvSearchType, req.SearchType},
		{KeySegmentation, "Segmentasi", bmvSegmentation, req.Segmentation},
	} {
		switch {
		case f.code == "":
			errs = append(errs, required(SectionSLIK, f.key, f.label))
		case f.bmvType == "":
			if utf8.RuneCountInString(f.code) > maxAddressRunes {
				errs = append(errs, FieldError{SectionSLIK, f.key, f.label, "ERR-029", msgAddressLength})
			}
		default:
			opts, err := optionsIn(ctx, tx, f.bmvType)
			if err != nil {
				return 0, err
			}
			if !hasCode(opts, f.code) {
				return 0, ErrUnsupported // not an option the page offered
			}
		}
	}

	// FR-FILE-004 rule 1: the files on this screen, i.e. of its categories.
	var total int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(sum(d.file_size_bytes), 0) FROM fos.case_document d
		JOIN fos.case_document_category c ON c.case_id = d.case_id AND c.document_category = d.document_category
		WHERE d.case_id = $1 AND c.list_source = $2`, caseID, listPreScreening).Scan(&total); err != nil {
		return 0, err
	}
	limit := totalLimitMB[customerTypeIndividual]
	if total > int64(limit)*bytesPerMB {
		errs = append(errs, FieldError{SectionForm, KeyTotalSize, "Notes", "ERR-020",
			fmt.Sprintf("Maksimal total ukuran file upload: %d MB (Total: %s MB)", limit, totalMB(total))})
	}
	missing, err := collectIn(ctx, tx, func(r pgx.Rows) (FieldError, error) {
		var cat, docType string
		err := r.Scan(&cat, &docType)
		return FieldError{SectionDocuments, cat + "/" + docType, docType, "ERR-026", msgMandatoryDocument}, err
	}, `
		SELECT r.document_category, r.document_type FROM fos.case_document_requirement r
		JOIN fos.case_document_category c ON c.case_id = r.case_id AND c.document_category = r.document_category
		WHERE r.case_id = $1 AND r.list_source = $2 AND r.is_mandatory
		  AND NOT EXISTS (SELECT 1 FROM fos.case_document d
		                  WHERE d.case_id = r.case_id AND d.document_category = r.document_category AND d.document_type = r.document_type)
		ORDER BY c.sort_order, r.document_category, r.sort_order, r.document_type`, caseID, listPreScreening)
	if err != nil {
		return 0, err
	}
	errs = append(errs, missing...)
	if len(errs) > 0 {
		return 0, &ValidationError{errs}
	}

	// TBD-029 (agreed): without a known Team Lead nobody could take the case.
	var known bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM fos.operator WHERE user_id = $1)`, value(teamLead)).Scan(&known); err != nil {
		return 0, err
	}
	if !known {
		slog.Error("Create refused: the case's Team Lead is not in the operator data", "case_id", caseID, "team_lead", value(teamLead))
		return 0, &ValidationError{[]FieldError{{SectionForm, KeyTeamLead, "", "", msgTeamLeadMissing}}}
	}

	now := s.now()
	if _, err := tx.Exec(ctx, `
		UPDATE fos.work SET slik_request_purpose_code = $2, slik_facility_option_code = $3, slik_identity_address = $4,
			slik_search_type_code = $5, slik_segmentation_code = $6, case_status = COALESCE(case_status, $7),
			updated_at = $8, updated_by = $9
		WHERE case_id = $1`,
		caseID, req.RequestPurpose, req.FacilityOption, req.Address, req.SearchType, req.Segmentation,
		CaseStatusNew, now, sess.UserID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE fos.assignments SET assignment_status = $2, completed_at = $3 WHERE assignment_id = $1`,
		a.id, string(AssignmentCompleted), now); err != nil {
		return 0, err
	}
	// FR-APP-009 step 2: opened time and worker are set when the Team Lead first opens it.
	var next int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING assignment_id`,
		caseID, AssignmentPreScreeningReview, string(Worklist), value(teamLead), string(AssignmentNew), now).Scan(&next); err != nil {
		return 0, err
	}
	return next, tx.Commit(ctx)
}

func hasCode(opts []Option, code string) bool {
	for _, o := range opts {
		if o.Code == code {
			return true
		}
	}
	return false
}
