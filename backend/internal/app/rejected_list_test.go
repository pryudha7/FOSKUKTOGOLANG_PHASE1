package app_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"fos/internal/apptest"
	"fos/internal/config"
)

type rejectedRow struct {
	ID, Name, Birth, Status, Type, Reason, CreatedBy, UpdatedBy string
	CreatedAt, UpdatedAt                                        time.Time
}

func rejectedList(t *testing.T, h *apptest.Harness) []rejectedRow {
	t.Helper()
	rows, err := h.DB.Query(context.Background(), `
		SELECT id_number, full_name, to_char(birth_date, 'YYYY-MM-DD'), status, customer_type, reason, created_by, updated_by, created_at, updated_at
		FROM fos.rejected_list ORDER BY customer_type, id_number`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []rejectedRow
	for rows.Next() {
		var r rejectedRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Birth, &r.Status, &r.Type, &r.Reason, &r.CreatedBy, &r.UpdatedBy, &r.CreatedAt, &r.UpdatedAt); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func listRejected(t *testing.T, h *apptest.Harness, id, name, birth, status, customerType string) {
	t.Helper()
	exec(t, h, `INSERT INTO fos.rejected_list (rejected_id, id_number, full_name, birth_date, status, customer_type, reason, created_at, created_by, updated_at, updated_by)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, '• Reject Pre-screening "Dukcapil"', '2025-01-01', 'TL01', '2025-01-01', 'TL01')`,
		id, name, birth, status, customerType)
}

func addSpouse(h *apptest.Harness, t *testing.T, caseID string) {
	t.Helper()
	exec(t, h, `INSERT INTO fos.customer (case_id, customer_type, id_number, full_name, birth_date) VALUES ($1, 'Pasangan', '3171019876543210', 'Siti Aminah', '1990-01-01')`, caseID)
}

var dukcapilOK = []apptest.FakeReply{apptest.DukcapilMatch}
var dhnOK = []apptest.FakeReply{apptest.DHNNotFound}

// AC-FR-PSAPI-006-04: a DHN code other than DN-00/DN-01, or no format, is
// a Fail retried up to four attempts, then the Team Lead gets the Retry view.
func TestDHNFailureReturnsToTheTeamLead(t *testing.T) {
	h, _, _, caseID := checksWith(t, dukcapilOK, []apptest.FakeReply{apptest.DHNOtherCode, {Status: 200, Body: "not json"}, apptest.DHNOtherCode}, nil)
	eventually(t, "the run to fail", func() bool { return run(t, h, caseID).State == "STOPPED_FAILED" })
	if got := apiNames(apiLogs(t, h, caseID)); !reflect.DeepEqual(got, []string{"Dukcapil Success", "DHN Fail", "DHN Fail", "DHN Fail", "DHN Fail"}) {
		t.Fatalf("logs = %v", got)
	}
	if run(t, h, caseID).Step != "DHN" || caseStatus(t, h, caseID) != "Failed-PrescreeningConnection" {
		t.Fatalf("run %+v, status %s", run(t, h, caseID), caseStatus(t, h, caseID))
	}
}

// AC-FR-PSAPI-006-02: a Business Entity is screened by its NPWP; Dukcapil
// and the Spouse are not checked.
func TestDHNScreensABusinessEntityByNPWP(t *testing.T) {
	h, dukcapil, dhn, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) {
		exec(t, h, `UPDATE fos.work SET customer_type = 'Badan Usaha' WHERE case_id = $1`, caseID)
		exec(t, h, `INSERT INTO fos.customer (case_id, customer_type, npwp, full_name, establishment_date) VALUES ($1, 'Badan Usaha', '012345678901234', 'PT Maju', '2010-05-01')`, caseID)
		addSpouse(h, t, caseID)
	})
	eventually(t, "the run to reach SLIK", func() bool { return run(t, h, caseID).Step == "SLIK" })
	calls := dhn.Calls()
	if len(calls) != 1 || calls[0].Path != "/v1/customer/dhn/check/012345678901234" || len(dukcapil.Requests()) != 0 {
		t.Fatalf("DHN calls %+v, Dukcapil calls %d", calls, len(dukcapil.Requests()))
	}
}

