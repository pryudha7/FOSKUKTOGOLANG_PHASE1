// Ticket 36: UI-023 Waiting SLIK (Gambar 44). A case made through the UI
// up to Create is put in WAITING_SLIK through psql (run, slik_report and the
// SLIKWQFlowAct assignment, as the Pre-Screening Check makes them); a small
// fake SFS on port 14555 answers the status check with 4040. Checks the
// header, Due, Check SLIK Status, the Message panel, the two panels with a
// Spouse, and no Submit. Needs what create.js needs, file storage as for
// duu.js, SLIK_APIKEY in the backend environment, and api/slik/baseurl set
// to http://127.0.0.1:14555.
// Usage: node waitslik.js shots
const { chromium } = require("playwright-core");
const fs = require("fs");
const http = require("http");
const { execSync } = require("child_process");
const out = process.argv[2];
const psql = (sql) => execSync(`"${process.env.DOCKER || "docker"}" exec -i fos-smoke-pg psql -U postgres -qtA`, { input: sql }).toString().trim();
const base = "http://localhost:13000";

async function login(browser, user) {
  const ctx = await browser.newContext({ viewport: { width: 1500, height: 1000 }, extraHTTPHeaders: { "X-Forwarded-Proto": "https" } });
  const page = await ctx.newPage();
  page.on("pageerror", (e) => console.error("pageerror", e.message));
  await page.goto(`${base}/PRWebDB1`);
  await page.fill('input[name="username"]', user);
  await page.fill('input[name="password"]', "Rahasia#1");
  await page.click('button[type="submit"]');
  await page.waitForURL("**/app/fossyariah");
  return page;
}

async function uploadDoc(page, doc) {
  await page.getByRole("button", { name: `Upload ${doc}` }).click();
  await page.locator("input[type=file]").setInputFiles(`${out}/small.pdf`);
  await page.getByRole("button", { name: "Submit" }).click();
  await page.waitForSelector(`[aria-label='Download ${doc}']`);
}

(async () => {
  const sfs = http.createServer((req, res) => {
    res.writeHead(404, { "Content-Type": "application/json" });
    res.end('{"code":"4040","message":"data not found"}');
  });
  sfs.listen(14555);
  fs.writeFileSync(`${out}/small.pdf`, "%PDF-1.4 small");
  const browser = await chromium.launch({ executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe" });
  const r = {};
  const ao = await login(browser, "pebrianto");
  await ao.hover("nav");
  await ao.waitForTimeout(300);
  await ao.getByRole("button", { name: "Create", exact: true }).click();
  await ao.getByRole("button", { name: "Financing", exact: true }).click();
  await ao.waitForURL("**/assignment/*");
  await ao.mouse.move(900, 500);
  r.caseId = decodeURIComponent(ao.url().split("/case/")[1].split("/")[0]);
  await ao.getByLabel("Tipe Nasabah").selectOption("Perorangan");
  await ao.getByLabel("Data Pribadi NIK").fill("1111111121212121");
  await ao.getByLabel("Data Pribadi Nama").fill("John Doe");
  await ao.getByLabel("Data Pribadi Tanggal Lahir").fill("01/01/2000");
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForSelector("text=SLIK Request Data");
  await ao.getByRole("button", { name: "Dokumen Pre-Screening" }).click();
  for (const doc of ["KK", "KTP", "Form Aplikasi Pembiayaan"]) await uploadDoc(ao, doc);
  await ao.getByLabel("Tujuan Permintaan").selectOption({ label: "Penilaian Calon Debitur" });
  await ao.getByLabel("Pilihan Fasilitas").selectOption({ label: "Seluruh" });
  await ao.getByLabel("Alamat Nasabah").fill("Jl. Sudirman No. 1");
  await ao.getByLabel("Tipe Pencarian").selectOption({ label: "Individual" });
  await ao.getByLabel("Segmentasi").selectOption({ label: "Ritel" });
  await ao.locator("footer button", { hasText: "Create" }).click();
  await ao.waitForURL("**/app/fossyariah");

  // The case as WAITING_SLIK leaves it (FR-PSAPI-011 Process 1).
  const id = psql(`
    UPDATE fos.work SET case_status = 'Pending-Prescreening' WHERE case_id = '${r.caseId}';
    INSERT INTO fos.prescreening_runs (case_id, state, current_step, attempt, heartbeat_at, wait_deadline_at, started_at, updated_at)
    VALUES ('${r.caseId}', 'WAITING_SLIK', 'Pefindo', 1, now(), (now() AT TIME ZONE 'UTC') + interval '56 minutes 30 seconds', now(), now());
    INSERT INTO fos.slik_report (case_id, subject_type, reference_code, identity_number, status, requested_at, created_at, updated_at)
    VALUES ('${r.caseId}', 'CUSTOMER', 'SKL253/LOS002/10/2026/1', '1111111121212121', 'REQUESTED', now(), now(), now());
    INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
    VALUES ('${r.caseId}', 'SLIKWQFlowAct', 'worklist', 'pebrianto', 'baru', now()) RETURNING assignment_id;`);
  await ao.goto(`${base}/app/fossyariah/case/${r.caseId}/assignment/${id}`);
  await ao.mouse.move(900, 500);
  await ao.waitForSelector("text=Check SLIK Status");
  r.due = await ao.locator("text=/^Due \\d+ minutes from now$/").innerText();
  r.messageBefore = await ao.locator("text=Message").count();
  r.submit = await ao.locator("footer button", { hasText: "Submit" }).count();
  await ao.screenshot({ path: `${out}/waitslik-open.png` });
  await ao.getByRole("button", { name: "Check SLIK Status" }).click();
  await ao.waitForSelector("text=data not found");
  r.panels = await ao.locator("[class*=messageLabel]").allInnerTexts();
  await ao.screenshot({ path: `${out}/waitslik-checked.png` });

  psql(`INSERT INTO fos.slik_report (case_id, subject_type, reference_code, identity_number, status, requested_at, created_at, updated_at)
    VALUES ('${r.caseId}', 'SPOUSE', 'SKL253/LOS002/10/2026/2', '2222222222222222', 'REQUESTED', now(), now(), now());`);
  await ao.getByRole("button", { name: "Check SLIK Status" }).click();
  await ao.waitForFunction(() => document.querySelectorAll("[class*=messageLabel]").length === 2);
  r.spousePanels = await ao.locator("[class*=messageLabel]").allInnerTexts();
  await ao.screenshot({ path: `${out}/waitslik-spouse.png` });
  // Completed by the system (e.g. the results came in): the next press is
  // refused and the button goes away.
  psql(`UPDATE fos.assignments SET assignment_status = 'sudah selesai dikerjakan', completed_at = now() WHERE assignment_id = ${id};`);
  await ao.getByRole("button", { name: "Check SLIK Status" }).click();
  await ao.waitForSelector("[role=alert][class*=errorBanner]");
  r.afterCompletion = { button: await ao.locator("text=Check SLIK Status").count(), panels: await ao.locator("[class*=messageLabel]").count() };
  r.logs = psql(`SELECT api_name || ' ' || api_status || ' ' || request FROM fos.connect_api_log WHERE case_id = '${r.caseId}' ORDER BY submitted_at;`);
  console.log(JSON.stringify(r, null, 2));
  await browser.close();
  sfs.close();
})();
