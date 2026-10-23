package cases

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"fos/internal/auth"
	"fos/internal/settings"
)

// Storage is where case files live (FR-FILE-001): GCS in production.
type Storage interface {
	Bucket() string
	// Missing names the empty configuration keys (ENV-010).
	Missing() []string
	// Put stores an object in the configured bucket.
	Put(ctx context.Context, object, contentType string, body []byte) error
	// Get and Delete address an object by the bucket its reference names
	// (DB-RULE-023).
	Get(ctx context.Context, bucket, object string) (io.ReadCloser, error)
	Delete(ctx context.Context, bucket, object string) error
}

var (
	// ErrStorageFailed is ERR-025: storage is unconfigured or unreachable, or
	// the file's reference could not be saved.
	ErrStorageFailed = errors.New("storage failed")
)

// Underlying Document messages (§12). The SoT leaves ERR-021, ERR-024,
// ERR-025 and ERR-027 TBD and proposes ERR-041; these are the texts agreed
// for ticket 07.
const (
	SectionUpload = "upload"
	KeyFile       = "file"

	msgEmptyName = "Harap tuliskan nama file sebelum mengupload dokumen."
	msgLongName  = "Nama file maksimal 100 karakter."
	msgFileType  = "Hanya file yang diperbolehkan: gambar, pdf, excel, rar, dan zip"
	msgFileSize  = "Ukuran file melebihi batas maksimum"

	MsgStorageFailed = "Upload dokumen gagal. Silakan coba lagi."        // ERR-025
	MsgListFailed    = "Daftar dokumen gagal dimuat. Silakan coba lagi." // ERR-027
)

// maxNameRunes bounds Name, without the extension (FR-FILE-002 rule 3).
const maxNameRunes = 100

// allowedExtensions are gambar, pdf, excel, rar and zip (agreed for TBD-027).
var allowedExtensions = []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".pdf", ".xls", ".xlsx", ".csv", ".rar", ".zip"}

// UploadRequest is one Submit of the Upload Document modal (UI-015).
type UploadRequest struct {
	Category, DocumentType string
	// Name is the modal's Name; the stored name adds the original extension.
	Name             string
	OriginalFileName string
	ContentType      string
	Data             []byte
	// TooLarge: the request exceeded MaxFileBytes while it was read, so the
	// file was not read in full (ERR-041).
	TooLarge bool
}

// MaxFileBytes is the per-file limit (DSS validation/upload/max_file_bytes).
func (s *Service) MaxFileBytes(ctx context.Context) (int64, error) {
	return s.settings.Int(ctx, settings.MaxFileBytes)
}

