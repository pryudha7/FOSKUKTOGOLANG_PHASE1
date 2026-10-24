package app_test

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"fos/internal/apptest"
	"fos/internal/config"
)

func download(t *testing.T, c *apptest.Client, caseID, fileID string) apptest.Response {
	t.Helper()
	return c.Do(t, http.MethodGet, fmt.Sprintf("%s/api/cases/%s/documents/%s", apptest.EndpointPathDesktop, caseID, fileID), nil)
}

// uploadErrors returns the code of each error the modal banner shows.
func uploadErrors(t *testing.T, r apptest.Response, status int) []string {
	t.Helper()
	if r.Status != status {
		t.Fatalf("upload = %d %s, want %d", r.Status, r.Raw, status)
	}
	var codes []string
	for _, e := range r.Body["errors"].([]any) {
		codes = append(codes, e.(map[string]any)["code"].(string))
	}
	return codes
}

// firstMessage is the first error message of a response.
func firstMessage(r apptest.Response) string {
	errs, _ := r.Body["errors"].([]any)
	if len(errs) == 0 {
		return ""
	}
	m, _ := errs[0].(map[string]any)["message"].(string)
	return m
}

// AC-FR-FILE-002-04, -05, AC-UI-011-09, ERR-041: refused uploads store
// nothing anywhere.
func TestUploadValidation(t *testing.T) {
	h, gcs := uploadHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	exec(t, h, `UPDATE fos.dynamic_system_settings SET value = '1024' WHERE purpose = 'validation/upload/max_file_bytes'`)
	long := strings.Repeat("é", 101)

	for _, tc := range []struct {
		name, fileName string
		size           int
		want           []string
		message        string
	}{
		{"", "scan.pdf", 10, []string{"ERR-023"}, "Harap tuliskan nama file sebelum mengupload dokumen."},
		{"   ", "scan.pdf", 10, []string{"ERR-023"}, ""},
		{long, "scan.pdf", 10, []string{"ERR-024"}, "Nama file maksimal 100 karakter."},
		{"KK", "scan.exe", 10, []string{"ERR-021"}, "Hanya file yang diperbolehkan: gambar, pdf, excel, rar, dan zip"},
		{"KK", "scan", 10, []string{"ERR-021"}, ""},
		{"KK", "scan.pdf", 1025, []string{"ERR-041"}, "Ukuran file melebihi batas maksimum"},
		{"", "scan.doc", 1025, []string{"ERR-023", "ERR-021", "ERR-041"}, ""},
	} {
		r := upload(t, c, caseID, id, "KK", tc.name, tc.fileName, make([]byte, tc.size))
		got := uploadErrors(t, r, http.StatusUnprocessableEntity)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q %s %d bytes: errors %v, want %v", tc.name, tc.fileName, tc.size, got, tc.want)
		}
		if tc.message != "" && firstMessage(r) != tc.message {
			t.Errorf("%q: message %s", tc.name, r.Raw)
		}
	}
	if len(caseDocuments(t, h, caseID)) != 0 || len(gcs.Objects()) != 0 {
		t.Fatalf("refused uploads stored something: %v %v", caseDocuments(t, h, caseID), gcs.Objects())
	}

	// The limits themselves pass: 100 characters, exactly the byte limit,
	// and every allowed type.
	for _, f := range []string{"a.jpg", "a.JPEG", "a.png", "a.gif", "a.bmp", "a.pdf", "a.xls", "a.xlsx", "a.csv", "a.rar", "a.zip"} {
		if r := upload(t, c, caseID, id, "KK", strings.Repeat("é", 100), f, make([]byte, 1024)); r.Status != http.StatusCreated {
			t.Errorf("%s = %d %s", f, r.Status, r.Raw)
		}
	}
}

