package app_test

import (
	"context"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"fos/internal/apptest"
)

// branchOfficer seeds BMV branch 032, a Team Lead and a Branch Officer of
// the given role at that branch reporting to the Team Lead, and logs the
// officer in.
func branchOfficer(t *testing.T, h *apptest.Harness, userID, role string) *apptest.Client {
	t.Helper()
	h.SeedOperator(t, apptest.Operator{UserID: "TL01", Password: "Rahasia#1", FullName: "Tini Lestari", RoleCode: "TL"})
	return loggedIn(t, h, apptest.Operator{UserID: userID, FullName: "Pebrianto", RoleCode: role, BranchCode: "032", ReportTo: "TL01"})
}

func caseHarness(t *testing.T) *apptest.Harness {
	t.Helper()
	h := apptest.New(t)
	h.SeedBMV(t, "BRANCH", "032", "Pekanbaru-Imam Munandar", 30, "Active")
	return h
}

func created(t *testing.T, r apptest.Response) (string, any) {
	t.Helper()
	if r.Status != http.StatusCreated {
		t.Fatalf("create case = %d %s", r.Status, r.Raw)
	}
	return r.Body["caseId"].(string), r.Body["assignmentId"]
}

type workRow struct {
	CaseType, BranchOfficer, CreatedBy, UpdatedBy string
	TeamLead, BranchCode, CustomerCode, Status    *string
	Stage, Parent                                 *string
	Urgency                                       int
	CreatedAt                                     time.Time
}

func work(t *testing.T, h *apptest.Harness, caseID string) workRow {
	t.Helper()
	var w workRow
	if err := h.DB.QueryRow(context.Background(), `
		SELECT case_type, branch_officer, created_by, updated_by, branch_officer_team_lead, branch_code,
		       customer_code, case_status, current_stage, parent_case_id, urgency_work_class, created_at
		FROM fos.work WHERE case_id = $1`, caseID).Scan(
		&w.CaseType, &w.BranchOfficer, &w.CreatedBy, &w.UpdatedBy, &w.TeamLead, &w.BranchCode,
		&w.CustomerCode, &w.Status, &w.Stage, &w.Parent, &w.Urgency, &w.CreatedAt); err != nil {
		t.Fatalf("work %s: %v", caseID, err)
	}
	return w
}

