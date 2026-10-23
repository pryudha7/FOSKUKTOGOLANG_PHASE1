// Ticket 07: UI-011 document list, UI-015 upload modal, document icon
// download, attachments panel and ERR-035 popup. Needs fakegcs.js running
// (see README). Usage: node duu.js shots
const { chromium } = require("playwright-core");
const fs = require("fs");
const { execSync } = require("child_process");
const out = process.argv[2];
const psql = (sql) => execSync(`"${process.env.DOCKER || "docker"}" exec -i fos-smoke-pg psql -U postgres -qtA`, { input: sql }).toString().trim();

(async () => {
  fs.writeFileSync(`${out}/kk.pdf`, "%PDF-1.4 uicheck kk");
  fs.copyFileSync(`${__dirname}/../../docs/ui-reference/gambar-043.png`, `${out}/ktp scan.png`);
  const browser = await chromium.launch({ executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe" });
  const ctx = await browser.newContext({ viewport: { width: 1500, height: 1000 }, acceptDownloads: true, extraHTTPHeaders: { "X-Forwarded-Proto": "https" } });
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
  await page.waitForFunction(() => document.querySelector("h2")?.textContent === "Document Underlying Upload");
  await page.waitForSelector("text=Nomor Memo");
  r.caseId = decodeURIComponent(page.url().split("/case/")[1].split("/")[0]);
  r.memo = await page.locator("[class*=readOnlyValue]").innerText();
  await page.screenshot({ path: `${out}/duu-1-page.png`, fullPage: true });

  await page.getByRole("button", { name: "Dokumen Pre-Screening" }).click();
  await page.getByRole("cell", { name: "KTP", exact: true }).click();
  r.rows = await page.locator("[class*=documentTable] tbody tr").allInnerTexts();
  await page.screenshot({ path: `${out}/duu-2-open.png`, fullPage: true });

  await page.getByRole("button", { name: "Upload KK" }).click();
  await page.waitForSelector("[role=dialog]");
  await page.screenshot({ path: `${out}/duu-3-modal.png` });
  await page.locator("input[type=file]").setInputFiles(`${out}/kk.pdf`);
  r.defaultName = await page.getByLabel("Name").inputValue();
  r.category = await page.getByLabel("Category").locator("option").innerText();
  r.categoryDisabled = await page.getByLabel("Category").isDisabled();
  await page.getByLabel("Name").fill("");
  await page.getByRole("button", { name: "Submit" }).click();
  await page.waitForSelector("[role=dialog] [role=alert]");
  r.err023 = await page.locator("[role=dialog] [role=alert]").innerText();
  await page.screenshot({ path: `${out}/duu-4-err023.png` });
  await page.getByLabel("Name").fill("KK Budi");
  await page.getByRole("button", { name: "Submit" }).click();
  await page.waitForSelector("[role=dialog]", { state: "detached" });
  await page.waitForSelector("[aria-label='Download KK']");

  await page.getByRole("button", { name: "Upload KTP" }).click();
  await page.locator("input[type=file]").setInputFiles(`${out}/ktp scan.png`);
  r.pngName = await page.getByLabel("Name").inputValue();
  await page.getByRole("button", { name: "Submit" }).click();
  await page.waitForSelector("[aria-label='Download KTP']");
  await page.locator("[class*=documentTable] tbody tr").nth(0).locator("td").nth(1).click();
  await page.screenshot({ path: `${out}/duu-5-icons.png`, fullPage: true });
  r.kkIconColor = await page.locator("[aria-label='Download KK']").evaluate((e) => getComputedStyle(e).color);
  r.ktpIconColor = await page.locator("[aria-label='Download KTP']").evaluate((e) => getComputedStyle(e).color);

  const dl = page.waitForEvent("download");
  await page.locator("[aria-label='Download KK']").click();
  const d = await dl;
  r.downloadName = d.suggestedFilename();
  r.downloadBody = fs.readFileSync(await d.path(), "utf8");
  // Clicking the name downloads nothing (AC-FR-FILE-003-06).
  let extra = false;
  page.once("download", () => (extra = true));
  await page.getByText("KK", { exact: true }).click();
  await page.waitForTimeout(500);
  r.nameClickDownloads = extra;

  await page.getByRole("button", { name: "Open files and documents" }).click();
  await page.waitForTimeout(500);
  r.panel = await page.locator("aside[class*=attachmentsOpen]").innerText();
  await page.screenshot({ path: `${out}/duu-6-panel.png` });

  // ERR-035: the operator loses access while the page is open.
  psql(`UPDATE fos.work SET created_by = 'TL01' WHERE case_id = '${r.caseId}'; UPDATE fos.assignments SET assigned_user_id = 'TL01' WHERE case_id = '${r.caseId}';`);
  await page.locator("[aria-label='Download KTP']").click();
  await page.waitForSelector("[role=alertdialog]");
  r.popup = await page.locator("[role=alertdialog]").innerText();
  await page.screenshot({ path: `${out}/duu-7-err035.png` });
  await page.getByRole("button", { name: "OK", exact: true }).click();
  r.popupClosed = (await page.locator("[role=alertdialog]").count()) === 0;
  r.errors = errors;
  console.log(JSON.stringify(r, null, 1));
  await browser.close();
})();
