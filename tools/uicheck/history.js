// Ticket 14: History Details under the Team Lead's Pre Screening: API Log
// History (UI-020, Gambar 36, 37: sort, filter, "Lihat semua") and
// Pre-Screening API Data (UI-021, Gambar 38-40: tabs, Reject banner, SLIK
// and Pefindo fields). A case made through the UI up to Create gets its log
// rows through psql. Needs what create.js and review.js need, and file
// storage as for duu.js.
// Usage: node history.js shots
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

const longRequest = JSON.stringify({ kode_reff_pengguna: "SKL253/LOS002/10/2026/1", tujuan_permintaan: "TP-1", nomor_identitas: "1111111121212121",
  pilihan_fasilitas: "PF-1", nomor_memo: "002/pebrianto/20261024010101", alamat_sesuai_identitas: "Jl. Sudirman No. 1, Kelurahan Karet, Kecamatan Setiabudi, Jakarta Selatan",
  tipe_pencarian: "TC-1", segmentasi: "SG-1", underlying: "[file Pre-Screening-LF-1-1111111121212121-20261024.zip, application/zip, 1234 bytes]" });

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

  const c = r.caseId;
  const row = (name, minutes, status, req, resp, type = "ConnectRest") =>
    `(gen_random_uuid(), '${c}', '${type}', '${name}', (now() AT TIME ZONE 'UTC') - interval '${minutes} minutes', '${status}', ${req === null ? "NULL" : `'${req.replace(/'/g, "''")}'`}, '${resp.replace(/'/g, "''")}')`;
  psql(`INSERT INTO fos.connect_api_log (log_id, case_id, api_type, api_name, submitted_at, api_status, request, response) VALUES
    ${row("Dukcapil", 30, "Rejected", '{"nik":"1111111121212121","tanggalLahir":"01-01-2000","namaLengkap":"John Doe"}', '{"NIK":"Sesuai (100)","NAMA_LGKP":"Sesuai (100)","TGL_LHR":"Tidak Sesuai"}')},
    ${row("DHN", 29, "Success", "1111111121212121", '{"code":"DN-00","message":"Data tidak ditemukan","data":{"blacklisted":false}}')},
    ${row("SLIK", 28, "Success", longRequest, '{"code":"1100","message":"success"}')},
    ${row("SLIKPBSResponse", 20, "Success", '{"Kode":"SKL253/LOS002/10/2026/1"}', '{"Code":"0"}', "Service Rest")},
    ${row("Pefindo", 10, "Success", '{"inquiry_reason":"12","product_id":"1","reference_code":"SMSME2600000001","type":"PERSONAL","params":[{"date_of_birth":"2000-01-01","id_no":"1111111121212121","id_type":"KTP","name":"John Doe"}]}',
      '{"code":"01","status":"Success","message":"Data ditemukan","data":[{"similarity_score":80,"id_pefindo":111,"nama_debitur":"JOHN D"},{"similarity_score":100,"id_pefindo":222,"nama_debitur":"JOHN DOE"}],"inquiry_id":1436899}')};`);

  const tl = await login(browser, "TL01");
  const id = psql(`SELECT max(assignment_id) FROM fos.assignments WHERE case_id = '${c}' AND assignment_name = 'Pre Screening';`);
  await tl.goto(`${base}/app/fossyariah/case/${c}/assignment/${id}`);
  await tl.mouse.move(900, 500);
  await tl.waitForSelector("text=API Log History");
  await tl.getByRole("button", { name: "API Log History" }).click();
  const names = async () => (await tl.locator("table tbody tr td:first-child").allInnerTexts()).join(",");
  r.initial = await names();
  await tl.getByRole("button", { name: "Request menu" }).click();
  await tl.screenshot({ path: `${out}/history-menu.png` });
  await tl.keyboard.press("Escape");
  await tl.getByRole("button", { name: "Request menu" }).click();
  await tl.getByRole("button", { name: "Waktu Connect menu" }).click();
  await tl.getByRole("menuitem", { name: /Sort ascending/ }).click();
  r.ascending = await names();
  await tl.getByRole("button", { name: "Nama API menu" }).click();
  await tl.getByRole("menuitem", { name: /Apply filter/ }).click();
  await tl.screenshot({ path: `${out}/history-filter.png` });
  await tl.getByRole("dialog", { name: "Apply filter" }).getByLabel("Dukcapil").check();
  await tl.getByRole("button", { name: "Apply" }).click();
  r.filtered = await names();
  await tl.getByRole("button", { name: "Nama API menu" }).click();
  await tl.getByRole("menuitem", { name: /Apply filter/ }).click();
  await tl.getByRole("button", { name: "Clear filter" }).click();
  r.cleared = await names();
  r.cut = await tl.locator("text=Lihat semua").count();
  await tl.locator("text=Lihat semua").first().click();
  r.expanded = await tl.locator("pre").first().innerText();
  await tl.screenshot({ path: `${out}/history-apilog.png`, fullPage: true });

  await tl.getByRole("button", { name: "API Log History" }).click();
  await tl.getByRole("button", { name: "Pre-Screening API Data" }).click();
  r.processes = (await tl.locator("[class*=processHeader]").allInnerTexts()).map((t) => t.replace("›", "").trim());
  await tl.getByRole("button", { name: "Dukcapil", exact: true }).click();
  r.banner = await tl.locator("text=Pengajuan ditolak karena tidak sesuai dengan ketentuan Pembiayaan.").count();
  await tl.screenshot({ path: `${out}/history-dukcapil.png`, fullPage: true });
  await tl.getByRole("button", { name: "Dukcapil", exact: true }).click();
  await tl.getByRole("button", { name: "SLIK", exact: true }).click();
  r.slikRequest = await tl.locator("[class*=processBody]").innerText();
  await tl.getByRole("button", { name: "SLIK", exact: true }).click();
  await tl.getByRole("button", { name: "Pefindo", exact: true }).click();
  await tl.getByRole("tab", { name: "RESPONSE" }).click();
  r.pefindoResponse = await tl.locator("[class*=processBody]").innerText();
  await tl.screenshot({ path: `${out}/history-pefindo.png`, fullPage: true });
  console.log(JSON.stringify(r, null, 2));
  await browser.close();
})();
