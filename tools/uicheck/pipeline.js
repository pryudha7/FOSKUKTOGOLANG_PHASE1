// Ticket 15: UI-024 Pipeline Data Entry (Gambar 45-47). A case made through
// the UI up to Create is moved to Pipeline Data Entry through psql (as a
// completed run would), with test Jenis Pengajuan, products and banks; then
// errors, Take Over with the Nama Bank autocomplete, the Bukti Surat upload,
// a Jenis Pengajuan change deleting it, and Submit. Needs what create.js and
// duu.js need.
// Usage: node pipeline.js shots
const { chromium } = require("playwright-core");
const fs = require("fs");
const { execSync } = require("child_process");
const out = process.argv[2];
const psql = (sql) => execSync(`"${process.env.DOCKER || "docker"}" exec -i fos-smoke-pg psql -U postgres -qtA`, { input: sql }).toString().trim();
const base = "http://localhost:13000";

async function login(browser, user, width = 1500) {
  const ctx = await browser.newContext({ viewport: { width, height: 1100 }, extraHTTPHeaders: { "X-Forwarded-Proto": "https" } });
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
  fs.writeFileSync(`${out}/surat.pdf`, "%PDF-1.4 surat");
  psql(`INSERT INTO fos.business_mapping_value (code, type, sub_type, description, sort_order, status, created_at, created_by, updated_at, updated_by) VALUES
      ('New', 'SUBMISSION_TYPE', '', 'Pengajuan Baru', 1, 'Active', now(), 'x', now(), 'x'),
      ('Take Over', 'SUBMISSION_TYPE', '', 'Take Over', 2, 'Active', now(), 'x', now(), 'x'),
      ('014', 'BANK', '', 'BCA', 1, 'Active', now(), 'x', now(), 'x'),
      ('008', 'BANK', '', 'Bank Mandiri', 2, 'Active', now(), 'x', now(), 'x'),
      ('536', 'BANK', '', 'BCA Syariah', 3, 'Active', now(), 'x', now(), 'x')
    ON CONFLICT DO NOTHING;
    INSERT INTO fos.product_management (code, name, financing_limit_start, financing_limit_end, tenure_limit, contract_type, created_at, created_by, updated_at, updated_by) VALUES
      ('794', 'testing_mikro_aira_qardh', 0, 2000000000, 60, 'Qardh', now(), 'x', now(), 'x'),
      ('798', 'testing_mikro_aira_murabahah', 5000000000, 7000000000, 84, 'Murabahah', now(), 'x', now(), 'x')
    ON CONFLICT DO NOTHING;`);
  const browser = await chromium.launch({ executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe" });
  const r = {};
  const ao = await login(browser, "pebrianto");
  await ao.hover("nav");
  await ao.waitForTimeout(300);
  await ao.getByRole("button", { name: "Create", exact: true }).click();
  await ao.getByRole("button", { name: "Financing", exact: true }).click();
  await ao.waitForURL("**/assignment/*");
  await ao.mouse.move(900, 500);
  const c = decodeURIComponent(ao.url().split("/case/")[1].split("/")[0]);
  r.caseId = c;
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

  // As a completed run would (FR-APP-004 Assignment 1-2).
  const id = psql(`UPDATE fos.assignments SET assignment_status = 'sudah selesai dikerjakan', completed_at = now() WHERE case_id = '${c}' AND assignment_status <> 'sudah selesai dikerjakan';
    UPDATE fos.work SET case_status = 'Pending-Submission' WHERE case_id = '${c}';
    INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
    VALUES ('${c}', 'Pipeline Data Entry', 'worklist', 'pebrianto', 'baru', now() AT TIME ZONE 'UTC') RETURNING assignment_id;`);
  await ao.goto(`${base}/app/fossyariah/case/${c}/assignment/${id}`);
  await ao.mouse.move(900, 500);
  await ao.waitForSelector("text=Inisiasi");
  r.initialLabels = await ao.locator("[class*=pipelinePanel] [class*=fieldLabel], [class*=identityLabel]").allInnerTexts();
  r.submissionOptions = await ao.getByLabel("Jenis Pengajuan").locator("option").allInnerTexts();
  r.productOptions = await ao.getByLabel("Produk").locator("option").allInnerTexts();
  r.bankFieldBefore = await ao.getByLabel("Nama Bank").count();
  await ao.screenshot({ path: `${out}/pipeline-initial.png` });

  await ao.getByRole("button", { name: "Submit" }).click();
  await ao.waitForSelector("[class*=errorBanner] li");
  r.errors = await ao.locator("[class*=errorBanner] li").allInnerTexts();
  await ao.screenshot({ path: `${out}/pipeline-errors.png` });

  await ao.getByLabel("Jenis Pengajuan").selectOption({ label: "Take Over" });
  await ao.getByLabel("Nama Bank").waitFor();
  await ao.getByLabel("Nama Bank").fill("bca");
  await ao.waitForSelector("[role=listbox] [role=option]");
  r.suggestions = await ao.locator("[role=listbox] [role=option]").allInnerTexts();
  r.boldPart = await ao.locator("[role=listbox] [role=option] strong").first().innerText();
  await ao.screenshot({ path: `${out}/pipeline-takeover.png` });
  await ao.keyboard.press("Enter");
  r.bankChosen = await ao.getByLabel("Nama Bank").inputValue();
  await ao.getByLabel("Bukti Surat dari Bank file").setInputFiles(`${out}/surat.pdf`);
  await ao.waitForSelector("text=Bukti Surat Take Over dari Bank");
  r.letterCaption = await ao.locator("[class*=bankLetter] [class*=fileItemText] span").last().innerText();
  r.letterRows = psql(`SELECT document_category || '|' || document_type || '|' || file_name FROM fos.case_document WHERE case_id = '${c}' AND document_category = 'DokumenInisiasi';`);
  await ao.screenshot({ path: `${out}/pipeline-letter.png` });

  await ao.getByLabel("Jenis Pengajuan").selectOption({ label: "Pengajuan Baru" });
  await ao.waitForFunction(() => !document.body.innerText.includes("Bukti Surat dari Bank"));
  r.letterRowsAfterChange = psql(`SELECT count(*) FROM fos.case_document WHERE case_id = '${c}' AND document_category = 'DokumenInisiasi';`);
  await ao.getByLabel("Jenis Pengajuan").selectOption({ label: "Take Over" });
  r.bankAfterChange = await ao.getByLabel("Nama Bank").inputValue();

  await ao.getByLabel("Nama Bank").fill("Bank Lain");
  await ao.keyboard.press("Escape");
  await ao.getByLabel("Produk").selectOption({ label: "testing_mikro_aira_murabahah" });
  await ao.getByRole("button", { name: "Submit" }).click();
  await ao.waitForURL((u) => !u.toString().endsWith(`/assignment/${id}`)).catch(async (e) => {
    await ao.screenshot({ path: `${out}/pipeline-submit-failed.png`, fullPage: true });
    throw e;
  });
  r.stored = psql(`SELECT submission_type || '|' || product_code || '|' || contract_type || '|' || bank_name || '|' || case_status FROM fos.work WHERE case_id = '${c}';`);
  r.next = psql(`SELECT assignment_name || '|' || assignment_status FROM fos.assignments WHERE case_id = '${c}' ORDER BY assignment_id DESC LIMIT 1;`);

  // Below 768 px the fields stack in one column (BR-UI-008, PROPOSED).
  const narrow = await login(browser, "pebrianto", 700);
  const id2 = psql(`INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
    VALUES ('${c}', 'Pipeline Data Entry', 'worklist', 'pebrianto', 'baru', now() AT TIME ZONE 'UTC') RETURNING assignment_id;`);
  await narrow.goto(`${base}/app/fossyariah/case/${c}/assignment/${id2}`);
  await narrow.waitForSelector("text=Inisiasi");
  await narrow.screenshot({ path: `${out}/pipeline-narrow.png`, fullPage: true });

  console.log(JSON.stringify(r, null, 1));
  await browser.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