func count(t *testing.T, h *apptest.Harness, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := h.DB.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// AC-FR-APP-001-01, AC-FR-APP-001-02, AC-FR-APP-001-03, AC-FR-CASE-001-01
func TestBranchOfficerCreatesFinancingCase(t *testing.T) {
	h := caseHarness(t)
	c := branchOfficer(t, h, "AO001", "AO")

	caseID, assignmentID := created(t, c.CreateCase(t, "Loan Financing"))
	if caseID != "LF-1" {
		t.Fatalf("first Case ID = %s, want LF-1", caseID)
	}
	w := work(t, h, caseID)
	if w.CaseType != "Loan Financing" || w.Parent != nil || w.Urgency != 10 || w.BranchOfficer != "AO001" ||
		deref(w.TeamLead) != "TL01" || deref(w.BranchCode) != "032" || deref(w.Status) != "New" ||
		deref(w.Stage) != "Financing Submission" || w.CreatedBy != "AO001" || w.UpdatedBy != "AO001" {
		t.Fatalf("work row = %+v", w)
	}
	// The branch name is read from BMV, never stored on the case (DEC-127).
	if n := count(t, h, `SELECT count(*) FROM fos.work w WHERE w::text LIKE '%Pekanbaru%'`); n != 0 {
		t.Fatalf("branch name stored on the case")
	}

	var (
		name, typ, status, assignee, worker string
		createdAt, firstOpened              time.Time
		completed                           *time.Time
	)
	if err := h.DB.QueryRow(context.Background(), `
		SELECT assignment_name, assignment_type, assignment_status, assigned_user_id, worked_by_user_id,
		       created_at, first_opened_at, completed_at
		FROM fos.assignments WHERE assignment_id = $1 AND case_id = $2`, assignmentID, caseID).Scan(
		&name, &typ, &status, &assignee, &worker, &createdAt, &firstOpened, &completed); err != nil {
		t.Fatalf("assignment: %v", err)
	}
	if name != "Pre-Screening Submission" || typ != "worklist" || status != "sudah dibuka" ||
		assignee != "AO001" || worker != "AO001" || !firstOpened.Equal(createdAt) || completed != nil {
		t.Fatalf("assignment = %s %s %s %s %s %v %v %v", name, typ, status, assignee, worker, createdAt, firstOpened, completed)
	}

	second, _ := created(t, c.CreateCase(t, "Loan Financing"))
	if second != "LF-2" {
		t.Fatalf("second Case ID = %s, want LF-2", second)
	}
}

// SAO creates cases too (DEC-029).
func TestSAOCreatesFinancingCase(t *testing.T) {
	h := caseHarness(t)
	created(t, branchOfficer(t, h, "SAO01", "SAO").CreateCase(t, "Loan Financing"))
}

// AC-FR-APP-001-06: refused for any other role, Authors included, and
// nothing is created, not even a Case ID number.
func TestOnlyAOAndSAOCreateCases(t *testing.T) {
	h := caseHarness(t)
	h.SeedMapping(t, apptest.Mapping{Role: "Ops Authors", AccessGroup: "Authors", WorkGroup: "Default"})
	for _, role := range []string{"TL", "BM", "Ops Authors"} {
		c := loggedIn(t, h, apptest.Operator{UserID: "U-" + role, RoleCode: role, BranchCode: "032"})
		r := c.CreateCase(t, "Loan Financing")
		assertError(t, r, http.StatusForbidden, "ERR-022", "Anda tidak memiliki akses untuk membuat pengajuan pembiayaan.")
	}
	if n := count(t, h, `SELECT count(*) FROM fos.work`) + count(t, h, `SELECT count(*) FROM fos.assignments`); n != 0 {
		t.Fatalf("%d rows created by refused requests", n)
	}
	caseID, _ := created(t, branchOfficer(t, h, "AO001", "AO").CreateCase(t, "Loan Financing"))
	if caseID != "LF-1" {
		t.Fatalf("refused requests consumed Case ID numbers: next is %s", caseID)
	}
}

// Only the parent case type offered by FR-NAV-003 can be created here.
func TestUnknownCaseTypeIsRejected(t *testing.T) {
	h := caseHarness(t)
	c := branchOfficer(t, h, "AO001", "AO")
	for _, typ := range []string{"Phone Verification", "Financing", ""} {
		if r := c.CreateCase(t, typ); r.Status != http.StatusBadRequest {
			t.Fatalf("case type %q: status %d, want 400", typ, r.Status)
		}
	}
}

// AC-FR-CASE-001-02
func TestConcurrentCaseCreationNeverDuplicatesCaseIDs(t *testing.T) {
	h := caseHarness(t)
	c := branchOfficer(t, h, "AO001", "AO")
	const n = 20
	ids := make(chan string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := c.CreateCase(t, "Loan Financing")
			if r.Status == http.StatusCreated {
				ids <- r.Body["caseId"].(string)
			}
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate Case ID %s", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Fatalf("%d of %d creations succeeded", len(seen), n)
	}
}

// AC-FR-CASE-001-04
func TestDuplicateCaseIDIsRejectedByTheDatabase(t *testing.T) {
	h := caseHarness(t)
	caseID, _ := created(t, branchOfficer(t, h, "AO001", "AO").CreateCase(t, "Loan Financing"))
	_, err := h.DB.Exec(context.Background(), `
		INSERT INTO fos.work (case_id, case_type, urgency_work_class, branch_officer, created_at, updated_at)
		VALUES ($1, 'Loan Financing', 10, 'AO001', now(), now())`, caseID)
	if err == nil {
		t.Fatalf("a second row with Case ID %s was accepted", caseID)
	}
}

// AC-FR-APP-001-04, AC-FR-APP-001-05: the year comes from the creator's
// timezone; created_at is stored in UTC.
func TestCustomerCodeUsesTheCreatorsYear(t *testing.T) {
	h := caseHarness(t)
	branchOfficer(t, h, "AO001", "AO")
	// Each clock jump outlasts the idle timeout, so log in again after it.
	relogin := func() *apptest.Client {
		c := h.NewClient()
		if r := c.LoginDatabase(t, "AO001", "Rahasia#1"); r.Status != http.StatusOK {
			t.Fatalf("login = %d", r.Status)
		}
		return c
	}

	h.Clock.Set(time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC))
	first, _ := created(t, relogin().CreateCase(t, "Loan Financing"))
	if got := deref(work(t, h, first).CustomerCode); got != "SMSME2600000001" {
		t.Fatalf("customer_code = %s, want SMSME2600000001", got)
	}

	at := time.Date(2026, 12, 31, 18, 0, 0, 0, time.UTC) // 01:00 on 1 January 2027 at GMT+7
	h.Clock.Set(at)
	second, _ := created(t, relogin().CreateCase(t, "Loan Financing"))
	w := work(t, h, second)
	if got := deref(w.CustomerCode); got != "SMSME2700000002" {
		t.Fatalf("customer_code = %s, want SMSME2700000002", got)
	}
	if !w.CreatedAt.Equal(at) {
		t.Fatalf("created_at = %v, want %v (UTC)", w.CreatedAt, at)
	}
}