// AC-FR-FILE-002-09, AC-FR-FILE-003-07: a new file of the same type replaces
// the row, deletes the old object and is what downloads afterwards.
func TestUploadReplacesTheEarlierFileOfTheSameType(t *testing.T) {
	h, gcs := uploadHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	upload(t, c, caseID, id, "KK", "KK lama", "lama.pdf", []byte("old"))
	upload(t, c, caseID, id, "KTP", "KTP", "ktp.pdf", []byte("ktp"))
	old := caseDocuments(t, h, caseID)[0]

	r := upload(t, c, caseID, id, "KK", "KK baru", "baru.pdf", []byte("new"))
	if r.Status != http.StatusCreated {
		t.Fatalf("replace = %d %s", r.Status, r.Raw)
	}
	docs := caseDocuments(t, h, caseID)
	var kk []caseDocumentRow
	for _, d := range docs {
		if d.Type == "KK" {
			kk = append(kk, d)
		}
	}
	if len(docs) != 2 || len(kk) != 1 || kk[0].FileName != "KK baru.pdf" || kk[0].FileID == old.FileID {
		t.Fatalf("documents = %+v", docs)
	}
	objects := gcs.Objects()
	if _, ok := objects["fos-test/"+old.Object]; ok || len(objects) != 2 {
		t.Fatalf("objects = %v", objects)
	}
	if d := download(t, c, caseID, kk[0].FileID); d.Status != http.StatusOK || d.Raw != "new" {
		t.Fatalf("download = %d %q", d.Status, d.Raw)
	}
}

// AC-FR-FILE-002-10, AC-FR-FILE-001-03: when GCS fails, ERR-025, the earlier
// file stays, and the log carries no credential.
func TestUploadWhenGCSIsUnreachableKeepsTheEarlierFile(t *testing.T) {
	h, gcs := uploadHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	upload(t, c, caseID, id, "KK", "KK", "kk.pdf", []byte("old"))
	before := caseDocuments(t, h, caseID)
	logs := apptest.CaptureLogs(t)
	gcs.SetDown(true)

	r := upload(t, c, caseID, id, "KK", "KK baru", "baru.pdf", []byte("new"))
	if got := uploadErrors(t, r, http.StatusBadGateway); !reflect.DeepEqual(got, []string{"ERR-025"}) ||
		firstMessage(r) != "Upload dokumen gagal. Silakan coba lagi." {
		t.Fatalf("errors = %s", r.Raw)
	}
	gcs.SetDown(false)
	if after := caseDocuments(t, h, caseID); !reflect.DeepEqual(after, before) || len(gcs.Objects()) != 1 {
		t.Fatalf("after failure: %+v %v", after, gcs.Objects())
	}
	if !strings.Contains(logs.String(), "upload to GCS failed") {
		t.Fatalf("not logged: %s", logs)
	}
	assertNotLogged(t, logs, strings.Split(strings.TrimSpace(gcs.PrivateKey), "\n")[1])
}

// AC-FR-FILE-001-04, ENV-010: with a GCS key empty the upload is refused and
// the log names the key; nothing falls back to a default.
func TestUploadWithIncompleteGCSConfigurationIsRefused(t *testing.T) {
	gcs, configure := apptest.StartFakeGCS(t)
	h := apptest.New(t, configure, func(c *config.Config) { c.GCS.BucketName = "" })
	h.SeedBMV(t, "BRANCH", "032", "Pekanbaru-Imam Munandar", 30, "Active")
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	logs := apptest.CaptureLogs(t)

	r := upload(t, c, caseID, id, "KK", "KK", "kk.pdf", []byte("x"))
	if got := uploadErrors(t, r, http.StatusBadGateway); !reflect.DeepEqual(got, []string{"ERR-025"}) {
		t.Fatalf("errors = %v", got)
	}
	if len(caseDocuments(t, h, caseID)) != 0 || len(gcs.Objects()) != 0 {
		t.Fatal("stored despite incomplete configuration")
	}
	if !strings.Contains(logs.String(), "GCS_BUCKET_NAME") {
		t.Fatalf("log does not name the key: %s", logs)
	}
}

// Only the owner uploads, to an open Document Underlying Upload, for a row
// of the case's list.
func TestUploadIsOnlyToTheOwnersOpenDocumentRow(t *testing.T) {
	h, gcs := uploadHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	other := loggedIn(t, h, apptest.Operator{UserID: "AO002", RoleCode: "AO", BranchCode: "032"})

	if r := upload(t, other, caseID, id, "KK", "KK", "kk.pdf", []byte("x")); r.Status != http.StatusForbidden {
		t.Errorf("other operator = %d", r.Status)
	}
	if r := upload(t, c, caseID, id, "NPWP", "NPWP", "npwp.pdf", []byte("x")); r.Status != http.StatusBadRequest {
		t.Errorf("unknown document type = %d", r.Status)
	}
	back := c.Do(t, http.MethodPost, submitPath(caseID, id, "back"), nil)
	if r := upload(t, c, caseID, id, "KK", "KK", "kk.pdf", []byte("x")); r.Status != http.StatusConflict {
		t.Errorf("completed assignment = %d", r.Status)
	}
	if r := upload(t, c, caseID, back.Body["assignmentId"], "KK", "KK", "kk.pdf", []byte("x")); r.Status != http.StatusBadRequest {
		t.Errorf("Pre-Screening Submission = %d", r.Status)
	}
	if len(gcs.Objects()) != 0 || len(caseDocuments(t, h, caseID)) != 0 {
		t.Fatal("refused uploads stored something")
	}
}

