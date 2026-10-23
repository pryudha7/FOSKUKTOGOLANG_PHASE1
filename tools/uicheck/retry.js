// Ticket 09: Sesuai starts the Pre-Screening Check; Dukcapil pointed at a
// closed port fails four times and the Team Lead gets the Retry view
// (Gambar 35). Needs what review.js needs, plus api/dukcapil/baseurl set to
// a closed port and DUKCAPIL_APIKEY in the backend environment.
// Usage: node retry.js shots
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

const openAssignment = async (page, caseId, name) => {
  const id = psql(`SELECT max(assignment_id) FROM fos.assignments WHERE case_id = '${caseId}' AND assignment_name = '${name}';`);
  await page.goto(`${base}/app/fossyariah/case/${caseId}/assignment/${id}`);
  await page.mouse.move(900, 500);
};

async function uploadMandatory(page) {
  await page.getByRole("button", { name: "Dokumen Pre-Screening" }).click();
  for (const doc of ["KK", "KTP", "Form Aplikasi Pembiayaan"]) {
    if ((await page.locator(`[aria-label='Download ${doc}']`).count()) > 0) continue;
    await page.getByRole("button", { name: `Upload ${doc}` }).click();
    await page.locator("input[type=file]").setInputFiles(`${out}/small.pdf`);
    await page.getByRole("button", { name: "Submit" }).click();
    await page.waitForSelector(`[aria-label='Download ${doc}']`);
  }
}

async function fillSLIK(page) {
  await page.getByLabel("Tujuan Permintaan").selectOption({ label: "Penilaian Calon Debitur" });
  await page.getByLabel("Pilihan Fasilitas").selectOption({ label: "Seluruh" });
  await page.getByLabel("Alamat Nasabah").fill("Jl. Sudirman No. 1");
  await page.getByLabel("Tipe Pencarian").selectOption({ label: "Individual" });
  await page.getByLabel("Segmentasi").selectOption({ label: "Ritel" });
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
  await uploadMandatory(ao);
  await fillSLIK(ao);
  await ao.locator("footer button", { hasText: "Create" }).click();
  await ao.waitForURL("**/app/fossyariah");

  const tl = await login(browser, "TL01");
  await openAssignment(tl, r.caseId, "Pre Screening");
  await tl.waitForSelector("select[aria-label=Keputusan]");
  await tl.getByLabel("Keputusan").selectOption({ label: "Sesuai" });
  await tl.getByLabel("Notes").fill("Data lengkap.");
  const t0 = Date.now();
  await tl.locator("footer button", { hasText: "Submit" }).click();
  await tl.waitForURL("**/app/fossyariah");
  r.submitMs = Date.now() - t0;
  for (let i = 0; i < 100 && psql(`SELECT case_status FROM fos.work WHERE case_id = '${r.caseId}';`) !== "Failed-PrescreeningConnection"; i++) {
    await tl.waitForTimeout(200);
  }
  r.status = psql(`SELECT case_status FROM fos.work WHERE case_id = '${r.caseId}';`);
  await openAssignment(tl, r.caseId, "Pre Screening");
  await tl.waitForSelector("text=API Log History");
  r.retryText = await tl.locator("[class*=retryText]").innerText();
  r.rows = await tl.locator("[class*=logTable] tbody tr").count();
  r.decisionShown = await tl.locator("select[aria-label=Keputusan]").count();
  await tl.screenshot({ path: `${out}/retry-1-view.png`, fullPage: true });
  await tl.locator("footer button", { hasText: "Submit" }).click();
  await tl.waitForURL("**/app/fossyariah");
  r.afterRetry = psql(`SELECT case_status FROM fos.work WHERE case_id = '${r.caseId}';`);
  console.log(JSON.stringify(r, null, 1));
  await browser.close();
})();
