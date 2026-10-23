// Ticket 33: SLIK Request Data, Create errors (ERR-010, ERR-020, ERR-026)
// and Create handing the case to the Team Lead. Needs fakegcs.js and the
// SLIK_* BMV rows (see README). Usage: node create.js shots
const { chromium } = require("playwright-core");
const fs = require("fs");
const { execSync } = require("child_process");
const out = process.argv[2];
const psql = (sql) => execSync(`"${process.env.DOCKER || "docker"}" exec -i fos-smoke-pg psql -U postgres -qtA`, { input: sql }).toString().trim();
const MB = 1024 * 1024;

(async () => {
  fs.writeFileSync(`${out}/big-kk.pdf`, Buffer.alloc(6 * MB));
  fs.writeFileSync(`${out}/big-ktp.pdf`, Buffer.alloc(4.5 * MB));
  fs.writeFileSync(`${out}/small.pdf`, "%PDF-1.4 small");
  const browser = await chromium.launch({ executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe" });
  const ctx = await browser.newContext({ viewport: { width: 1500, height: 1000 }, extraHTTPHeaders: { "X-Forwarded-Proto": "https" } });
  const page = await ctx.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  const r = {};
  await page.goto("http://localhost:13000/PRWebDB1");
  await page.fill('input[name="username"]', "pebrianto");
  await page.fill('input[name="password"]', "Rahasia#1");
  await page.click('button[type="submit"]');
  await page.waitForURL("**/app/fossyariah");
  await page.hover("nav");
  await page.waitForTimeout(300);
  await page.getByRole("button", { name: "Create", exact: true }).click();
  await page.getByRole("button", { name: "Financing", exact: true }).click();
  await page.waitForURL("**/assignment/*");
  await page.mouse.move(900, 500);
  await page.getByLabel("Tipe Nasabah").selectOption("Perorangan");
  await page.getByLabel("Data Pribadi NIK").fill("3171011234567890");
  await page.getByLabel("Data Pribadi Nama").fill("John Doe");
  await page.getByLabel("Data Pribadi Tanggal Lahir").fill("15/08/1990");
  await page.getByRole("button", { name: "Next" }).click();
  await page.waitForSelector("text=SLIK Request Data");
  r.caseId = decodeURIComponent(page.url().split("/case/")[1].split("/")[0]);
  r.placeholders = await page.locator("select option[disabled]").allInnerTexts();
  r.maxlength = await page.getByLabel("Alamat Nasabah").getAttribute("maxlength");
  await page.screenshot({ path: `${out}/create-1-page.png`, fullPage: true });

  const uploadFile = async (doc, file) => {
    await page.getByRole("button", { name: `Upload ${doc}` }).click();
    await page.locator("input[type=file]").setInputFiles(file);
    await page.getByRole("button", { name: "Submit" }).click();
    await page.waitForSelector("[role=dialog]", { state: "detached" });
    await page.waitForSelector(`[aria-label='Download ${doc}']`);
  };
  await page.getByRole("button", { name: "Dokumen Pre-Screening" }).click();
  await uploadFile("KK", `${out}/big-kk.pdf`);
  await uploadFile("KTP", `${out}/big-ktp.pdf`);
  // Close the accordion: the failed Create must open it again.
  await page.getByRole("button", { name: "Dokumen Pre-Screening" }).click();

  await page.locator("footer button", { hasText: "Create" }).click();
  await page.waitForSelector("[role=alert][class*=errorBanner]");
  r.banner = await page.locator("[role=alert][class*=errorBanner]").innerText();
  r.rowError = await page.locator("[class*=documentError]").allInnerTexts();
  r.accordionMarker = await page.locator("[class*=accordionError]").count();
  await page.screenshot({ path: `${out}/create-2-errors.png`, fullPage: true });

  await page.getByLabel("Tujuan Permintaan").selectOption({ label: "Penilaian Calon Debitur" });
  await page.getByLabel("Pilihan Fasilitas").selectOption({ label: "Seluruh" });
  await page.getByLabel("Alamat Nasabah").fill("Jl. Sudirman No. 1\nJakarta");
  await page.getByLabel("Tipe Pencarian").selectOption({ label: "Individual" });
  await page.getByLabel("Segmentasi").selectOption({ label: "Ritel" });
  r.bannerStillShown = (await page.locator("[role=alert][class*=errorBanner]").count()) > 0;
  await uploadFile("KTP", `${out}/small.pdf`);
  await uploadFile("Form Aplikasi Pembiayaan", `${out}/small.pdf`);
  await page.locator("footer button", { hasText: "Create" }).click();
  await page.waitForURL("**/app/fossyariah");
  r.afterCreate = page.url();
  r.assignments = psql(`SELECT assignment_name || ' ' || assignment_status || ' ' || coalesce(assigned_user_id, '') FROM fos.assignments WHERE case_id = '${r.caseId}' ORDER BY assignment_id;`);
  r.slik = psql(`SELECT slik_request_purpose_code || '|' || slik_facility_option_code || '|' || slik_identity_address || '|' || slik_search_type_code || '|' || slik_segmentation_code FROM fos.work WHERE case_id = '${r.caseId}';`);
  r.errors = errors;
  console.log(JSON.stringify(r, null, 1));
  await browser.close();
})();
