// Ticket 08: UI-017 Pre Screening (Gambar 27, 28) and the UI-010 Notes
// banner after Tidak Sesuai (Gambar 29). Needs fakegcs.js, the SLIK_* BMV
// rows of create.js and operators pebrianto (AO) and TL01 (Team Lead).
// Usage: node review.js shots
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
  await ao.getByLabel("Data Pribadi NIK").fill("3674034512750017");
  await ao.getByLabel("Data Pribadi Nama").fill("John Doe");
  await ao.getByLabel("Data Pribadi Tanggal Lahir").fill("01/01/1975");
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForSelector("text=SLIK Request Data");
  await uploadMandatory(ao);
  await fillSLIK(ao);
  await ao.locator("footer button", { hasText: "Create" }).click();
  await ao.waitForURL("**/app/fossyariah");

  const tl = await login(browser, "TL01");
  await openAssignment(tl, r.caseId, "Pre Screening");
  await tl.waitForSelector("select[aria-label=Keputusan]");
  r.firstHistory = await tl.locator("text=History Pre-screening Notes").count();
  await tl.screenshot({ path: `${out}/review-1-form.png` });
  await tl.locator("footer button", { hasText: "Submit" }).click();
  await tl.waitForSelector("[role=alert][class*=errorBanner]");
  r.errors = await tl.locator("[role=alert][class*=errorBanner]").innerText();
  await tl.screenshot({ path: `${out}/review-2-errors.png` });
  await tl.getByLabel("Keputusan").selectOption({ label: "Tidak Sesuai" });
  await tl.getByLabel("Notes").fill("Terdapat typo pada nama.");
  await tl.locator("footer button", { hasText: "Submit" }).click();
  await tl.waitForURL("**/app/fossyariah");

  await openAssignment(ao, r.caseId, "Pre-Screening Submission");
  await ao.waitForSelector("[class*=notesBanner]");
  r.banner = await ao.locator("[class*=notesBanner]").innerText();
  r.typeDisabled = await ao.getByLabel("Tipe Nasabah").isDisabled();
  r.nikEditable = await ao.getByLabel("Data Pribadi NIK").isEditable();
  await ao.screenshot({ path: `${out}/review-3-banner.png` });
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForSelector("text=SLIK Request Data");
  await ao.locator("footer button", { hasText: "Create" }).click();
  await ao.waitForURL("**/app/fossyariah");

  await openAssignment(tl, r.caseId, "Pre Screening");
  await tl.waitForSelector("text=History Pre-screening Notes");
  await tl.locator("[class*=accordionHeader]", { hasText: "Tini Lestari" }).click();
  r.history = await tl.locator("[class*=historyBody]").innerText();
  await tl.screenshot({ path: `${out}/review-4-history.png` });
  console.log(JSON.stringify(r, null, 1));
  await browser.close();
})();
