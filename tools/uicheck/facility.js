// Ticket 19: UI-032 Data Permohonan Fasilitas (Gambar 74-77). A case made
// through the UI up to Create is moved to Financing Application through
// psql with product 794 (Qardh, limits 0 to 2.000.000.000) and an open Data
// Permohonan Fasilitas; then the empty page, the computed values following
// the typed ones, the read-only approval fields, the five-decimal margin,
// Effective and Flat, the Credit Simulation modal, ERR-010, ERR-042 and Next
// to QCA Data. Needs what create.js and duu.js need.
// Usage: node facility.js shots
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

(async () => {
  fs.writeFileSync(`${out}/small.pdf`, "%PDF-1.4 small");
  psql(`INSERT INTO fos.business_mapping_value (code, type, sub_type, description, sort_order, status, created_at, created_by, updated_at, updated_by) VALUES
      ('MK', 'DPFHPDE', '', 'Modal Kerja', 1, 'Active', now(), 'x', now(), 'x'),
      ('INV', 'DPFHPDE', '', 'Investasi', 2, 'Active', now(), 'x', now(), 'x')
    ON CONFLICT DO NOTHING;
    INSERT INTO fos.product_management (code, name, financing_limit_start, financing_limit_end, tenure_limit, contract_type, created_at, created_by, updated_at, updated_by)
    VALUES ('794', 'testing_mikro_aira_qardh', 0, 2000000000, 60, 'Qardh', now(), 'x', now(), 'x')
    ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, financing_limit_start = EXCLUDED.financing_limit_start,
      financing_limit_end = EXCLUDED.financing_limit_end, contract_type = EXCLUDED.contract_type;`);
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
  await ao.getByLabel("Data Pribadi NIK").fill("1111111111111191");
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

  // As Next on Data Pembiayaan Lain would, the product chosen on Pipeline.
  const fid = psql(`UPDATE fos.assignments SET assignment_status = 'sudah selesai dikerjakan', completed_at = now() WHERE case_id = '${c}' AND assignment_status <> 'sudah selesai dikerjakan';
    INSERT INTO fos.case_stage_progress (case_id, stage_name, first_entered_at) VALUES ('${c}', 'Financing Application', now() AT TIME ZONE 'UTC') ON CONFLICT DO NOTHING;
    UPDATE fos.work SET current_stage = 'Financing Application', case_status = 'Pending-Submission', product_code = '794', contract_type = 'Qardh' WHERE case_id = '${c}';
    INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
    VALUES ('${c}', 'Data Permohonan Fasilitas', 'worklist', 'pebrianto', 'baru', now() AT TIME ZONE 'UTC') RETURNING assignment_id;`);

  const value = (label) => ao.getByLabel(label, { exact: true }).first().innerText();
  const input = (label, nth = 0) => ao.getByLabel(label, { exact: true }).nth(nth);
  const simulate = ao.getByRole("button", { name: "Hitung Simulasi Angsuran" });
  const settle = () => ao.waitForTimeout(700);

  // AC-FR-APP-018-01 (Gambar 74, 75).
  await ao.goto(`${base}/app/fossyariah/case/${c}/assignment/${fid}`);
  await ao.mouse.move(1400, 1000);
  await ao.waitForSelector("text=Fasilitas Pemohonan Nasabah");
  await settle();
  r.empty = [await value("Total Requested Amount"), await value("Total Obligor"), await value("Angsuran Per Bulan"), await simulate.isDisabled(),
    await input("Tipe Akad").isDisabled(), await input("Tipe Akad").inputValue()];
  // The steps after this one before its recommendation (ticket 19 review).
  r.crumbsPastDisabled = [await ao.getByRole("button", { name: "QCA Data" }).isDisabled(), await ao.getByRole("button", { name: "Data Pembiayaan Lain" }).isDisabled()];
  await ao.screenshot({ path: `${out}/facility-empty.png`, fullPage: true });

  // AC-02, AC-03.
  await ao.getByLabel("Modal Kerja").check();
  await input("Pembiayaan yang di ajukan").fill("100000000");
  await input("Jangka Waktu (Bulan)").fill("12");
  await input("Jumlah Pembiayaan Eksisting di Bank Nano").fill("0");
  r.followed = [await value("Total Requested Amount"), await value("Total Obligor"), await input("Pembiayaan Yang Disetujui").inputValue(),
    await input("Jangka Waktu (Bulan)", 1).inputValue()];
  await input("Pembiayaan Yang Disetujui").pressSequentially("9");
  r.approvedTyped = await input("Pembiayaan Yang Disetujui").inputValue();

  // AC-04, AC-09 (a sixth decimal cannot be typed), AC-06.
  await input("Tipe Margin").selectOption({ label: "Effective" });
  await input("Margin / Ujrah (%)").pressSequentially("12,000001");
  r.marginTyped = await input("Margin / Ujrah (%)").inputValue();
  await input("Notes").click();
  r.marginShown = await input("Margin / Ujrah (%)").inputValue();
  await settle();
  r.effective = [await value("Angsuran Per Bulan"), await value("Margin Flat (%)"), await simulate.isDisabled()];
  await input("Notes").fill("Layak dibiayai");
  await ao.screenshot({ path: `${out}/facility-effective.png`, fullPage: true });
  await input("Jangka Waktu (Bulan)").fill("24");
  await settle();
  r.tenure24 = await value("Angsuran Per Bulan");
  await input("Jangka Waktu (Bulan)").fill("12");

  // AC-05.
  await input("Tipe Margin").selectOption({ label: "Flat" });
  await input("Margin / Ujrah (%)").fill("6,61855");
  await settle();
  r.flat = [await value("Angsuran Per Bulan"), await value("Margin Efektif (%)")];

  // AC-10, AC-11 (Gambar 77).
  await simulate.click();
  await ao.waitForSelector("[role=dialog]");
  r.modal = await ao.locator("[role=dialog] dl").innerText();
  await ao.screenshot({ path: `${out}/facility-simulation.png` });
  await ao.locator("[role=dialog]").getByRole("button", { name: "Home" }).click();
  r.afterHome = [await ao.locator("[role=dialog]").count(), ao.url().endsWith(`/assignment/${fid}`), await input("Margin / Ujrah (%)").inputValue()];
  await simulate.click();
  await ao.getByRole("button", { name: "Tutup" }).click();
  r.afterX = await ao.locator("[role=dialog]").count();

  // AC-18.
  await input("Margin / Ujrah (%)").fill("");
  await settle();
  r.cleared = [await simulate.isDisabled(), await value("Angsuran Per Bulan"), await value("Margin Efektif (%)")];
  await input("Margin / Ujrah (%)").fill("6,61855");

  // AC-12 and ERR-042 above the product's 2.000.000.000.
  await input("Notes").fill("");
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForSelector("[class*=errorBanner] li");
  r.notesError = await ao.locator("[class*=errorBanner] li").allInnerTexts();
  await input("Notes").fill("Layak dibiayai");
  await input("Pembiayaan yang di ajukan").fill("3000000000");
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForFunction(() => document.body.innerText.includes("batas atas"));
  r.limitError = await ao.locator("[class*=errorBanner] li").allInnerTexts();
  await ao.screenshot({ path: `${out}/facility-limit.png`, fullPage: true });
  r.stored = psql(`SELECT count(*) FROM fos.case_financing_request WHERE case_id = '${c}';`);

  // AC-13: Next to QCA Data.
  await input("Pembiayaan yang di ajukan").fill("100000000");
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForURL((u) => !u.toString().includes(`/assignment/${fid}`) && u.toString().includes("/assignment/"));
  await ao.waitForTimeout(500);
  r.next = psql(`SELECT assignment_name || '|' || assignment_status FROM fos.assignments WHERE case_id = '${c}' ORDER BY assignment_id DESC LIMIT 1;`);
  r.recommendation = psql(`SELECT concat_ws('|', d.recommendation_status, d.requested_amount, d.requested_margin_effective, s.debtor_name, s.installment_flat)
    FROM fos.case_pre_screening_decision d JOIN fos.recommendation_credit_simulation s ON s.decision_id = d.decision_id WHERE d.case_id = '${c}';`);

  console.log(JSON.stringify(r, null, 1));
  await browser.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
