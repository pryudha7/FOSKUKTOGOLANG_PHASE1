package cases

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/google/uuid"

	"fos/internal/auth"
)

var (
	// ErrNoFileAccess is ERR-035: the operator may not open the case's files.
	ErrNoFileAccess = errors.New("no access to the case's files")
	// ErrDownloadFailed: the file could not be fetched from GCS (TBD-037e).
	ErrDownloadFailed = errors.New("download failed")
)

// Download is a case file on its way to the browser.
type Download struct {
	FileName    string
	ContentType string
	Body        io.ReadCloser
}

// OpenDocument fetches a case file through the server (FR-FILE-003,
// DEC-081). Access (DEC-082, agreed interim): the case's creator, or an
// operator holding an unfinished assignment of the case.
func (s *Service) OpenDocument(ctx context.Context, sess auth.Session, caseID, fileID string) (Download, error) {
	id, err := uuid.Parse(fileID)
	if err != nil {
		return Download{}, ErrNotFound
	}
	var (
		d       Download
		stored  storedObject
		ctype   *string
		allowed bool
	)
	err = s.db.QueryRow(ctx, `
		SELECT d.file_name, d.content_type, d.bucket_name, d.object_name,
		       w.created_by = $3 OR EXISTS (
		           SELECT 1 FROM fos.assignments a
		           WHERE a.case_id = w.case_id AND a.assigned_user_id = $3 AND a.assignment_status <> $4)
		FROM fos.case_document d JOIN fos.work w ON w.case_id = d.case_id
		WHERE d.case_id = $1 AND d.file_id = $2`, caseID, id, sess.UserID, string(AssignmentCompleted)).
		Scan(&d.FileName, &ctype, &stored.bucket, &stored.object, &allowed)
	if isNotFound(err) {
		return Download{}, ErrNotFound
	}
	if err != nil {
		return Download{}, err
	}
	if !allowed {
		slog.Warn("file access refused (ERR-035)", "user_id", sess.UserID, "case_id", caseID, "file_id", fileID)
		return Download{}, ErrNoFileAccess
	}
	if err := s.storageConfigured(); err != nil {
		return Download{}, err
	}
	d.ContentType = value(ctype)
	if d.Body, err = s.storage.Get(ctx, stored.bucket, stored.object); err != nil {
		slog.Error("fetching the file from GCS failed", "case_id", caseID, "file_id", fileID, "error", err)
		return Download{}, ErrDownloadFailed
	}
	return d, nil
}