// AC-FR-PSAPI-007-01, -07: an ACTIVE row with the Customer's identity and
// type rejects at the Rejected List: nothing is logged, the list does not
// change, SLIK never runs.
func TestRejectedListMatchRejectsTheCase(t *testing.T) {
	var before []rejectedRow
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) {
		listRejected(t, h, "3171011234567890", "Budi Lama", "1980-01-01", "ACTIVE", "Perorangan")
		before = rejectedList(t, h)
	})
	eventually(t, "the run to stop", func() bool { return run(t, h, caseID).State != "RUNNING" })
	if got := run(t, h, caseID); got.State != "STOPPED_REJECTED" || got.Step != "Rejected List" {
		t.Fatalf("run = %+v", got)
	}
	if caseStatus(t, h, caseID) != "ResolvedRejected-Prescreening" || currentStage(t, h, caseID) != "Rejected" {
		t.Fatalf("case = %s %s", caseStatus(t, h, caseID), currentStage(t, h, caseID))
	}
	if got := apiNames(apiLogs(t, h, caseID)); !reflect.DeepEqual(got, []string{"Dukcapil Success", "DHN Success"}) {
		t.Fatalf("logs = %v", got)
	}
	if !reflect.DeepEqual(rejectedList(t, h), before) {
		t.Fatalf("list changed: %+v", rejectedList(t, h))
	}
}

// AC-FR-PSAPI-007-02, -03, -09: an INACTIVE row, a listed Spouse only, or
// the same number with another Customer Type all pass.
func TestRejectedListPassesWithoutAnActiveMatch(t *testing.T) {
	for name, seed := range map[string]func(*apptest.Harness, string){
		"inactive": func(h *apptest.Harness, _ string) {
			listRejected(t, h, "3171011234567890", "Budi", "1990-01-01", "INACTIVE", "Perorangan")
		},
		"spouse only": func(h *apptest.Harness, caseID string) {
			addSpouse(h, t, caseID)
			listRejected(t, h, "3171019876543210", "Siti Aminah", "1990-01-01", "ACTIVE", "Perorangan")
		},
		"other customer type": func(h *apptest.Harness, _ string) {
			listRejected(t, h, "3171011234567890", "PT Budi", "2000-01-01", "ACTIVE", "Badan Usaha")
		},
	} {
		t.Run(name, func(t *testing.T) {
			h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, caseID string) { seed(h, caseID) })
			eventually(t, "the run to reach SLIK", func() bool { return run(t, h, caseID).Step == "SLIK" })
		})
	}
}

// AC-FR-PSAPI-007-04: a Dukcapil reject records the Customer and the Spouse,
// ACTIVE, Perorangan, with the reason and the Team Lead as recorder.
func TestCheckRejectionRecordsCustomerAndSpouse(t *testing.T) {
	h, _, dhn, caseID := checksWith(t, []apptest.FakeReply{apptest.DukcapilMismatch}, dhnOK, func(h *apptest.Harness, caseID string) { addSpouse(h, t, caseID) })
	eventually(t, "the run to stop", func() bool { return run(t, h, caseID).State != "RUNNING" })
	got := rejectedList(t, h)
	reason := `• Reject Pre-screening "Dukcapil"`
	if len(got) != 2 || len(dhn.Calls()) != 0 {
		t.Fatalf("rows %+v, DHN calls %d", got, len(dhn.Calls()))
	}
	want := map[string][2]string{"3171011234567890": {"Budi Santoso", yearsAgo(h, 30, 0)}, "3171019876543210": {"Siti Aminah", "1990-01-01"}}
	for _, r := range got {
		w := want[r.ID]
		if r.Name != w[0] || r.Birth != w[1] || r.Status != "ACTIVE" || r.Type != "Perorangan" || r.Reason != reason ||
			r.CreatedBy != "TL01" || r.UpdatedBy != "TL01" {
			t.Fatalf("row = %+v", r)
		}
	}
}