// AC-FR-FILE-001-05: a reference must belong to an existing case.
func TestCaseDocumentNeedsAnExistingCase(t *testing.T) {
	h, _ := uploadHarness(t)
	_, err := h.DB.Exec(context.Background(), `
		INSERT INTO fos.case_document (file_id, case_id, original_file_name, file_name, bucket_name, object_name, created_at, created_by)
		VALUES (gen_random_uuid(), 'LF-999', 'a.pdf', 'a.pdf', 'b', 'o', now(), 'x')`)
	if err == nil {
		t.Fatal("reference to a missing case was stored")
	}
}

// uploadHarness is a case harness whose GCS is a fake.
func uploadHarness(t *testing.T) (*apptest.Harness, *apptest.FakeGCS) {
	t.Helper()
	gcs, configure := apptest.StartFakeGCS(t)
	h := apptest.New(t, configure)
	h.SeedBMV(t, "BRANCH", "032", "Pekanbaru-Imam Munandar", 30, "Active")
	fakeGCSOf[h] = gcs
	return h, gcs
}

// fakeGCSOf is each upload harness's fake GCS, for tests that reach the
// harness through helpers that do not return it.
var fakeGCSOf = map[*apptest.Harness]*apptest.FakeGCS{}

func uploadPath(caseID string, assignmentID any) string {
	return fmt.Sprintf("%s/api/cases/%s/assignments/%v/documents", apptest.EndpointPathDesktop, caseID, assignmentID)
}

// upload submits the Upload Document modal for a row of Dokumen Pre-Screening.
func upload(t *testing.T, c *apptest.Client, caseID string, assignmentID any, documentType, name, fileName string, data []byte) apptest.Response {
	t.Helper()
	return c.PostMultipart(t, uploadPath(caseID, assignmentID), apptest.Upload{
		Fields:   map[string]string{"category": "DokumenPreScreening", "documentType": documentType, "name": name},
		FileName: fileName, ContentType: "application/pdf", Data: data,
	})
}

type caseDocumentRow struct {
	FileID, Category, Type, Original, FileName, Bucket, Object, ContentType, CreatedBy string
	Size                                                                               int64
}