// Agreed for TBD-026 / TBD-029c: a case is still created without branch or
// Team Lead; the form then shows ERR-006.
func TestCaseWithoutBranchShowsERR006(t *testing.T) {
	h := caseHarness(t)
	for userID, branch := range map[string]string{"AO-NOBRANCH": "", "AO-BADBRANCH": "999"} {
		c := loggedIn(t, h, apptest.Operator{UserID: userID, RoleCode: "AO", BranchCode: branch})
		caseID, assignmentID := created(t, c.CreateCase(t, "Loan Financing"))
		if w := work(t, h, caseID); w.TeamLead != nil {
			t.Fatalf("team lead = %s, want none", deref(w.TeamLead))
		}
		page := c.OpenAssignment(t, caseID, assignmentID)
		if field(t, page, "page", "branchName") != "" ||
			field(t, page, "page", "branchError", "code") != "ERR-006" ||
			field(t, page, "page", "branchError", "message") != "Branch ID operator tidak ditemukan" {
			t.Fatalf("%s: page = %s", userID, page.Raw)
		}
	}
}

// FR-APP-002: Nama Branch from BMV BRANCH; Tipe Nasabah options from BMV
// CUSTOMER_TYPE, Active only (blank counts as Active, DB-RULE-092), sorted by
// sort_order then code.
func TestPreScreeningSubmissionPageShowsBranchAndCustomerTypes(t *testing.T) {
	h := caseHarness(t)
	h.SeedBMV(t, "CUSTOMER_TYPE", "Koperasi", "Koperasi", 1, "")
	h.SeedBMV(t, "CUSTOMER_TYPE", "Lama", "Lama", 0, "Inactive")
	c := branchOfficer(t, h, "AO001", "AO")
	caseID, assignmentID := created(t, c.CreateCase(t, "Loan Financing"))

	page := c.OpenAssignment(t, caseID, assignmentID)
	if page.Status != http.StatusOK || page.Body["name"] != "Pre-Screening Submission" {
		t.Fatalf("open = %d %s", page.Status, page.Raw)
	}
	if field(t, page, "page", "branchName") != "Pekanbaru-Imam Munandar" || field(t, page, "page", "branchError") != nil {
		t.Fatalf("branch = %s", page.Raw)
	}
	var got []any
	for _, o := range field(t, page, "page", "customerTypeOptions").([]any) {
		got = append(got, o.(map[string]any)["description"])
	}
	if want := []any{"Koperasi", "Badan Usaha", "Perorangan"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("options = %v, want %v", got, want)
	}
}

func stageNames(t *testing.T, r apptest.Response) (names []string, current string, entered []string) {
	t.Helper()
	for _, s := range r.Body["stages"].([]any) {
		m := s.(map[string]any)
		names = append(names, m["name"].(string))
		if m["current"] == true {
			current = m["name"].(string)
		}
		if m["entered"] == true {
			entered = append(entered, m["name"].(string))
		}
	}
	return names, current, entered
}

