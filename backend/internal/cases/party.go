package cases

import (
	"context"
	"errors"
	"strings"
)

// Reads of the case's parties shared by the Pre-Screening Check steps.

var errIdentityMissing = errors.New("customer identity missing: NIK or NPWP is empty")

// caseCustomerType is the case's Tipe Nasabah (BMV CUSTOMER_TYPE).
func caseCustomerType(ctx context.Context, q rowQuerier, caseID string) (string, error) {
	var customerType *string
	err := q.QueryRow(ctx, `SELECT customer_type FROM fos.work WHERE case_id = $1`, caseID).Scan(&customerType)
	return value(customerType), err
}

// customerParty is the customer row's customer_type for a Tipe Nasabah:
// Nasabah for an Individual, Badan Usaha for a Business Entity (§9.28).
func customerParty(customerType string) string {
	if customerType == customerTypeBusinessEntity {
		return customerTypeBusinessEntity
	}
	return partyCustomer
}

// customerIdentity is the case's Tipe Nasabah and the Customer's identity
// number: the NIK of an Individual, the NPWP of a Business Entity.
func customerIdentity(ctx context.Context, q rowQuerier, caseID string) (string, string, error) {
	customerType, err := caseCustomerType(ctx, q, caseID)
	if err != nil {
		return "", "", err
	}
	column := "id_number"
	if customerType == customerTypeBusinessEntity {
		column = "npwp"
	}
	var id *string
	err = q.QueryRow(ctx, `SELECT `+column+` FROM fos.customer WHERE case_id = $1 AND customer_type = $2`,
		caseID, customerParty(customerType)).Scan(&id)
	if isNotFound(err) || (err == nil && strings.TrimSpace(value(id)) == "") {
		return customerType, "", errIdentityMissing
	}
	return customerType, value(id), err
}