func caseDocuments(t *testing.T, h *apptest.Harness, caseID string) []caseDocumentRow {
	t.Helper()
	rows, err := h.DB.Query(context.Background(), `
		SELECT file_id::text, document_category, document_type, original_file_name, file_name, bucket_name, object_name,
		       content_type, created_by, file_size_bytes
		FROM fos.case_document WHERE case_id = $1 ORDER BY created_at, document_type`, caseID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []caseDocumentRow
	for rows.Next() {
		var r caseDocumentRow
		if err := rows.Scan(&r.FileID, &r.Category, &r.Type, &r.Original, &r.FileName, &r.Bucket, &r.Object,
			&r.ContentType, &r.CreatedBy, &r.Size); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

// AC-FR-FILE-001-01, -02, AC-FR-FILE-002-03, -08: Submit stores the file in
// the bucket and one reference row, named from Name plus the extension.
func TestUploadStoresTheFileInGCSAndOneReference(t *testing.T) {
	h, gcs := uploadHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))

	r := upload(t, c, caseID, id, "KTP", "KK Budi", "scan.pdf", []byte("%PDF-1.4 ktp"))
	if r.Status != http.StatusCreated {
		t.Fatalf("upload = %d %s", r.Status, r.Raw)
	}
	docs := caseDocuments(t, h, caseID)
	if len(docs) != 1 {
		t.Fatalf("case documents = %+v", docs)
	}
	d := docs[0]
	want := fmt.Sprintf("LoanFinancing/%s/DokumenPreScreening/%s-KK Budi.pdf", caseID, d.FileID)
	if d.Category != "DokumenPreScreening" || d.Type != "KTP" || d.FileName != "KK Budi.pdf" || d.Original != "scan.pdf" ||
		d.Bucket != "fos-test" || d.Object != want || d.ContentType != "application/pdf" || d.Size != 12 || d.CreatedBy != "AO001" {
		t.Fatalf("case document = %+v", d)
	}
	if obj, ok := gcs.Objects()["fos-test/"+want]; !ok || string(obj.Data) != "%PDF-1.4 ktp" {
		t.Fatalf("objects = %v", gcs.Objects())
	}
	if r.Body["fileId"] != d.FileID || r.Body["fileName"] != "KK Budi.pdf" {
		t.Fatalf("response = %s", r.Raw)
	}
	_, rows := documentList(t, c.OpenAssignment(t, caseID, id))
	if got := rows["Dokumen Pre-Screening"][1]; got != (documentRow{"KTP", true, true}) {
		t.Fatalf("KTP row = %+v", got)
	}
}

// AC-FR-FILE-003-03..05, ERR-035: the holder of an open assignment of the
// case and its creator download any file type as an attachment named
// file_name; others are refused, logged, and nothing is fetched.
func TestDownloadingACaseDocument(t *testing.T) {
	h, _ := uploadHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	upload(t, c, caseID, id, "KK", "Kartu Keluarga Budi", "kk.jpg", []byte("jpg-bytes"))
	upload(t, c, caseID, id, "KTP", "KTP", "ktp.png", []byte("png-bytes"))
	docs := caseDocuments(t, h, caseID)

	for _, d := range docs {
		r := download(t, c, caseID, d.FileID)
		if r.Status != http.StatusOK || r.Raw != map[string]string{"KK": "jpg-bytes", "KTP": "png-bytes"}[d.Type] ||
			!strings.HasPrefix(r.Header.Get("Content-Disposition"), "attachment") ||
			!strings.Contains(r.Header.Get("Content-Disposition"), d.FileName) {
			t.Fatalf("download %s = %d %q %v", d.Type, r.Status, r.Raw, r.Header)
		}
	}

	logs := apptest.CaptureLogs(t)
	other := loggedIn(t, h, apptest.Operator{UserID: "AO002", RoleCode: "AO", BranchCode: "032"})
	r := download(t, other, caseID, docs[0].FileID)
	assertError(t, r, http.StatusForbidden, "ERR-035", "Unable to open the attachment. You lack the required permissions.")
	if !strings.Contains(logs.String(), "AO002") || !strings.Contains(logs.String(), docs[0].FileID) {
		t.Fatalf("refusal not logged: %s", logs)
	}

	// The creator keeps access after the assignment is finished; a file of
	// another case is not found through this case.
	c.Do(t, http.MethodPost, submitPath(caseID, id, "back"), nil)
	exec(t, h, `UPDATE fos.assignments SET assignment_status = 'sudah selesai dikerjakan' WHERE case_id = $1`, caseID)
	if r := download(t, c, caseID, docs[0].FileID); r.Status != http.StatusOK {
		t.Fatalf("creator after completion = %d", r.Status)
	}
	if r := download(t, c, "LF-999", docs[0].FileID); r.Status != http.StatusNotFound {
		t.Fatalf("wrong case = %d", r.Status)
	}
	if r := download(t, c, caseID, "not-a-uuid"); r.Status != http.StatusNotFound {
		t.Fatalf("bad id = %d", r.Status)
	}
}

// A holder of an open assignment who did not create the case may download.
func TestOpenAssignmentHolderDownloads(t *testing.T) {
	h, _ := uploadHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	upload(t, c, caseID, id, "KK", "KK", "kk.pdf", []byte("pdf"))
	tl := relogin(t, h, "TL01")
	file := caseDocuments(t, h, caseID)[0].FileID
	if r := download(t, tl, caseID, file); r.Status != http.StatusForbidden {
		t.Fatalf("TL without assignment = %d", r.Status)
	}
	exec(t, h, `INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
		VALUES ($1, 'Document Checklist', 'worklist', 'TL01', 'baru', now())`, caseID)
	if r := download(t, tl, caseID, file); r.Status != http.StatusOK || r.Raw != "pdf" {
		t.Fatalf("TL with open assignment = %d", r.Status)
	}
}

// AC-FR-CASE-006-01, -02: the attachments panel lists every file of the
// case, newest first, with its category label, uploader and time.
func TestAttachmentsPanelListsTheCaseFiles(t *testing.T) {
	h, _ := uploadHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	upload(t, c, caseID, id, "KK", "KK Budi", "kk.pdf", []byte("1"))
	h.Clock.Advance(time.Minute)
	upload(t, c, caseID, id, "KTP", "KTP Budi", "ktp.png", []byte("2"))
	h.Clock.Advance(time.Minute)
	upload(t, c, caseID, id, "SLIK", "SLIK", "slik.pdf", []byte("3"))

	r := c.Case(t, caseID)
	if r.Body["attachmentCount"] != float64(3) {
		t.Fatalf("attachmentCount = %v", r.Body["attachmentCount"])
	}
	var got []string
	for _, a := range r.Body["attachments"].([]any) {
		m := a.(map[string]any)
		at, err := time.Parse(time.RFC3339Nano, m["createdAt"].(string))
		if err != nil || m["fileId"] == "" {
			t.Fatalf("attachment = %v", m)
		}
		got = append(got, fmt.Sprintf("%s|%s|%s|%s", m["fileName"], m["categoryLabel"], m["uploadedBy"], at.Sub(h.Clock.Now()).Round(time.Second)))
	}
	want := []string{
		"SLIK.pdf|Dokumen Pre-Screening|Pebrianto|0s",
		"KTP Budi.png|Dokumen Pre-Screening|Pebrianto|-1m0s",
		"KK Budi.pdf|Dokumen Pre-Screening|Pebrianto|-2m0s",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("attachments = %v", got)
	}
}

// ENV-010: GCS keys come from the environment, "TBD" counts as empty, and
// the API base is Google's, not configurable.
func TestGCSConfigurationFromTheEnvironment(t *testing.T) {
	gcs, configure := apptest.StartFakeGCS(t)
	var fake config.Config
	configure(&fake)
	t.Setenv("GCS_PROJECT_ID", "fos-test-project")
	t.Setenv("GCS_CREDENTIAL", fake.GCS.Credential)
	loaded := loadConfigFromEnv(t, apptest.StartFakeLDAP(t), "GCS_BUCKET_NAME", "TBD")
	if loaded.GCS.APIBaseURL != "https://storage.googleapis.com" || loaded.GCS.BucketName != "" ||
		loaded.GCS.Credential != fake.GCS.Credential {
		t.Fatalf("GCS config = %+v", loaded.GCS)
	}

	h := apptest.New(t, func(c *config.Config) { c.GCS = loaded.GCS; c.GCS.APIBaseURL = gcs.URL })
	h.SeedBMV(t, "BRANCH", "032", "Pekanbaru-Imam Munandar", 30, "Active")
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	if got := uploadErrors(t, upload(t, c, caseID, id, "KK", "KK", "kk.pdf", []byte("x")), http.StatusBadGateway); got[0] != "ERR-025" {
		t.Fatalf("errors = %v", got)
	}
	if len(gcs.Objects()) != 0 {
		t.Fatal("uploaded with bucket TBD")
	}
}

// DB-RULE-083, DB-RULE-084: the per-file limit is read from the settings
// table, a change applies within 60 seconds, GLOBAL serves when the app has
// no row, and a missing setting fails the upload.
func TestPerFileLimitFollowsDynamicSystemSettings(t *testing.T) {
	h, _ := uploadHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	try := func(size int) int {
		t.Helper()
		return upload(t, c, caseID, id, "KK", "KK", "kk.pdf", make([]byte, size)).Status
	}
	if s := try(2000); s != http.StatusCreated {
		t.Fatalf("2000 bytes under the seeded 10 MB = %d", s)
	}
	exec(t, h, `UPDATE fos.dynamic_system_settings SET value = '1000' WHERE purpose = 'validation/upload/max_file_bytes'`)
	h.Clock.Advance(61 * time.Second)
	c = relogin(t, h, "AO001")
	if s := try(2000); s != http.StatusUnprocessableEntity {
		t.Fatalf("2000 bytes after lowering to 1000 = %d", s)
	}

	exec(t, h, `UPDATE fos.dynamic_system_settings SET app_scope = 'GLOBAL', value = '3000'`)
	h.Clock.Advance(61 * time.Second)
	c = relogin(t, h, "AO001")
	if s := try(2000); s != http.StatusCreated {
		t.Fatalf("2000 bytes under GLOBAL 3000 = %d", s)
	}
	exec(t, h, `INSERT INTO fos.dynamic_system_settings (app_scope, purpose, value, value_type, updated_at, updated_by)
		VALUES ('LOAN_APPLICATION', 'validation/upload/max_file_bytes', '1000', 'INTEGER', now(), 'test')`)
	h.Clock.Advance(61 * time.Second)
	c = relogin(t, h, "AO001")
	if s := try(2000); s != http.StatusUnprocessableEntity {
		t.Fatalf("2000 bytes with the app's 1000 over GLOBAL 3000 = %d", s)
	}

	exec(t, h, `DELETE FROM fos.dynamic_system_settings`)
	h.Clock.Advance(61 * time.Second)
	c = relogin(t, h, "AO001")
	if s := try(10); s != http.StatusBadGateway {
		t.Fatalf("missing setting = %d", s)
	}
}

// DB-RULE-025: when the reference cannot be saved the new object is removed,
// so no object is left without a reference.
func TestFailedReferenceSaveRemovesTheUploadedObject(t *testing.T) {
	h, gcs := uploadHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	exec(t, h, `ALTER TABLE fos.case_document ADD CONSTRAINT refuse_all CHECK (file_size_bytes < 0) NOT VALID`)

	if got := uploadErrors(t, upload(t, c, caseID, id, "KK", "KK", "kk.pdf", []byte("x")), http.StatusBadGateway); got[0] != "ERR-025" {
		t.Fatalf("errors = %v", got)
	}
	if len(gcs.Objects()) != 0 {
		t.Fatalf("orphan objects = %v", gcs.Objects())
	}
}

// Agreed decision 9: with a GCS key empty, downloads are refused with
// ERR-025 and the log names the key.
func TestDownloadWithIncompleteGCSConfigurationGivesERR025(t *testing.T) {
	h, _ := uploadHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	upload(t, c, caseID, id, "KK", "KK", "kk.pdf", []byte("x"))
	file := caseDocuments(t, h, caseID)[0].FileID

	// A second backend on the same database, its credential unset.
	_, configure := apptest.StartFakeGCS(t)
	other := apptest.New(t, configure, func(cfg *config.Config) { cfg.DatabaseURL = h.Config.DatabaseURL; cfg.GCS.Credential = "" })
	logs := apptest.CaptureLogs(t)
	c2 := relogin(t, other, "AO001")
	r := download(t, c2, caseID, file)
	if got := uploadErrors(t, r, http.StatusBadGateway); got[0] != "ERR-025" {
		t.Fatalf("errors = %v", got)
	}
	if !strings.Contains(logs.String(), "GCS_CREDENTIAL") {
		t.Fatalf("log does not name the key: %s", logs)
	}
}

// DB-RULE-023: a download reads the bucket its reference names, not the
// configured one.
func TestDownloadUsesTheStoredBucket(t *testing.T) {
	h, _ := uploadHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	upload(t, c, caseID, id, "KK", "KK", "kk.pdf", []byte("x"))
	file := caseDocuments(t, h, caseID)[0].FileID
	exec(t, h, `UPDATE fos.case_document SET bucket_name = 'fos-archive' WHERE file_id = $1`, file)
	if r := download(t, c, caseID, file); r.Status != http.StatusBadGateway {
		t.Fatalf("download from a bucket without the object = %d", r.Status)
	}
}

// AC-FR-FILE-005-09: a failure reading the list is ERR-027 too; the page
// still opens.
func TestFailedDocumentListReadGivesERR027(t *testing.T) {
	h := caseHarness(t)
	c, caseID, id := underlying(t, h)
	documentList(t, c.OpenAssignment(t, caseID, id))
	exec(t, h, `ALTER TABLE fos.case_document RENAME TO case_document_gone`)
	r := c.OpenAssignment(t, caseID, id)
	if e, _ := field(t, r, "page", "listError").(map[string]any); r.Status != http.StatusOK || e["code"] != "ERR-027" {
		t.Fatalf("open = %d %s", r.Status, r.Raw)
	}
}
