// Ticket 20: UI-033 QCA Data (Gambar 79). A case made through the UI up to
// Create is moved through psql to an open QCA Data, with its Data
// Permohonan Fasilitas recommendation and test options for QCA1 to QCA10;
// then the empty page, the option order, ERR-010 on question 3, Save, Next
// to Collateral Data, and the page reopened with an answer now Inactive.
// Needs what create.js and duu.js need.
// Usage: node qca.js shots
const { chromium } = require("playwright-core");
const fs = require("fs");
const { execSync } = require("child_process");
const out = process.argv[2];
const psql = (sql) => execSync(`"${process.env.DOCKER || "docker"}" exec -i fos-smoke-pg psql -U postgres -qtA`, { input: sql }).toString().trim();
const base = "http://localhost:13000";

async function login(browser, user) {
  const ctx = await browser.newContext({ viewport: { width: 1500, height: 1100 }, extraHTTPHeaders: { "X-Forwarded-Proto": "https" } });
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

const questions = [
  "Bagaimana hubungan pemilik dengan Bank Nano Syariah?",
  "Apa tujuan dari pembiayaan tersebut?",
  "Berapa lama usaha telah beroperasi?",
  "Bagaimana penampilan tempat tinggal pemohon?",
  "Bagaimana kondisi keseluruhan lokasi usaha?",
  "Apakah pemilik memiliki sertifikat atas lokasi usahanya?",
  "Jika tempat usaha adalah sewa, berapa persen biaya sewa dari pendapatan bulanan?",
  "Bagaimana jarak lokasi usaha/bisnis pemohon dengan cabang?",
  "Apa saja sumber pemasukan dalam keluarga selain dari pemasukan usaha pemohon?",
  "Berapa pemasukan pemilik usaha yang digunakan untuk biaya rumah tangga?",
];

(async () => {
  fs.writeFileSync(`${out}/small.pdf`, "%PDF-1.4 small");
  const options = [];
  for (let n = 1; n <= 10; n++) {
    options.push(`('Q${n}A', 'QCA${n}', '', 'Jawaban ${n} A', 2, 'Active', now(), 'x', now(), 'x')`);
    options.push(`('Q${n}B', 'QCA${n}', '', 'Jawaban ${n} B', 1, 'Active', now(), 'x', now(), 'x')`);
  }
  psql(`INSERT INTO fos.business_mapping_value (code, type, sub_type, description, sort_order, status, created_at, created_by, updated_at, updated_by) VALUES
    ${options.join(",\n")} ON CONFLICT DO NOTHING;
    UPDATE fos.business_mapping_value SET status = 'Active' WHERE type LIKE 'QCA%';`);
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
  await ao.getByLabel("Data Pribadi NIK").fill("1111111111111192");
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

  // As Next on Data Permohonan Fasilitas would.
  const qid = psql(`UPDATE fos.assignments SET assignment_status = 'sudah selesai dikerjakan', completed_at = now() WHERE case_id = '${c}' AND assignment_status <> 'sudah selesai dikerjakan';
    INSERT INTO fos.case_stage_progress (case_id, stage_name, first_entered_at) VALUES ('${c}', 'Financing Application', now() AT TIME ZONE 'UTC') ON CONFLICT DO NOTHING;
    UPDATE fos.work SET current_stage = 'Financing Application', case_status = 'Pending-Submission' WHERE case_id = '${c}';
    INSERT INTO fos.case_pre_screening_decision (case_id, recommendation_type, notes, decided_by, decided_by_role_code, decided_at)
    VALUES ('${c}', 'Data Permohonan Fasilitas', 'Layak', 'pebrianto', 'AO', now() AT TIME ZONE 'UTC');
    INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
    VALUES ('${c}', 'QCA Data', 'worklist', 'pebrianto', 'baru', now() AT TIME ZONE 'UTC') RETURNING assignment_id;`);
  const select = (i) => ao.getByLabel(questions[i], { exact: true });

  // AC-FR-APP-020-01, -03 (Gambar 79).
  await ao.goto(`${base}/app/fossyariah/case/${c}/assignment/${qid}`);
  await ao.mouse.move(1400, 1000);
  await ao.waitForSelector("text=Berapa lama usaha telah beroperasi?");
  r.selects = await ao.locator("select").count();
  r.shown = await Promise.all(questions.map((_, i) => select(i).locator("option:checked").innerText()));
  r.q1Options = await select(0).locator("option").allInnerTexts();
  r.crumbsOpen = await ao.getByRole("button", { name: "Collateral Data" }).isEnabled();
  await ao.screenshot({ path: `${out}/qca-empty.png`, fullPage: true });

  // AC-02 and Save.
  for (let i = 0; i < 10; i++) if (i !== 2) await select(i).selectOption({ label: `Jawaban ${i + 1} A` });
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForSelector("[class*=errorBanner] li");
  r.error = [await ao.locator("[class*=errorBanner] li").allInnerTexts(), await ao.locator("[class*=fieldError]").allInnerTexts()];
  await ao.screenshot({ path: `${out}/qca-error.png`, fullPage: true });
  await ao.getByRole("button", { name: "Save" }).click();
  await ao.waitForTimeout(800);
  r.afterSave = [ao.url().endsWith(`/assignment/${qid}`), psql(`SELECT count(*) FROM fos.case_qca_answer WHERE case_id = '${c}';`)];

  // AC-04: Next to Collateral Data.
  await select(2).selectOption({ label: "Jawaban 3 B" });
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForURL((u) => !u.toString().includes(`/assignment/${qid}`) && u.toString().includes("/assignment/"));
  await ao.waitForTimeout(500);
  r.rows = psql(`SELECT string_agg(answer_code, ',' ORDER BY question_number) FROM fos.case_qca_answer WHERE case_id = '${c}';`);
  r.next = psql(`SELECT assignment_name || '|' || assignment_status FROM fos.assignments WHERE case_id = '${c}' ORDER BY assignment_id DESC LIMIT 1;`);

  // AC-06 with an answer now Inactive (BR-UI-012).
  psql(`UPDATE fos.business_mapping_value SET status = 'Inactive' WHERE type = 'QCA1' AND code = 'Q1A';
    UPDATE fos.assignments SET assignment_status = 'sudah dibuka', completed_at = NULL WHERE assignment_id = ${qid};`);
  await ao.goto(`${base}/app/fossyariah/case/${c}/assignment/${qid}`);
  await ao.waitForSelector("text=Berapa lama usaha telah beroperasi?");
  r.reopened = [await select(0).locator("option:checked").innerText(), await select(2).locator("option:checked").innerText()];
  await ao.screenshot({ path: `${out}/qca-filled.png`, fullPage: true });

  console.log(JSON.stringify(r, null, 1));
  await browser.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