// UploadDocument stores an Underlying Document for a row of the UI-011
// list: the file goes to GCS first, then its reference replaces any earlier
// one of the same category and type in one transaction, then the replaced
// object is deleted (FR-FILE-001, FR-FILE-002, DB-RULE-025, DB-RULE-031).
func (s *Service) UploadDocument(ctx context.Context, sess auth.Session, caseID string, assignmentID int64, u UploadRequest) (StoredFile, error) {
	if err := s.checkUploadTarget(ctx, sess, caseID, assignmentID, u); err != nil {
		return StoredFile{}, err
	}
	name := strings.TrimSpace(u.Name)
	ext := strings.ToLower(path.Ext(u.OriginalFileName))
	var errs []FieldError
	switch {
	case name == "":
		errs = append(errs, FieldError{SectionUpload, KeyName, "Name", "ERR-023", msgEmptyName})
	case utf8.RuneCountInString(name) > maxNameRunes:
		errs = append(errs, FieldError{SectionUpload, KeyName, "Name", "ERR-024", msgLongName})
	}
	// A request cut short before its file part has no file type to judge.
	if (u.OriginalFileName != "" || !u.TooLarge) && !slices.Contains(allowedExtensions, ext) {
		errs = append(errs, FieldError{SectionUpload, KeyFile, "File", "ERR-021", msgFileType})
	}
	if u.TooLarge {
		errs = append(errs, FieldError{SectionUpload, KeyFile, "File", "ERR-041", msgFileSize})
	}
	if len(errs) > 0 {
		return StoredFile{}, &ValidationError{errs}
	}
	if err := s.storageConfigured(); err != nil {
		return StoredFile{}, err
	}

	fileID := uuid.New()
	// The original extension is kept as uploaded; Name cannot change it.
	fileName := name + path.Ext(u.OriginalFileName)
	object := "LoanFinancing/" + caseID + "/" + u.Category + "/" + fileID.String() + "-" + strings.ReplaceAll(fileName, "/", "-")
	contentType := u.ContentType
	if contentType == "" || contentType == "application/octet-stream" {
		if t := mime.TypeByExtension(ext); t != "" {
			contentType = t
		}
	}
	if err := s.storage.Put(ctx, object, contentType, u.Data); err != nil {
		slog.Error("upload to GCS failed", "case_id", caseID, "error", err)
		return StoredFile{}, ErrStorageFailed
	}
	replaced, err := s.saveReference(ctx, sess, caseID, assignmentID, caseDocument{
		fileID: fileID, category: u.Category, docType: u.DocumentType, original: u.OriginalFileName,
		fileName: fileName, object: object, contentType: contentType, size: int64(len(u.Data)),
	})
	if err != nil {
		slog.Error("saving the case document reference failed", "case_id", caseID, "error", err)
		if derr := s.storage.Delete(ctx, s.storage.Bucket(), object); derr != nil {
			slog.Error("removing the unreferenced object failed", "case_id", caseID, "object", object, "error", derr)
		}
		if errors.Is(err, ErrForbidden) || errors.Is(err, ErrCompleted) || errors.Is(err, ErrNotFound) {
			return StoredFile{}, err
		}
		return StoredFile{}, ErrStorageFailed
	}
	if replaced != nil {
		// A failure here leaves the old object behind (TBD-031e); the case is consistent.
		if err := s.storage.Delete(ctx, replaced.bucket, replaced.object); err != nil {
			slog.Error("deleting the replaced object failed", "case_id", caseID, "object", replaced.object, "error", err)
		}
	}
	return StoredFile{FileID: fileID.String(), FileName: fileName}, nil
}

// checkUploadTarget refuses an upload that is not to a row of the operator's
// own open Document Underlying Upload assignment.
func (s *Service) checkUploadTarget(ctx context.Context, sess auth.Session, caseID string, assignmentID int64, u UploadRequest) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	a, err := ownOpenAssignment(ctx, tx, sess, caseID, assignmentID)
	if err != nil {
		return err
	}
	if a.name != AssignmentUnderlyingDocumentUpload {
		return ErrUnsupported
	}
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM fos.case_document_requirement
		WHERE case_id = $1 AND document_category = $2 AND document_type = $3 AND list_source = $4)`,
		caseID, u.Category, u.DocumentType, listPreScreening).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrUnsupported
	}
	return nil
}

type caseDocument struct {
	fileID                                                     uuid.UUID
	category, docType, original, fileName, object, contentType string
	size                                                       int64
}

// storedObject is where a reference points in GCS.
type storedObject struct{ bucket, object string }

// storageConfigured refuses storage use while a GCS key is empty, logging
// the keys' names only (ENV-010).
func (s *Service) storageConfigured() error {
	if missing := s.storage.Missing(); len(missing) > 0 {
		slog.Error("GCS configuration incomplete (ENV-010)", "missing", strings.Join(missing, ","))
		return ErrStorageFailed
	}
	return nil
}

// saveReference replaces the reference of the category and type with d,
// returning the replaced object, if any.
func (s *Service) saveReference(ctx context.Context, sess auth.Session, caseID string, assignmentID int64, d caseDocument) (*storedObject, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// The assignment may have moved on while the file was uploading.
	if _, err := ownOpenAssignment(ctx, tx, sess, caseID, assignmentID); err != nil {
		return nil, err
	}
	var replaced storedObject
	err = tx.QueryRow(ctx, `
		DELETE FROM fos.case_document WHERE case_id = $1 AND document_category = $2 AND document_type = $3
		RETURNING bucket_name, object_name`, caseID, d.category, d.docType).Scan(&replaced.bucket, &replaced.object)
	found := err == nil
	if err != nil && !isNotFound(err) {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fos.case_document (file_id, case_id, document_category, document_type, original_file_name, file_name,
			bucket_name, object_name, content_type, file_size_bytes, created_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		d.fileID, caseID, d.category, d.docType, d.original, d.fileName, s.storage.Bucket(), d.object,
		d.contentType, d.size, s.now(), sess.UserID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil || !found {
		return nil, err
	}
	return &replaced, nil
}
