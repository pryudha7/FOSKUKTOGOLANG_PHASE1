// Ticket 16: UI-025 Prospect Data Entry, Badan Usaha (Gambar 48-50). The
// Pre-Screening Badan Usaha form is ticket 31, so a case made through the UI
// up to Create becomes a Badan Usaha with one Pengurus through psql, then
// goes to Pipeline Data Entry (as a completed run would); Pipeline Submit
// leads to Badan Usaha: layout, errors with "Pengurus <n> - ", the two-line
// autocompletes, the region cascade, Tambah and delete, Next to the
// Dashboard. Last, the page answer is faked at the network boundary with a
// failed dropdown: ERR-038 in it and the ERR-039 popup. Needs what create.js
// and duu.js need.
// Usage: node business.js shots
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
const banner = (page) => page.locator("[class*=errorBanner] li").allInnerTexts();

(async () => {
  fs.writeFileSync(`${out}/small.pdf`, "%PDF-1.4 small");
  psql(`INSERT INTO fos.business_mapping_value (code, type, sub_type, description, sort_order, value1, status, created_at, created_by, updated_at, updated_by) VALUES
    ${[
      bmv("SUBMISSION_TYPE", "New", "Pengajuan Baru", 1),
      bmv("BUSINESS_ENTITY", "PT", "Perseroan Terbatas", 1),
      bmv("RELATIONSHIP_WITH_BANK", "RB1", "Tidak Ada", 1),
      bmv("BUSINESS_TYPE", "BT1", "Swasta", 1),
      bmv("GO_PUBLIC_STATUS", "GP1", "Tidak Go Public", 1), bmv("GO_PUBLIC_STATUS", "GP2", "Go Public", 2),
      bmv("BUSINESSENTITY_CATEGORY", "0101", "Perusahaan Swasta Nasional", 1), bmv("BUSINESSENTITY_CATEGORY", "0202", "BUMN", 2),
      bmv("BUSINESS_ROLE", "R1", "Pemilik Perusahaan", 1), bmv("BUSINESS_ROLE", "R2", "Direktur", 2),
      bmv("ECONOMIC_SECTOR", "011303", "PERTANIAN BIT GULA DAN TANAMAN PEMANIS BUKAN TEBU", 1),
      bmv("ECONOMIC_SECTOR", "011309", "PERTANIAN SAYURAN, BUAH DAN ANEKA UMBI LAINNYA", 2),
      bmv("ECONOMIC_SECTOR", "011400", "PERTANIAN TANAMAN TEBU", 3),
      bmv("PROVINCE", "31", "DKI Jakarta", 1),
      bmv("CITY", "3171", "Jakarta Selatan", 1, "JAKARTA SELATAN", "31"),
      bmv("DISTRICT", "317101", "Tebet", 1, null, "3171"),
      bmv("SUBDISTRICT", "3171011", "Manggarai", 1, null, "317101"),
      bmv("POSTALCODE", "12850", "12850", 1, null, "3171011"),
    ].join(",\n")}
    ON CONFLICT DO NOTHING;
    INSERT INTO fos.product_management (code, name, contract_type, created_at, created_by, updated_at, updated_by)
    VALUES ('794', 'testing_mikro_aira_qardh', 'Qardh', now(), 'x', now(), 'x') ON CONFLICT DO NOTHING;`);
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

  // The case as a Badan Usaha with its Pre-Screening Pengurus, then Pipeline.
  const pid = psql(`UPDATE fos.assignments SET assignment_status = 'sudah selesai dikerjakan', completed_at = now() WHERE case_id = '${c}' AND assignment_status <> 'sudah selesai dikerjakan';
    UPDATE fos.work SET customer_type = 'Badan Usaha', case_status = 'Pending-Submission' WHERE case_id = '${c}';
    INSERT INTO fos.customer (case_id, customer_type, npwp, full_name, establishment_date) VALUES ('${c}', 'Badan Usaha', '1111111111111110', 'PT John Doe', '2000-01-01');
    INSERT INTO fos.customer (case_id, customer_type, sort_order, role_code, id_number, full_name, birth_date) VALUES ('${c}', 'Pengurus', 1, 'R1', '1111111111111110', 'PT John Doe', '1975-01-01');
    INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
    VALUES ('${c}', 'Pipeline Data Entry', 'worklist', 'pebrianto', 'baru', now() AT TIME ZONE 'UTC') RETURNING assignment_id;`);
  await ao.goto(`${base}/app/fossyariah/case/${c}/assignment/${pid}`);
  await ao.waitForSelector("text=Inisiasi");
  await ao.getByRole("combobox", { name: "Jenis Pengajuan" }).click();
  await ao.getByRole("option", { name: "Pengajuan Baru", exact: true }).click();
  await ao.getByLabel("Produk").selectOption({ label: "testing_mikro_aira_qardh" });
  await ao.getByRole("button", { name: "Submit" }).click();
  await ao.waitForURL("**/app/fossyariah");
  const bid = psql(`SELECT assignment_id FROM fos.assignments WHERE case_id = '${c}' AND assignment_name = 'Badan Usaha';`);
  const pageURL = `${base}/app/fossyariah/case/${c}/assignment/${bid}`;

  // UI-025.
  await ao.goto(pageURL);
  await ao.mouse.move(1400, 1000);
  await ao.waitForSelector("text=Nasabah Baru");
  r.prefilled = [await ao.getByLabel("Nomor NPWP", { exact: true }).inputValue(), await ao.getByLabel("Nama Badan Usaha", { exact: true }).inputValue(),
    await ao.getByLabel("Nama", { exact: true }).inputValue()];
  r.cityDisabled = await ao.getByLabel("Kota", { exact: true }).isDisabled();
  await ao.screenshot({ path: `${out}/business.png`, fullPage: true });

  await ao.getByRole("button", { name: "Hapus Pengurus 1" }).click();
  r.cardsAfterDeletingTheLast = await ao.getByRole("group", { name: /Pengurus/ }).count();
  await ao.getByLabel("Nomor Telepon", { exact: true }).fill("+62-21-555 1234");
  await ao.getByLabel("Alamat Email", { exact: true }).fill("a@b");
  await ao.getByRole("button", { name: "Tambah" }).click();
  await ao.getByLabel("NIK", { exact: true }).nth(1).fill("1111111111111110");
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForSelector("[class*=errorBanner] li");
  r.errors = await banner(ao);
  await ao.screenshot({ path: `${out}/business-errors.png`, fullPage: true });

  await ao.getByRole("button", { name: "Hapus Pengurus 2" }).click();
  r.cardErrorsAfterDelete = await ao.locator("text=NIK tidak boleh sama").count();
  await ao.getByLabel("Tempat Pendirian", { exact: true }).fill("Jakarta");
  await ao.getByLabel("Nomor Akta Pendirian", { exact: true }).fill("AKTA-1");
  await ao.getByLabel("Nomor Telepon", { exact: true }).fill("021-5551234");
  await ao.getByLabel("Alamat Email", { exact: true }).fill("info@contoh.co.id");
  await ao.getByLabel("Kode Bidang Usaha", { exact: true }).fill("pertanian");
  await ao.waitForSelector("[role=listbox] [role=option]");
  r.sectorItems = await ao.locator("[role=listbox] [role=option]").allInnerTexts();
  await ao.screenshot({ path: `${out}/business-sector.png` });
  await ao.keyboard.press("Enter");
  r.sectorShown = await ao.getByLabel("Kode Bidang Usaha", { exact: true }).inputValue();
  await select(ao, "Tipe Bisnis", "Swasta");
  await ao.getByLabel("Golongan Nasabah", { exact: true }).fill("bum");
  await ao.waitForSelector("[role=listbox] [role=option]");
  await ao.keyboard.press("Enter");
  await ao.getByLabel("Alamat", { exact: true }).fill("Jl. Usaha 5");
  await select(ao, "Go Public", "Tidak Go Public");
  await select(ao, "Provinsi", "DKI Jakarta");
  await select(ao, "Kota", "Jakarta Selatan");
  await select(ao, "Kecamatan", "Tebet");
  await select(ao, "Kelurahan", "Manggarai");
  await ao.waitForFunction(() => document.body.innerText.includes("12850"));
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForURL("**/app/fossyariah");
  r.saved = psql(`SELECT concat_ws('|', place_of_establishment, economic_sector_code, customer_category_code, go_public) FROM fos.customer WHERE case_id = '${c}' AND customer_type = 'Badan Usaha';`);
  r.officers = psql(`SELECT count(*) FROM fos.customer WHERE case_id = '${c}' AND customer_type = 'Pengurus';`);
  r.stage = psql(`SELECT current_stage || '|' || case_status FROM fos.work WHERE case_id = '${c}';`);
  r.next = psql(`SELECT assignment_name || '|' || assignment_status FROM fos.assignments WHERE case_id = '${c}' ORDER BY assignment_id DESC LIMIT 1;`);

  // AC-FR-APP-005-07: one initial dropdown failed (faked at the network boundary).
  const bid2 = psql(`INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
    VALUES ('${c}', 'Badan Usaha', 'worklist', 'pebrianto', 'baru', now() AT TIME ZONE 'UTC') RETURNING assignment_id;`);
  await ao.route(`**/api/cases/${c}/assignments/${bid2}`, async (route) => {
    const res = await route.fetch();
    const body = await res.json();
    body.page.failedOptions = ["businessRole"];
    body.page.options.businessRole = [];
    await route.fulfill({ response: res, json: body });
  });
  await ao.goto(`${base}/app/fossyariah/case/${c}/assignment/${bid2}`);
  await ao.waitForSelector("[role=alertdialog]");
  r.popup = await ao.locator("[role=alertdialog]").innerText();
  await ao.screenshot({ path: `${out}/business-err039.png` });
  await ao.getByRole("button", { name: "OK" }).click();
  r.roleItems = await ao.getByLabel("Jabatan", { exact: true }).locator("option").allInnerTexts();

  console.log(JSON.stringify(r, null, 1));
  await browser.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
