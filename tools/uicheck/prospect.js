// Ticket 17: Prospect Data Entry Perorangan, UI-026 to UI-028 (Gambar 51-56).
// A case with a Spouse made through the UI up to Create is moved to Pipeline
// Data Entry through psql (as a completed run would), with test master data;
// Pipeline Submit leads to Data Pribadi: errors, money and number fields,
// the sector autocomplete, the region cascade, "sama dengan e-KTP"; Next
// to the not-married Data Pasangan, Back, Kawin, Data Pasangan, Data
// Keluarga Dekat and Create. Needs what create.js and duu.js need.
// Usage: node prospect.js shots
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

// select chooses an option by its text in the field labelled label (the nth one).
const select = (page, label, text, nth = 0) => page.getByLabel(label, { exact: true }).nth(nth).selectOption({ label: text });

(async () => {
  fs.writeFileSync(`${out}/small.pdf`, "%PDF-1.4 small");
  psql(`INSERT INTO fos.business_mapping_value (code, type, sub_type, description, sort_order, value1, status, created_at, created_by, updated_at, updated_by) VALUES
    ${[
      bmv("SUBMISSION_TYPE", "New", "Pengajuan Baru", 1),
      bmv("JOB", "J1", "Karyawan Swasta", 1),
      bmv("GENDER", "G1", "Pria", 1), bmv("GENDER", "G2", "Wanita", 2),
      bmv("SOURCE_INCOME", "SI1", "Gaji", 1),
      bmv("RELATIONSHIP_WITH_BANK", "RB1", "Tidak Ada", 1),
      bmv("CUSTOMER_CATEGORY", "CC1", "Perorangan", 1),
      bmv("ECONOMIC_SECTOR", "4100", "Konstruksi Gedung", 1), bmv("ECONOMIC_SECTOR", "4700", "Perdagangan Eceran", 2),
      bmv("BUSINESS_TYPE", "BT1", "Swasta", 1),
      bmv("HOUSE_OWNERSHIP_STATUS", "HO1", "Milik Sendiri", 1),
      bmv("MAILING_ADDRESS", "MA-1", "e-KTP", 1), bmv("MAILING_ADDRESS", "MA-2", "Domisili", 2),
      bmv("LAST_EDUCATION", "ED1", "S1", 1),
      bmv("MARITAL_STATUS", "MS-1", "Belum Kawin", 1, "B"), bmv("MARITAL_STATUS", "MS-2", "Kawin", 2, "K"),
      bmv("DEPENDENTSLIST", "D0", "0", 1),
      bmv("PRENUP", "PN1", "Ya", 1), bmv("PRENUP", "PN2", "Tidak", 2),
      bmv("FAMILY_RELATIONSHIP", "FR1", "Saudara Kandung", 1),
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
  await ao.getByLabel("Data Pasangan NIK").fill("1111111111111191");
  await ao.getByLabel("Data Pasangan Nama").fill("Jane Doe");
  await ao.getByLabel("Data Pasangan Tanggal Lahir").fill("01/01/2000");
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

  // As a completed run would (FR-APP-004 Assignment 1-2), then Pipeline Submit.
  const pid = psql(`UPDATE fos.assignments SET assignment_status = 'sudah selesai dikerjakan', completed_at = now() WHERE case_id = '${c}' AND assignment_status <> 'sudah selesai dikerjakan';
    UPDATE fos.work SET case_status = 'Pending-Submission' WHERE case_id = '${c}';
    INSERT INTO fos.assignments (case_id, assignment_name, assignment_type, assigned_user_id, assignment_status, created_at)
    VALUES ('${c}', 'Pipeline Data Entry', 'worklist', 'pebrianto', 'baru', now() AT TIME ZONE 'UTC') RETURNING assignment_id;`);
  await ao.goto(`${base}/app/fossyariah/case/${c}/assignment/${pid}`);
  await ao.waitForSelector("text=Inisiasi");
  await ao.getByRole("combobox", { name: "Jenis Pengajuan" }).click();
  await ao.getByRole("option", { name: "Pengajuan Baru", exact: true }).click();
  await ao.getByLabel("Produk").selectOption({ label: "testing_mikro_aira_qardh" });
  await ao.getByRole("button", { name: "Submit" }).click();
  await ao.waitForURL("**/app/fossyariah");
  const did = psql(`SELECT assignment_id FROM fos.assignments WHERE case_id = '${c}' AND assignment_name = 'Data Pribadi';`);

  // UI-026 Data Pribadi.
  await ao.goto(`${base}/app/fossyariah/case/${c}/assignment/${did}`);
  await ao.mouse.move(1400, 1000);
  await ao.waitForSelector("text=Nasabah Baru");
  await ao.screenshot({ path: `${out}/prospect-personal.png`, fullPage: true });
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForSelector("[class*=errorBanner] li");
  r.emptyErrors = (await ao.locator("[class*=errorBanner] li").allInnerTexts()).length;
  r.firstErrors = (await ao.locator("[class*=errorBanner] li").allInnerTexts()).slice(0, 3);
  r.ektpErrorSample = (await ao.locator("[class*=errorBanner] li", { hasText: "e-KTP - " }).allInnerTexts()).slice(0, 2);
  await ao.screenshot({ path: `${out}/prospect-personal-errors.png` });

  await ao.getByText("Seumur Hidup").click();
  await ao.getByLabel("Kewarganegaraan", { exact: true }).fill("Indonesia");
  await ao.getByLabel("Nomor NPWP", { exact: true }).fill("123456789012345");
  await select(ao, "Pekerjaan", "Karyawan Swasta");
  await ao.getByLabel("Tempat Bekerja", { exact: true }).fill("PT Maju");
  await ao.getByRole("radio", { name: "Pria" }).check();
  await ao.getByLabel("Alamat Tempat Kerja", { exact: true }).fill("Jl. Kerja 2");
  await ao.getByLabel("Penghasilan Kotor Pertahun", { exact: true }).fill("15000000-.");
  r.income = await ao.getByLabel("Penghasilan Kotor Pertahun", { exact: true }).inputValue();
  await select(ao, "Sumber Penghasilan", "Gaji");
  await select(ao, "Hubungan dengan Bank", "Tidak Ada");
  await select(ao, "Golongan Nasabah", "Perorangan");
  await ao.getByLabel("Bidang Usaha Tempat Bekerja", { exact: true }).fill("kon");
  await ao.waitForSelector("[role=listbox] [role=option]");
  r.sectorItems = await ao.locator("[role=listbox] [role=option]").allInnerTexts();
  await ao.screenshot({ path: `${out}/prospect-sector.png` });
  await ao.keyboard.press("Enter");
  r.sectorShown = await ao.getByLabel("Bidang Usaha Tempat Bekerja", { exact: true }).inputValue();
  await ao.getByLabel("Tempat Lahir", { exact: true }).fill("Jakarta");
  await select(ao, "Tipe Bisnis Tempat Bekerja", "Swasta");
  await ao.getByLabel("Alamat Rumah Tinggal (Sesuai e-KTP)", { exact: true }).fill("Jl. Tebet Raya 1");
  await ao.getByLabel("Nomor Handphone", { exact: true }).first().fill("0812-3456-7890");
  await ao.getByLabel("Nomor Telepon", { exact: true }).first().fill("021-123");
  await ao.getByLabel("RT", { exact: true }).first().fill("007");
  await ao.getByLabel("RW", { exact: true }).first().fill("0123");
  r.rwTyped = await ao.getByLabel("RW", { exact: true }).first().inputValue();
  await ao.getByLabel("Lama Tinggal (Tahun)", { exact: true }).first().fill("5");
  await ao.getByLabel("Lama Tinggal (Bulan)", { exact: true }).first().fill("11");
  await select(ao, "Status Kepemilikan Rumah", "Milik Sendiri");
  r.cityDisabledBefore = await ao.getByLabel("Kota", { exact: true }).first().isDisabled();
  await select(ao, "Provinsi", "DKI Jakarta");
  await select(ao, "Kota", "Jakarta Selatan");
  await select(ao, "Kecamatan", "Tebet");
  await select(ao, "Kelurahan", "Manggarai");
  await ao.waitForFunction(() => document.body.innerText.includes("12850"));
  r.datiII = await ao.getByLabel("Dati II", { exact: true }).first().innerText();
  r.kodePos = await ao.getByLabel("Kode Pos", { exact: true }).first().innerText();
  await ao.screenshot({ path: `${out}/prospect-ektp.png`, fullPage: true });
  // Checked: Domisili follows e-KTP and hides; unchecked again: a copy to edit.
  await ao.getByText("Alamat Rumah Tinggal sama dengan e-KTP").click();
  r.domicileHidden = (await ao.getByLabel("Alamat Rumah Tinggal Sekarang (Jika berbeda dengan e-KTP)", { exact: true }).count()) === 0;
  await ao.getByText("Alamat Rumah Tinggal sama dengan e-KTP").click();
  r.domicileCopy = await ao.getByLabel("Alamat Rumah Tinggal Sekarang (Jika berbeda dengan e-KTP)", { exact: true }).inputValue();
  await ao.getByText("Alamat Rumah Tinggal sama dengan e-KTP").click();
  await select(ao, "Pendidikan Terakhir", "S1");
  await select(ao, "Status Perkawinan", "Belum Kawin");
  await ao.getByLabel("Nama Gadis Ibu Kandung", { exact: true }).fill("Siti");
  await select(ao, "Jumlah Tanggungan", "0");
  await ao.getByRole("button", { name: "Next" }).click();

  // UI-027 not married (Gambar 55), Back, then Kawin.
  await ao.waitForSelector("text=Status Nasabah Tidak Kawin");
  await ao.mouse.move(1400, 1000);
  await ao.screenshot({ path: `${out}/prospect-not-married.png` });
  await ao.getByRole("button", { name: "Back" }).click();
  await ao.waitForSelector("text=Nasabah Baru");
  r.refilledPlace = await ao.getByLabel("Tempat Lahir", { exact: true }).inputValue();
  r.refilledIncome = await ao.getByLabel("Penghasilan Kotor Pertahun", { exact: true }).inputValue();
  await select(ao, "Status Perkawinan", "Kawin");
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForSelector("text=Nama Pasangan (Sesuai e-KTP)");
  await ao.mouse.move(1400, 1000);
  r.spouseName = await ao.getByLabel("Nama Pasangan (Sesuai e-KTP)", { exact: true }).inputValue();
  await ao.screenshot({ path: `${out}/prospect-spouse.png`, fullPage: true });
  await ao.getByRole("button", { name: "Next" }).click();
  await ao.waitForSelector("[class*=errorBanner] li");
  r.spouseErrors = await ao.locator("[class*=errorBanner] li").allInnerTexts();
  await ao.getByLabel("Tempat Lahir", { exact: true }).fill("Bogor");
  await select(ao, "Pekerjaan", "Karyawan Swasta");
  await ao.getByLabel("Nomor Handphone", { exact: true }).fill("0813");
  await ao.getByRole("radio", { name: "Tidak" }).check();
  await ao.getByRole("button", { name: "Next" }).click();

  // UI-028 Data Keluarga Dekat, then Create.
  await ao.waitForSelector("text=Data Keluarga Dekat (Yang Tidak Tinggal Serumah)");
  await ao.mouse.move(1400, 1000);
  await ao.screenshot({ path: `${out}/prospect-family.png`, fullPage: true });
  await ao.getByLabel("Nama Lengkap (Sesuai e-KTP)", { exact: true }).fill("Andi");
  await select(ao, "Hubungan Keluarga", "Saudara Kandung");
  await ao.getByLabel("Alamat Tempat Tinggal Saat Ini", { exact: true }).fill("Jl. Saudara 4");
  await select(ao, "Provinsi", "DKI Jakarta");
  await select(ao, "Kota", "Jakarta Selatan");
  await select(ao, "Kecamatan", "Tebet");
  await select(ao, "Kelurahan", "Manggarai");
  await ao.getByLabel("No. HP", { exact: true }).fill("0814");
  await select(ao, "Pendidikan Terakhir", "S1");
  await ao.locator("footer button", { hasText: "Create" }).click();
  await ao.waitForURL("**/app/fossyariah");
  r.stage = psql(`SELECT current_stage || '|' || case_status FROM fos.work WHERE case_id = '${c}';`);
  r.next = psql(`SELECT assignment_name || '|' || assignment_status FROM fos.assignments WHERE case_id = '${c}' ORDER BY assignment_id DESC LIMIT 1;`);
  r.spouseRow = psql(`SELECT full_name || '|' || prenup_code FROM fos.customer WHERE case_id = '${c}' AND customer_type = 'Pasangan';`);

  console.log(JSON.stringify(r, null, 1));
  await browser.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
