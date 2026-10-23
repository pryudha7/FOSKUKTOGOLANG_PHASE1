package cases

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// The Rejected List (FR-PSAPI-007, §9.16) holds Customers and Spouses that
// had a Check Rejection. Its check is not an API: nothing goes to the
// Connect API Log.

const (
	stepRejectedList = "Rejected List"

	rejectedActive = "ACTIVE"
)

func (e *Executor) rejectedListCheck() check {
	return check{
		try: e.tryRejectedList,
		// Never retried: a failed query stops the run (FR-PSAPI-007).
		retry: func(context.Context) (int, time.Duration, error) { return 0, 0, nil },
	}
}

// tryRejectedList matches the Customer's identity number and Customer Type
// against ACTIVE rows (DB-RULE-049). A failed query is a system failure,
// never a pass (fail-closed); the run stops broken, case_status unchanged.
func (e *Executor) tryRejectedList(ctx context.Context, caseID string, _ int) (attempt, error) {
	timeoutMS, err := e.s.settings.Int(ctx, "process/rejectedlist/query_timeout_ms")
	if err != nil {
		return attempt{}, err
	}
	qctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	customerType, identity, err := customerIdentity(qctx, e.s.db, caseID)
	if err != nil {
		return attempt{}, fmt.Errorf("Rejected List check: %w", err)
	}
	var listed bool
	if err := e.s.db.QueryRow(qctx, `
		SELECT EXISTS (SELECT 1 FROM fos.rejected_list WHERE id_number = $1 AND customer_type = $2 AND status = $3)`,
		identity, customerType, rejectedActive).Scan(&listed); err != nil {
		if ctx.Err() != nil {
			return attempt{}, ctx.Err()
		}
		return attempt{}, fmt.Errorf("Rejected List check: %w", err)
	}
	if listed {
		return attempt{outcome: outcomeRejected}, nil
	}
	return attempt{outcome: outcomeSuccess}, nil
}

// rejectedParty is one row to record.
type rejectedParty struct {
	idNumber, fullName, customerType string
	birthDate                        *time.Time
}

// recordRejection puts the case's Customer and Spouse on the Rejected List
// after a Check Rejection by api (FR-PSAPI-007 Pencatatan, DB-RULE-050), in
// the reject's transaction: a new row, or the existing (id_number,
// customer_type) row updated and made ACTIVE again.
func (e *Executor) recordRejection(ctx context.Context, tx pgx.Tx, caseID, api string) error {
	var customerType *string
	if err := tx.QueryRow(ctx, `SELECT customer_type FROM fos.work WHERE case_id = $1`, caseID).Scan(&customerType); err != nil {
		return err
	}
	// Recorded by the Team Lead who submitted Sesuai or Retry (agreed).
	var by *string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(worked_by_user_id, assigned_user_id) FROM fos.assignments
		WHERE case_id = $1 AND assignment_name = $2 AND assignment_status = $3
		ORDER BY completed_at DESC NULLS LAST, assignment_id DESC LIMIT 1`,
		caseID, AssignmentPreScreeningReview, string(AssignmentCompleted)).Scan(&by); err != nil {
		return fmt.Errorf("Rejected List recorder: %w", err)
	}

	parties, err := collectIn(ctx, tx, func(r pgx.Rows) (rejectedParty, error) {
		var (
			p         rejectedParty
			id, name  *string
			birthDate *time.Time
		)
		err := r.Scan(&p.customerType, &id, &name, &birthDate)
		p.idNumber, p.fullName, p.birthDate = value(id), value(name), birthDate
		return p, err
	}, `
		SELECT CASE WHEN customer_type = $3 THEN $4 ELSE $2 END,
		       CASE WHEN customer_type = $5 THEN npwp ELSE id_number END,
		       full_name,
		       CASE WHEN customer_type = $5 THEN establishment_date ELSE birth_date END
		FROM fos.customer
		WHERE case_id = $1 AND customer_type IN ($6, $5, $3)
		ORDER BY customer_type = $3`,
		caseID, value(customerType), partySpouse, customerTypeIndividual, customerTypeBusinessEntity, partyCustomer)
	if err != nil {
		return err
	}
	reason := fmt.Sprintf("• Reject Pre-screening %q", api)
	now := e.s.now()
	for _, p := range parties {
		if p.idNumber == "" || p.fullName == "" || p.birthDate == nil {
			return fmt.Errorf("Rejected List entry incomplete for a %s party of case %s", p.customerType, caseID)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO fos.rejected_list (rejected_id, id_number, full_name, birth_date, status, customer_type, reason,
				created_at, created_by, updated_at, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $8, $9)
			ON CONFLICT (id_number, customer_type) DO UPDATE SET full_name = EXCLUDED.full_name, birth_date = EXCLUDED.birth_date,
				reason = EXCLUDED.reason, status = EXCLUDED.status, updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.updated_by`,
			uuid.New(), p.idNumber, p.fullName, *p.birthDate, rejectedActive, p.customerType, reason, now, value(by)); err != nil {
			return err
		}
	}
	return nil
}