var mainStages = []string{"Financing Submission", "Financing Application", "Financing Checker", "Survey Analyst",
	"Recommendation", "Pre-Disbursement", "FAC Checker", "Disbursement"}

// AC-FR-CASE-004-01, AC-FR-CASE-004-06
func TestStageBarOfANewCase(t *testing.T) {
	h := caseHarness(t)
	c := branchOfficer(t, h, "AO001", "AO")
	caseID, _ := created(t, c.CreateCase(t, "Loan Financing"))

	names, current, entered := stageNames(t, c.Case(t, caseID))
	if !reflect.DeepEqual(names, mainStages) || current != "Financing Submission" || len(entered) != 0 {
		t.Fatalf("stages %v current %s entered %v", names, current, entered)
	}
}

// moveCase records the case entering stages in order, the way later tickets'
// transitions will, and leaves it in the last one.
func moveCase(t *testing.T, h *apptest.Harness, caseID string, steps ...[2]string) {
	t.Helper()
	ctx := context.Background()
	for i, s := range steps {
		stage, from := s[0], s[1]
		at := time.Date(2026, 1, 1, 0, i, 0, 0, time.UTC)
		if _, err := h.DB.Exec(ctx, `
			INSERT INTO fos.case_stage_progress (case_id, stage_name, first_entered_at, entered_from_stage)
			VALUES ($1, $2, $3, NULLIF($4, ''))
			ON CONFLICT (case_id, stage_name) DO UPDATE SET entered_from_stage = EXCLUDED.entered_from_stage`,
			caseID, stage, at, from); err != nil {
			t.Fatal(err)
		}
		if _, err := h.DB.Exec(ctx, `UPDATE fos.work SET current_stage = $2 WHERE case_id = $1`, caseID, stage); err != nil {
			t.Fatal(err)
		}
	}
}

// AC-FR-CASE-004-02 to AC-FR-CASE-004-07
func TestStageBarFollowsTheCasesProgress(t *testing.T) {
	h := caseHarness(t)
	c := branchOfficer(t, h, "AO001", "AO")
	cases := []struct {
		name        string
		steps       [][2]string
		wantNames   []string
		wantCurrent string
		wantEntered []string
	}{
		// Every case is recorded entering Financing Submission when created (FR-CASE-007).
		{"004-02 current Survey Analyst", [][2]string{{"Survey Analyst", ""}}, mainStages, "Survey Analyst", []string{"Financing Submission"}},
		{"004-03 into Financing Application", [][2]string{{"Financing Application", ""}}, mainStages,
			"Financing Application", []string{"Financing Submission"}},
		{"004-04 back to Financing Submission", [][2]string{{"Financing Application", ""}, {"Financing Submission", ""}}, mainStages,
			"Financing Submission", []string{"Financing Application"}},
		{"004-05 Financing Revision after Financing Application",
			[][2]string{{"Financing Application", ""}, {"Financing Revision", "Financing Application"}, {"Financing Checker", ""}, {"Survey Analyst", ""}},
			[]string{"Financing Submission", "Financing Application", "Financing Revision", "Financing Checker", "Survey Analyst",
				"Recommendation", "Pre-Disbursement", "FAC Checker", "Disbursement"},
			"Survey Analyst", []string{"Financing Submission", "Financing Application", "Financing Revision", "Financing Checker"}},
		{"004-07 Financing Revision re-entered from Survey Analyst",
			[][2]string{{"Financing Application", ""}, {"Financing Revision", "Financing Application"}, {"Financing Checker", ""},
				{"Survey Analyst", ""}, {"Financing Revision", "Survey Analyst"}},
			[]string{"Financing Submission", "Financing Application", "Financing Checker", "Survey Analyst", "Financing Revision",
				"Recommendation", "Pre-Disbursement", "FAC Checker", "Disbursement"},
			"Financing Revision", []string{"Financing Submission", "Financing Application", "Financing Checker", "Survey Analyst"}},
		{"Rejected after Financing Submission", [][2]string{{"Rejected", "Financing Submission"}},
			append([]string{"Financing Submission", "Rejected"}, mainStages[1:]...), "Rejected", []string{"Financing Submission"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caseID, _ := created(t, c.CreateCase(t, "Loan Financing"))
			moveCase(t, h, caseID, tc.steps...)
			names, current, entered := stageNames(t, c.Case(t, caseID))
			if !reflect.DeepEqual(names, tc.wantNames) || current != tc.wantCurrent || !reflect.DeepEqual(entered, tc.wantEntered) {
				t.Fatalf("stages %v\ncurrent %s\nentered %v", names, current, entered)
			}
		})
	}
}

