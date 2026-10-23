// Ticket 12: Manual SLIK Entry. A case whose SLIK wait timed out (set up
// through psql: the run STOPPED_SLIK_TIMEOUT, its SLIK log rows and the
// System's Input Manual) shows ERR-036 on Create until the SLIK document is
// uploaded; the Team Lead's Pre Screening then has the API Log History panel
// above History Pre-screening Notes and the decision (Varian Timeout SLIK).
// Needs what create.js and review.js need, and file storage as for duu.js.
// Usage: node slik.js shots
const { chromium } = require("playwright-core");
const fs = require("fs");
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

  // The case as the SLIK timeout leaves it (FR-PSAPI-011 Process 3).
  psql(`
    INSERT INTO fos.prescreening_runs (case_id, state, current_step, attempt, heartbeat_at, wait_deadline_at, started_at, updated_at)
    VALUES ('${r.caseId}', 'STOPPED_SLIK_TIMEOUT', 'Pefindo', 1, now() - interval '70 minutes', now() - interval '10 minutes', now() - interval '75 minutes', now());
    INSERT INTO fos.connect_api_log (log_id, case_id, api_type, api_name, submitted_at, api_status, request, response, attempt_number)
    VALUES (gen_random_uuid(), '${r.caseId}', 'ConnectRest', 'SLIK', now() - interval '72 minutes', 'Success', '{"kode_reff_pengguna":"SKL253/LOS002/10/2026/1"}', '{"code":"1100","message":"success"}', 1),
           (gen_random_uuid(), '${r.caseId}', 'Service Rest', 'SLIKPBSResponse', now() - interval '10 minutes', 'Fail', NULL, 'Timeout SLIK API No Response', NULL);
    INSERT INTO fos.case_pre_screening_decision (case_id, recommendation_type, decision_code, notes, decided_by, decided_by_role_code, decided_at)
    VALUES ('${r.caseId}', 'Pre Screening', 'INPUTMANUAL', 'Hasil SLIK belum diterima sistem lebih dari waktu tunggu. Harap lanjutkan untuk unggah hasil SLIK secara manual.', 'System', 'System', now() - interval '10 minutes');`);

  await ao.locator("footer button", { hasText: "Create" }).click();
  const banner = ao.locator("[role=alert][class*=errorBanner]");
  await banner.waitFor();
  r.err036 = (await banner.innerText()).trim();
  await ao.screenshot({ path: `${out}/slik-err036.png`, fullPage: true });

  await uploadDoc(ao, "SLIK");
  await ao.locator("footer button", { hasText: "Create" }).click();
  await ao.waitForURL("**/app/fossyariah");

  const tl = await login(browser, "TL01");
  const id = psql(`SELECT max(assignment_id) FROM fos.assignments WHERE case_id = '${r.caseId}' AND assignment_name = 'Pre Screening';`);
  await tl.goto(`${base}/app/fossyariah/case/${r.caseId}/assignment/${id}`);
  await tl.mouse.move(900, 500);
  await tl.waitForSelector("text=API Log History");
  r.panelBeforeHistory = await tl.evaluate(() => {
    const heads = [...document.querySelectorAll("h3")].map((h) => h.textContent);
    return heads.indexOf("API Log History") >= 0 && heads.indexOf("API Log History") < heads.indexOf("History Pre-screening Notes");
  });
  r.retryText = await tl.locator("text=Server failed for this API").count();
  r.decisionField = await tl.locator("select[aria-label=Keputusan]").count();
  r.logRows = await tl.locator("table tbody tr").allInnerTexts();
  await tl.screenshot({ path: `${out}/slik-timeout-review.png`, fullPage: true });
  console.log(JSON.stringify(r, null, 2));
  await browser.close();
})();