// AC-FR-PSAPI-007-06, -08: a known number is updated, not added: name,
// birth date, reason and update fields from the new case, ACTIVE again;
// created_at and created_by stay.
func TestCheckRejectionUpdatesAnExistingRow(t *testing.T) {
	for _, status := range []string{"ACTIVE", "INACTIVE"} {
		t.Run(status, func(t *testing.T) {
			h, _, _, caseID := checksWith(t, []apptest.FakeReply{apptest.DukcapilMismatch}, dhnOK, func(h *apptest.Harness, _ string) {
				h.SeedOperator(t, apptest.Operator{UserID: "OLD01", Password: "x", RoleCode: "TL"})
				exec(t, h, `INSERT INTO fos.rejected_list (rejected_id, id_number, full_name, birth_date, status, customer_type, reason, notes, created_at, created_by, updated_at, updated_by)
					VALUES (gen_random_uuid(), '3171011234567890', 'Nama Lama', '1970-01-01', $1, 'Perorangan', '• Reject Pre-screening "Pefindo"', 'catatan', '2025-01-01', 'OLD01', '2025-01-01', 'OLD01')`, status)
			})
			eventually(t, "the run to stop", func() bool { return run(t, h, caseID).State != "RUNNING" })
			got := rejectedList(t, h)
			if len(got) != 1 {
				t.Fatalf("rows = %+v", got)
			}
			r := got[0]
			if r.Name != "Budi Santoso" || r.Birth != yearsAgo(h, 30, 0) || r.Reason != `• Reject Pre-screening "Dukcapil"` || r.Status != "ACTIVE" ||
				r.CreatedBy != "OLD01" || !r.CreatedAt.Equal(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)) || r.UpdatedBy != "TL01" {
				t.Fatalf("row = %+v", r)
			}
			if n := count(t, h, `SELECT count(*) FROM fos.rejected_list WHERE notes = 'catatan'`); n != 1 {
				t.Fatal("notes changed")
			}
		})
	}
}

// AC-FR-PSAPI-007-10 (without the Problem Flow, ticket 14): a failing query
// is never a pass: the run stops broken at the Rejected List, case_status
// stays, nothing is logged in the Connect API Log, and the cause is in the
// application log.
func TestRejectedListQueryFailureStopsTheRunBroken(t *testing.T) {
	logs := apptest.CaptureLogs(t)
	h, _, _, caseID := checksWith(t, dukcapilOK, dhnOK, func(h *apptest.Harness, _ string) {
		exec(t, h, `ALTER TABLE fos.rejected_list RENAME TO rejected_list_gone`)
	})
	eventually(t, "the run to stop", func() bool { return run(t, h, caseID).State != "RUNNING" })
	if got := run(t, h, caseID); got.State != "STOPPED_BROKEN" || got.Step != "Rejected List" || caseStatus(t, h, caseID) != "Pending-Prescreening" {
		t.Fatalf("run %+v, status %s", got, caseStatus(t, h, caseID))
	}
	if got := apiNames(apiLogs(t, h, caseID)); !reflect.DeepEqual(got, []string{"Dukcapil Success", "DHN Success"}) {
		t.Fatalf("logs = %v", got)
	}
	if !strings.Contains(logs.String(), "Rejected List check") {
		t.Fatalf("cause not logged: %s", logs)
	}
}

// AC-FR-PSAPI-007-12: a run interrupted at the Rejected List is resumed by
// the sweep from that step; SLIK only after it passes.
func TestRecoveryRepeatsTheRejectedListCheck(t *testing.T) {
	h, _, _, caseID, _ := toReview(t)
	exec(t, h, `UPDATE fos.work SET case_status = 'Pending-Prescreening' WHERE case_id = $1`, caseID)
	exec(t, h, `INSERT INTO fos.prescreening_runs (case_id, state, current_step, attempt, owner_id, heartbeat_at, started_at, updated_at)
		VALUES ($1, 'RUNNING', 'Rejected List', 1, 'dead-executor', $2, $2, $2)`, caseID, time.Now().UTC().Add(-2*time.Minute))
	apptest.New(t, func(c *config.Config) { c.DatabaseURL = h.Config.DatabaseURL })
	eventually(t, "the run to reach SLIK", func() bool { return run(t, h, caseID).Step == "SLIK" })
	if len(apiLogs(t, h, caseID)) != 0 {
		t.Fatal("the Rejected List was logged")
	}
}
