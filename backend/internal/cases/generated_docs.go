package cases

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Documents the system makes for a case (SLIK ZIPs, DB-RULE-064) live in a
// slot (category, document type) of the Case Document table: a new one
// replaces the old with DB-RULE-031's order — upload, swap the row in one
// transaction, then delete the old object.

// generatedDoc is a file to store in a slot.
type generatedDoc struct {
	category, docType, fileName, contentType string
	data                                     []byte
	createdBy                                string
}

// storedDoc is a stored generated document: its row and object, and the
// object it replaced (to delete after the commit).
type storedDoc struct {
	fileID   uuid.UUID
	object   storedObject
	replaced *storedObject
}

// putGenerated uploads the file to GCS (DB-RULE-025: before any row).
func (s *Service) putGenerated(ctx context.Context, caseID string, d generatedDoc) (storedDoc, error) {
	if err := s.storageConfigured(); err != nil {
		return storedDoc{}, err
	}
	id := uuid.New()
	object := "LoanFinancing/" + caseID + "/" + d.category + "/" + id.String() + "-" + strings.ReplaceAll(d.fileName, "/", "-")
	if err := s.storage.Put(ctx, object, d.contentType, d.data); err != nil {
		return storedDoc{}, err
	}
	return storedDoc{fileID: id, object: storedObject{s.storage.Bucket(), object}}, nil
}

// slotGenerated replaces the slot's row with the uploaded file inside tx.
func (s *Service) slotGenerated(ctx context.Context, tx pgx.Tx, caseID string, d generatedDoc, sd *storedDoc) error {
	var old storedObject
	err := tx.QueryRow(ctx, `
		DELETE FROM fos.case_document WHERE case_id = $1 AND document_category = $2 AND document_type = $3
		RETURNING bucket_name, object_name`, caseID, d.category, d.docType).Scan(&old.bucket, &old.object)
	switch {
	case err == nil:
		sd.replaced = &old
	case !isNotFound(err):
		return err
	}
	// The stored name keeps "/" out of the file name (FR-PSAPI-009 step 3).
	name := strings.ReplaceAll(d.fileName, "/", "-")
	_, err = tx.Exec(ctx, `
		INSERT INTO fos.case_document (file_id, case_id, document_category, document_type, original_file_name, file_name,
			bucket_name, object_name, content_type, file_size_bytes, created_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $5, $6, $7, $8, $9, $10, $11)`,
		sd.fileID, caseID, d.category, d.docType, name, sd.object.bucket, sd.object.object, d.contentType, len(d.data), s.now(), d.createdBy)
	return err
}

// settleGenerated deletes, after the transaction, the replaced object when
// it committed, or the new one when it did not.
func (s *Service) settleGenerated(ctx context.Context, sd storedDoc, committed bool) {
	gone := sd.replaced
	if !committed {
		gone = &sd.object
	}
	if gone == nil {
		return
	}
	if err := s.storage.Delete(ctx, gone.bucket, gone.object); err != nil {
		// A leftover object is TBD-031e; the case data is consistent.
		slog.Error("deleting a generated document object failed", "object", gone.object, "error", err)
	}
}

// storeGenerated stores a generated document in its slot in its own
// transaction.
func (s *Service) storeGenerated(ctx context.Context, caseID string, d generatedDoc) (uuid.UUID, error) {
	sd, err := s.putGenerated(ctx, caseID, d)
	if err != nil {
		return uuid.Nil, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.settleGenerated(ctx, sd, false)
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	if err := s.slotGenerated(ctx, tx, caseID, d, &sd); err != nil {
		s.settleGenerated(ctx, sd, false)
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		s.settleGenerated(ctx, sd, false)
		return uuid.Nil, err
	}
	s.settleGenerated(ctx, sd, true)
	return sd.fileID, nil
}