// AC-FR-CASE-005-01, AC-FR-CASE-005-02
func TestCaseInformationPanel(t *testing.T) {
	h := caseHarness(t)
	c := branchOfficer(t, h, "AO001", "AO")
	caseID, _ := created(t, c.CreateCase(t, "Loan Financing"))

	r := c.Case(t, caseID)
	if r.Body["caseId"] != caseID || r.Body["caseTypeLabel"] != "Financing" || r.Body["priority"] != float64(10) ||
		r.Body["status"] != "New" || field(t, r, "created", "name") != "Pebrianto" || field(t, r, "updated", "name") != "Pebrianto" ||
		field(t, r, "created", "at") == nil {
		t.Fatalf("case = %s", r.Raw)
	}
	if _, err := h.DB.Exec(context.Background(), `UPDATE fos.operator SET full_name = 'Pebrianto Baru' WHERE user_id = 'AO001'`); err != nil {
		t.Fatal(err)
	}
	if got := field(t, c.Case(t, caseID), "created", "name"); got != "Pebrianto Baru" {
		t.Fatalf("created name = %v, want the operator's current full name", got)
	}
	if r.Body["attachmentCount"] != float64(0) {
		t.Fatalf("attachmentCount = %v", r.Body["attachmentCount"])
	}
}

func lifecycle(t *testing.T, r apptest.Response) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	for _, s := range r.Body["lifecycle"].([]any) {
		m := s.(map[string]any)
		procs := map[string]any{"entered": m["entered"]}
		var names []string
		for _, p := range m["processes"].([]any) {
			pm := p.(map[string]any)
			names = append(names, pm["name"].(string))
			if pm["started"] == true {
				procs["started:"+pm["name"].(string)] = true
			}
		}
		procs["names"] = names
		out[m["stage"].(string)] = procs
	}
	return out
}

// AC-FR-CASE-007-01 to AC-FR-CASE-007-03
func TestCaseLifecycle(t *testing.T) {
	h := caseHarness(t)
	c := branchOfficer(t, h, "AO001", "AO")
	caseID, _ := created(t, c.CreateCase(t, "Loan Financing"))

	r := c.Case(t, caseID)
	var order []string
	for _, s := range r.Body["lifecycle"].([]any) {
		order = append(order, s.(map[string]any)["stage"].(string))
	}
	if !reflect.DeepEqual(order, mainStages) {
		t.Fatalf("lifecycle stages %v", order)
	}
	lc := lifecycle(t, r)
	if got := lc["Financing Submission"]["names"]; !reflect.DeepEqual(got, []string{"Pre-Screening Submission", "Pipeline Data Entry",
		"Badan Usaha", "Mapping Value Scoring DataSet", "Data Pribadi", "Data Pasangan", "Data Keluarga Dekat", "Mapping Value Scoring DataSet"}) {
		t.Fatalf("Financing Submission processes %v", got)
	}
	// A process counts as started when its first assignment is created (TBD-034a, agreed).
	if lc["Financing Submission"]["started:Pre-Screening Submission"] != true || lc["Financing Submission"]["started:Pipeline Data Entry"] != nil {
		t.Fatalf("started = %v", lc["Financing Submission"])
	}

	// AC-FR-CASE-007-02/03: recorded progress stays, even after going back.
	if _, err := h.DB.Exec(context.Background(), `
		INSERT INTO fos.case_process_progress (case_id, process_id, first_started_at)
		SELECT $1, process_id, now() FROM fos.lifecycle_process
		WHERE (stage_name, process_name) IN (('Financing Application', 'Set Initial Data'), ('Financing Checker', 'Financing Checker'))`,
		caseID); err != nil {
		t.Fatal(err)
	}
	moveCase(t, h, caseID, [2]string{"Financing Application", ""}, [2]string{"Financing Checker", ""}, [2]string{"Financing Application", ""})
	lc = lifecycle(t, c.Case(t, caseID))
	if lc["Financing Application"]["started:Set Initial Data"] != true || lc["Financing Application"]["started:Data Usaha Nasabah"] != nil ||
		lc["Financing Checker"]["entered"] != true || lc["Financing Checker"]["started:Financing Checker"] != true ||
		lc["Financing Application"]["entered"] != false {
		t.Fatalf("lifecycle = %v", lc)
	}
}

