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

// generatedDoc is a file to store in a slot. originalName, when set, is
// its original_file_name (a copy keeps its source's); else fileName.
type generatedDoc struct {
	category, docType, fileName, contentType string
	originalName                             string
	data                                     []byte
	createdBy                                string
}

// storedDoc is a stored generated document: its row and object, and the
// object it replaced (to delete after the commit).
type storedDoc struct {
	caseID, docType string
	fileID          uuid.UUID
	object          storedObject
	replaced        *storedObject
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
	return storedDoc{caseID: caseID, docType: d.docType, fileID: id, object: storedObject{s.storage.Bucket(), object}}, nil
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
	_, err = s.insertDocument(ctx, tx, caseID, d, *sd, false)
	return err
}

// insertDocument adds the stored file's Case Document row; with onlyIfEmpty
// it is added only while the slot has no row, and inserted tells whether it
// was.
func (s *Service) insertDocument(ctx context.Context, q rowQuerier, caseID string, d generatedDoc, sd storedDoc, onlyIfEmpty bool) (inserted bool, err error) {
	// The stored name keeps "/" out of the file name (FR-PSAPI-009 step 3).
	name := strings.ReplaceAll(d.fileName, "/", "-")
	original := d.originalName
	if original == "" {
		original = name
	}
	var id uuid.UUID
	err = q.QueryRow(ctx, `
		INSERT INTO fos.case_document (file_id, case_id, document_category, document_type, original_file_name, file_name,
			bucket_name, object_name, content_type, file_size_bytes, created_at, created_by)
		SELECT $1, $2::varchar, $3::varchar, $4::varchar, $5, $6, $7, $8, $9, $10, $11, $12
		WHERE NOT $13 OR NOT EXISTS (SELECT 1 FROM fos.case_document
		                             WHERE case_id = $2::varchar AND document_category = $3::varchar AND document_type = $4::varchar)
		RETURNING file_id`,
		sd.fileID, caseID, d.category, d.docType, original, name, sd.object.bucket, sd.object.object, d.contentType, len(d.data),
		s.now(), d.createdBy, onlyIfEmpty).Scan(&id)
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, err
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
		// A leftover object is TBD-031e; the case data is consistent. The
		// object name holds personal data (a NIK) and is not logged (DB-RULE-110).
		slog.Error("deleting a generated document object failed", "case_id", sd.caseID, "document_type", sd.docType)
	}
}

// storeGenerated stores a generated document in its slot in its own
// transaction.
func (s *Service) storeGenerated(ctx context.Context, caseID string, d generatedDoc) (uuid.UUID, error) {
	sd, err := s.putGenerated(ctx, caseID, d)
	if err != nil {
		return uuid.Nil, err
	}
	committed := false
	defer func() { s.settleGenerated(context.WithoutCancel(ctx), sd, committed) }()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	if err := s.slotGenerated(ctx, tx, caseID, d, &sd); err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	committed = true
	return sd.fileID, nil
}
