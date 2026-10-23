package app_test

import (
	"context"
	"reflect"
	"testing"

	"fos/internal/apptest"
)

// AC-FR-NAV-003-04, AC-FR-NAV-003-05, AC-FR-NAV-003-01/02: only AO and SAO
// may create cases (DEC-029, DEC-030), and the only parent case type is
// Financing; child case types such as Phone Verification are never offered.
func TestCreatableCaseTypesFollowTheRole(t *testing.T) {
	h := apptest.New(t)
	financing := map[string]any{"code": "Loan Financing", "label": "Financing"}
	for role, want := range map[string][]any{
		"AO":  {financing},
		"SAO": {financing},
		"TL":  {},
		"BM":  {},
		"FAO": {},
	} {
		me := loggedIn(t, h, apptest.Operator{UserID: "U-" + role, RoleCode: role}).Me(t)
		got, ok := me.Body["caseTypes"].([]any)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("role %s: caseTypes = %v, want %v", role, me.Body["caseTypes"], want)
		}
	}
}

// The Create rule follows the stable role_code (§9.3), not the editable role
// name: renaming the AO role keeps Create, and a role merely named "SAO" does
// not gain it.
func TestCreateFollowsRoleCodeNotName(t *testing.T) {
	h := apptest.New(t)
	ctx := context.Background()
	if _, err := h.DB.Exec(ctx, `UPDATE fos.roles SET role_name = 'Account Officer' WHERE role_code = 'AO'`); err != nil {
		t.Fatal(err)
	}
	if _, err := h.DB.Exec(ctx, `INSERT INTO fos.roles (role_code, role_name, is_active, created_at, updated_at)
		VALUES ('SAO-COPY', 'SAO', TRUE, now(), now())`); err != nil {
		t.Fatal(err)
	}
	financing := map[string]any{"code": "Loan Financing", "label": "Financing"}
	for role, want := range map[string][]any{"AO": {financing}, "SAO-COPY": {}} {
		me := loggedIn(t, h, apptest.Operator{UserID: "U-" + role, RoleCode: role}).Me(t)
		if got, _ := me.Body["caseTypes"].([]any); !reflect.DeepEqual(got, want) {
			t.Fatalf("role code %s: caseTypes = %v, want %v", role, me.Body["caseTypes"], want)
		}
	}
}
