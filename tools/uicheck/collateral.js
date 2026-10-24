// Ticket 21: UI-034 Collateral Data (Gambar 84-90). A case made through the
// UI up to Create is moved through psql to an open Collateral Data with a
// Total Requested Amount of 150.000.000, its Data Permohonan Fasilitas
// recommendation and test options; then the empty page, ERR-044, each
// collateral type with its fields, the Tipe Bukti Kepemilikan number
// fields, the paripasu rule, Tahun Pembuatan, delete, the 10-card limit and
// Next to Document Upload. Needs what create.js and duu.js need.
// Usage: node collateral.js shots
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
      ('GD', 'BENTUKTANAHBANGUNAN', '', 'Gedung', 1, 'Active', now(), 'x', now(), 'x'),
      ('TS', 'COLLATERAL_STATUS', '', 'Tersedia', 1, 'Active', now(), 'x', now(), 'x'),
      ('JY', 'COLLATERAL_JOINT_ACCOUNT', '', 'Ya', 1, 'Active', now(), 'x', now(), 'x'),
      ('AY', 'COLLATERAL_ASURANSI', '', 'Ya', 1, 'Active', now(), 'x', now(), 'x')
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
  await ao.getByLabel("Data Pribadi NIK").fill("1111111111111193");
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

  // As Next on QCA Data would, the request over 100.000.000.
  const cid = psql(`UPDATE fos.assignments SET assignment_status = 'sudah selesai dikerjakan', completed_at = now() WHERE case_id = '${c}' AND assignment_status <> 'sudah selesai dikerjakan';
    INSERT INTO fos.case_stage_progress (case_id, stage_name, first_entered_at) VALUES ('${c}', 'Financing Application', now() AT TIME ZONE 'UTC') ON CONFLICT DO NOTHING;
    UPDATE fos.work SET current_stage = 'Financing Application', case_status = 'Pending-Submission' WHERE case_id = '${c}';
    INSERT INTO fos.case_financing (case_id, total_requested_amount, existing_financing_amount, total_obligor) VALUES ('${c}', 150000000, 0, 150000000);
    INSERT INTO fos.case_pre_screening_decision (case_id, recommendation_type, notes, decided_by, decided_by_role_code, decided_at)
    VALUES ('${c}', 'Data Permohonan Fasilitas', 'Layak', 'pebrianto', 'AO', now() AT TIME ZONE 'UTC');
    INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
    VALUES ('${c}', 'Collateral Data', 'worklist', 'pebrianto', 'baru', now() AT TIME ZONE 'UTC') RETURNING assignment_id;`);
  const card = (title, n) => ao.getByRole("group", { name: `${title} ${n}` });
  const main1 = card("Agunan Utama", 1);
  const support = (n) => card("Jaminan Pendukung", n);
  const field = (scope, label) => scope.getByLabel(label, { exact: true });

  // AC-FR-APP-021-01, -02 (Gambar 84).
  await ao.goto(`${base}/app/fossyariah/case/${c}/assignment/${cid}`);
  await ao.mouse.move(1400, 1000);
  await ao.waitForSelector("text=Kolateral Data");
  r.cards = [await ao.getByRole("group", { name: /Agunan Utama/ }).count(), await ao.getByRole("group", { name: /Jaminan Pendukung/ }).count()];
  r.types = [await field(main1, "Agunan Utama").locator("option").allInnerTexts(), await field(support(1), "Jaminan Pendukung").locator("option").allInnerTexts()];
  await ao.screenshot({ path: `${out}/collateral-empty.png`, fullPage: true });

  // ERR-044.
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForSelector("[class*=errorBanner] li");
  r.err044 = await ao.locator("[class*=errorBanner] li").allInnerTexts();

  // Gambar 85, 86; AC-04.
  await field(main1, "Agunan Utama").selectOption({ label: "Tanah dan Bangunan" });
  r.percentDisabled = await field(main1, "Presentase Paripasu").isDisabled();
  await ao.screenshot({ path: `${out}/collateral-land.png`, fullPage: true });
  await field(main1, "Nama Pemilik").fill("Budi");
  await field(main1, "Tipe Bukti Kepemilikan").selectOption({ label: "SHM" });
  await field(main1, "Nomor SHM").fill("SHM-1");
  r.shm = await field(main1, "Nomor SHM").count();
  await field(main1, "Tipe Bukti Kepemilikan").selectOption({ label: "SHGB" });
  r.afterSHGB = [await field(main1, "Nomor SHM").count(), await field(main1, "Nomor SHGB").count(), await field(main1, "Berlaku Sampai").count()];
  await ao.screenshot({ path: `${out}/collateral-land-shgb.png`, fullPage: true });

  // AC-03 (Gambar 87).
  await field(main1, "Agunan Utama").selectOption({ label: "Deposito Bank Nano Syariah" });
  r.deposit = [await field(main1, "Nama Pemilik").count(), await field(main1, "Jumlah Deposito Bank Nano Syariah").count()];
  await ao.screenshot({ path: `${out}/collateral-deposit.png`, fullPage: true });

  // AC-07, AC-09 on a Tanah dan Bangunan again.
  await field(main1, "Agunan Utama").selectOption({ label: "Tanah dan Bangunan" });
  r.reset = await field(main1, "Nama Pemilik").inputValue();
  await field(main1, "Tanah dan Bangunan Yang Berbentuk").selectOption({ label: "Gedung" });
  await field(main1, "Status Paripasu").selectOption({ label: "Ya" });
  await field(main1, "Presentase Paripasu").fill("25");
  r.percentShown = await field(main1, "Presentase Paripasu").inputValue();
  await field(main1, "Status Paripasu").selectOption({ label: "Tidak" });
  r.afterTidak = [await field(main1, "Presentase Paripasu").isDisabled(), await field(main1, "Presentase Paripasu").inputValue()];
  await field(main1, "Status Paripasu").selectOption({ label: "Ya" });
  await field(main1, "Presentase Paripasu").fill("25");

  // Gambar 88, 89, 90.
  await field(support(1), "Jaminan Pendukung").selectOption({ label: "Tanah Kosong Produktif" });
  await field(support(1), "Tipe Bukti Kepemilikan").selectOption({ label: "SHGB" });
  await field(support(1), "Nomor SHGB").fill("SHGB-9");
  await ao.screenshot({ path: `${out}/collateral-vacant.png`, fullPage: true });
  await ao.getByRole("button", { name: "Tambah Jaminan Pendukung" }).click();
  await field(support(2), "Jaminan Pendukung").selectOption({ label: "Kendaraan" });
  await field(support(2), "Tahun Pembuatan").pressSequentially("20a24");
  r.year = await field(support(2), "Tahun Pembuatan").inputValue();
  await ao.screenshot({ path: `${out}/collateral-vehicle.png`, fullPage: true });

  // AC-16 and the 10-card limit (AC-20).
  await ao.getByRole("button", { name: "Hapus Jaminan Pendukung 1" }).click();
  r.afterDelete = [await ao.getByRole("group", { name: /Jaminan Pendukung/ }).count(), await field(support(1), "Jaminan Pendukung").inputValue()];
  for (let i = 0; i < 9; i++) await ao.getByRole("button", { name: "Tambah Agunan Utama" }).click();
  r.addDisabledAt10 = await ao.getByRole("button", { name: "Tambah Agunan Utama" }).isDisabled();

  // Next: empty cards dropped, Document Upload opened.
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForURL((u) => !u.toString().includes(`/assignment/${cid}`) && u.toString().includes("/assignment/"));
  await ao.waitForTimeout(500);
  r.rows = psql(`SELECT string_agg(concat_ws('|', collateral_section, sequence_number, collateral_type_code, COALESCE(owner_name, '-'), COALESCE(pari_passu_percent::text, '-'), COALESCE(vehicle_year::text, '-')), ' / ' ORDER BY collateral_section, sequence_number) FROM fos.case_collateral WHERE case_id = '${c}';`);
  r.next = psql(`SELECT assignment_name || '|' || assignment_status FROM fos.assignments WHERE case_id = '${c}' ORDER BY assignment_id DESC LIMIT 1;`);

  console.log(JSON.stringify(r, null, 1));
  await browser.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
