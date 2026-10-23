package app_test

import (
	"reflect"
	"testing"

	"fos/internal/apptest"
)

// AC-FR-NAV-003-04, AC-FR-NAV-003-05, AC-FR-NAV-003-01/02: only AO and SAO
// may create cases (DEC-029, DEC-030), and the only parent case type is
// Financing; child case types such as Phone Verification are never offered.
func TestCreatableCaseTypesFollowTheRole(t *testing.T) {
	h := apptest.New(t)
	for role, want := range map[string][]any{
		"AO":  {"Financing"},
		"SAO": {"Financing"},
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
