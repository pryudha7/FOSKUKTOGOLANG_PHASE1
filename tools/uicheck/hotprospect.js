// Ticket 18: UI-030 Data Usaha Nasabah and UI-031 Data Pembiayaan Lain
// (Gambar 71-73). A case made through the UI up to Create is moved to
// Financing Application through psql (as Create on Data Keluarga Dekat
// would), with a received customer SLIK holding three facilities and test
// master data; then errors, Save, Next, the margin field, Isi Data dari SLIK
// (accordions, ticks, Submit), Tambah, delete and Next. Needs what create.js
// and duu.js need.
// Usage: node hotprospect.js shots
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

const bmv = (type, code, description, sort, value1 = null, subType = "") =>
  `('${code}', '${type}', '${subType}', '${description}', ${sort}, ${value1 === null ? "NULL" : `'${value1}'`}, 'Active', now(), 'x', now(), 'x')`;
const select = (page, label, text, nth = 0) => page.getByLabel(label, { exact: true }).nth(nth).selectOption({ label: text });

(async () => {
  fs.writeFileSync(`${out}/small.pdf`, "%PDF-1.4 small");
  psql(`INSERT INTO fos.business_mapping_value (code, type, sub_type, description, sort_order, value1, status, created_at, created_by, updated_at, updated_by) VALUES
    ${[
      bmv("BUSINESSCATEGORY", "BC1", "Perdagangan", 1),
      bmv("BUSINESSLOCATION", "BL1", "Ruko", 1),
      bmv("HOUSE_OWNERSHIP_STATUS", "HO1", "Milik Sendiri", 1),
      bmv("PROVINCE", "31", "DKI Jakarta", 1),
      bmv("CITY", "3171", "Jakarta Selatan", 1, "JAKARTA SELATAN", "31"),
      bmv("DISTRICT", "317101", "Tebet", 1, null, "3171"),
      bmv("SUBDISTRICT", "3171011", "Manggarai", 1, null, "317101"),
      bmv("POSTALCODE", "12850", "12850", 1, null, "3171011"),
    ].join(",\n")}
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
  await ao.getByLabel("Data Pribadi NIK").fill("1111111111111190");
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

  // As Create on Data Keluarga Dekat would, with the customer's SLIK received.
  const did = psql(`UPDATE fos.assignments SET assignment_status = 'sudah selesai dikerjakan', completed_at = now() WHERE case_id = '${c}' AND assignment_status <> 'sudah selesai dikerjakan';
    INSERT INTO fos.case_stage_progress (case_id, stage_name, first_entered_at) VALUES ('${c}', 'Financing Application', now() AT TIME ZONE 'UTC') ON CONFLICT DO NOTHING;
    UPDATE fos.work SET current_stage = 'Financing Application', case_status = 'Pending-Submission' WHERE case_id = '${c}';
    INSERT INTO fos.slik_report (case_id, subject_type, reference_code, identity_number, status, created_at, updated_at) VALUES ('${c}', 'CUSTOMER', 'UI-${c}', '1111111111111190', 'RECEIVED', now(), now());
    INSERT INTO fos.slik_facility (slik_report_id, sequence_number, financial_institution_description, credit_financing_type_description, initial_credit_limit,
      outstanding_principal, credit_start_date, due_date, interest_rate, quality, usage_type_description, condition, condition_description, detail_json)
    SELECT slik_report_id, f.seq, f.bank, f.kind, f.amount, f.outstanding, f.started::date, f.due::date, f.rate, f.quality, f.usage, f.cond, f.cond_ket, '{}'::jsonb
    FROM fos.slik_report, (VALUES
      (1, 'PT Bank Mega Tbk', 'Kartu Kredit/ Kartu Pembiayaan Syariah', 7000000::numeric, 9822753::numeric, '2014-01-15', '2027-01-15', 21::numeric, 1::smallint, 'Konsumsi', '00', 'Aktif'),
      (2, 'Bank Kedua', 'Kredit Modal Kerja', 50000000, 12500000, '2024-01-01', '2026-01-01', 12.5, 1::smallint, 'Modal Kerja', '00', 'Aktif'),
      (3, 'Bank Lama', 'KPR', 300000000, 0, '2005-01-01', '2020-01-01', 9, 1::smallint, 'Investasi', '02', 'Lunas')
    ) AS f (seq, bank, kind, amount, outstanding, started, due, rate, quality, usage, cond, cond_ket)
    WHERE reference_code = 'UI-${c}';
    INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
    VALUES ('${c}', 'Data Usaha Nasabah', 'worklist', 'pebrianto', 'baru', now() AT TIME ZONE 'UTC') RETURNING assignment_id;`);

  // UI-030.
  await ao.goto(`${base}/app/fossyariah/case/${c}/assignment/${did}`);
  await ao.mouse.move(1400, 1000);
  await ao.waitForSelector("text=Jenis Bidang Usaha");
  r.previousStepDisabled = await ao.getByRole("button", { name: "Back to Previous Step" }).isDisabled();
  await ao.screenshot({ path: `${out}/hot-business.png`, fullPage: true });
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForSelector("[class*=errorBanner] li");
  r.errors = (await ao.locator("[class*=errorBanner] li").allInnerTexts()).length;
  await ao.getByLabel("Jenis Bidang Usaha", { exact: true }).fill("Toko Kelontong");
  await ao.getByLabel("Lama Usaha (Bulan)", { exact: true }).fill("120000");
  r.ageTyped = await ao.getByLabel("Lama Usaha (Bulan)", { exact: true }).inputValue();
  await ao.getByRole("button", { name: "Save" }).click();
  await ao.waitForTimeout(800);
  r.afterSave = [ao.url().endsWith(`/assignment/${did}`), await ao.locator("[class*=errorBanner] li").count()];
  await select(ao, "Kategori Bidang Usaha", "Perdagangan");
  await ao.getByLabel("Jumlah Karyawan", { exact: true }).fill("25");
  await select(ao, "Jenis Tempat Usaha", "Ruko");
  await ao.getByLabel("Alamat Usaha", { exact: true }).fill("Jl. Pasar 3");
  await select(ao, "Status Kepemilikan Tempat Usaha", "Milik Sendiri");
  await ao.getByLabel("Lama Tempat Usaha (Tahun)", { exact: true }).fill("10");
  await ao.getByLabel("Lama Tempat Usaha (Bulan)", { exact: true }).fill("6");
  await ao.getByLabel("RT", { exact: true }).fill("007");
  await ao.getByLabel("RW", { exact: true }).fill("008");
  await select(ao, "Provinsi", "DKI Jakarta");
  await select(ao, "Kota", "Jakarta Selatan");
  await select(ao, "Kecamatan", "Tebet");
  await select(ao, "Kelurahan", "Manggarai");
  await ao.waitForFunction(() => document.body.innerText.includes("12850"));
  await ao.getByRole("button", { name: "Next" }).click();

  // UI-031.
  await ao.waitForSelector("text=Tambah Pembiayaan");
  await ao.mouse.move(1400, 1000);
  r.business = psql(`SELECT concat_ws('|', business_field, business_age_months, employee_count) FROM fos.customer_business b JOIN fos.customer c ON c.customer_id = b.customer_id WHERE c.case_id = '${c}';`);
  await ao.screenshot({ path: `${out}/hot-financing.png`, fullPage: true });
  await ao.getByLabel("Margin / Ujrah (%)", { exact: true }).fill("21");
  await ao.getByLabel("Nama Bank", { exact: true }).click();
  r.marginShown = await ao.getByLabel("Margin / Ujrah (%)", { exact: true }).inputValue();
  await ao.getByRole("button", { name: "Isi Data dari SLIK" }).click();
  await ao.waitForSelector("text=Fasilitas Aktif (2)");
  r.groups = await ao.locator("[class*=slikGroupToggle]").allInnerTexts();
  await ao.getByRole("button", { name: /Fasilitas Aktif/ }).click();
  await ao.screenshot({ path: `${out}/hot-slik.png` });
  await ao.getByLabel("Pilih semua Fasilitas Aktif (2)").check();
  r.bothTicked = await ao.locator("[class*=slikFacilityTick]:checked").count();
  await ao.getByLabel("Pilih Bank Kedua").uncheck();
  r.headerAfterUntick = await ao.getByLabel("Pilih semua Fasilitas Aktif (2)").isChecked();
  await ao.getByLabel("Pilih Bank Kedua").check();
  r.headerAfterRetick = await ao.getByLabel("Pilih semua Fasilitas Aktif (2)").isChecked();
  await ao.getByRole("button", { name: "Submit" }).click();
  r.cards = await ao.getByRole("group", { name: /Pembiayaan/ }).count();
  r.firstCard = [await ao.getByLabel("Nama Bank", { exact: true }).first().inputValue(), await ao.getByLabel("Jangka Waktu (Bulan)", { exact: true }).first().inputValue(),
    await ao.getByLabel("Margin / Ujrah (%)", { exact: true }).first().inputValue(), await ao.getByLabel("Jumlah Outstanding", { exact: true }).first().inputValue()];
  await ao.getByRole("button", { name: "Tambah Pembiayaan" }).click();
  await ao.getByRole("button", { name: "Hapus Pembiayaan 3" }).click();
  await ao.screenshot({ path: `${out}/hot-financing-filled.png`, fullPage: true });
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForURL((u) => !u.toString().includes(`/assignment/${did}`) && u.toString().includes("/assignment/"));
  await ao.waitForTimeout(500);
  r.rows = psql(`SELECT string_agg(concat_ws('|', bank_name, tenor_months, margin_rate, outstanding_amount), ' / ' ORDER BY sort_order) FROM fos.other_financing WHERE case_id = '${c}';`);
  r.next = psql(`SELECT assignment_name || '|' || assignment_status FROM fos.assignments WHERE case_id = '${c}' ORDER BY assignment_id DESC LIMIT 1;`);

  console.log(JSON.stringify(r, null, 1));
  await browser.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