// FR-CASE-009 Go rules applied on the server (agreed): the worklist owner
// and Authors may open an assignment; anyone else may not.
func TestOnlyTheOwnerOrAuthorsOpenAnAssignment(t *testing.T) {
	h := caseHarness(t)
	owner := branchOfficer(t, h, "AO001", "AO")
	caseID, assignmentID := created(t, owner.CreateCase(t, "Loan Financing"))

	other := loggedIn(t, h, apptest.Operator{UserID: "AO002", RoleCode: "AO", BranchCode: "032"})
	if r := other.OpenAssignment(t, caseID, assignmentID); r.Status != http.StatusForbidden {
		t.Fatalf("another AO opened the assignment: %d", r.Status)
	}
	h.SeedMapping(t, apptest.Mapping{Role: "Ops Authors", AccessGroup: "Authors", WorkGroup: "Default"})
	authors := loggedIn(t, h, apptest.Operator{UserID: "AU01", RoleCode: "Ops Authors"})
	if r := authors.OpenAssignment(t, caseID, assignmentID); r.Status != http.StatusOK {
		t.Fatalf("Authors could not open the assignment: %d", r.Status)
	}
	if r := owner.OpenAssignment(t, "LF-404", assignmentID); r.Status != http.StatusNotFound {
		t.Fatalf("assignment opened under the wrong case: %d", r.Status)
	}
}

// AC-FR-ASSIGN-001-01, AC-FR-ASSIGN-001-03: a new assignment becomes opened
// on first opening; first_opened_at and worked_by_user_id are set once.
func TestFirstOpeningMarksAnAssignmentOpened(t *testing.T) {
	h := caseHarness(t)
	c := branchOfficer(t, h, "AO001", "AO")
	caseID, _ := created(t, c.CreateCase(t, "Loan Financing"))
	var id int64
	if err := h.DB.QueryRow(context.Background(), `
		INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
		VALUES ($1, 'Pre-Screening Submission', 'worklist', 'AO001', 'baru', $2) RETURNING assignment_id`,
		caseID, h.Clock.Now()).Scan(&id); err != nil {
		t.Fatal(err)
	}
	read := func() (string, *time.Time, *string) {
		var (
			status string
			opened *time.Time
			worker *string
		)
		if err := h.DB.QueryRow(context.Background(),
			`SELECT assignment_status, first_opened_at, worked_by_user_id FROM fos.assignments WHERE assignment_id = $1`, id).
			Scan(&status, &opened, &worker); err != nil {
			t.Fatal(err)
		}
		return status, opened, worker
	}

	h.Clock.Advance(time.Minute)
	c.OpenAssignment(t, caseID, id)
	status, first, worker := read()
	// PostgreSQL TIMESTAMP keeps microseconds.
	if status != "sudah dibuka" || first == nil || !first.Equal(h.Clock.Now().Truncate(time.Microsecond)) || deref(worker) != "AO001" {
		t.Fatalf("after first open: %s %v %s", status, first, deref(worker))
	}
	h.Clock.Advance(time.Minute)
	c.OpenAssignment(t, caseID, id)
	if _, again, _ := read(); !again.Equal(*first) {
		t.Fatalf("first_opened_at changed on a later opening: %v -> %v", *first, *again)
	}
}

func assertError(t *testing.T, r apptest.Response, status int, code, message string) {
	t.Helper()
	if r.Status != status || r.Body["code"] != code || r.Body["message"] != message {
		t.Fatalf("got %d %s, want %d %s %q", r.Status, r.Raw, status, code, message)
	}
}
