# UI checks (headless Chrome)

Scripts that drive the running app in a local Chrome via `playwright-core` and save screenshots, used to compare screens with `docs/ui-reference/`. They are manual checks, not part of the test suite.

```sh
cd tools/uicheck && npm install          # playwright-core only; uses the installed Chrome
node ps.js shots                         # writes screenshots into ./shots
```

Before running: start PostgreSQL, the backend and `next start -p 13000` (see the root README; start servers from PowerShell on Windows), and seed BMV branch `002` and an AO operator `pebrianto` / `Rahasia#1` (`go run ./cmd/hashpw` gives the hash). The scripts expect Chrome at `C:/Program Files/Google/Chrome/Application/chrome.exe`.

| Script | Checks |
|---|---|
| `check.js` | Sidebar collapsed/expanded, Create menu (AO vs TL), avatar Log off (ticket 04) |
| `hover.js` | Menu hover colours and Create reset (ticket 04) |
| `idle.js` | UI-019 idle popup with a virtual clock (ticket 03) |
| `case.js` | Create → Financing, Case View, lifecycle popup, info panel hide/show, attachments (ticket 05) |
| `ps.js`, `check2.js` | Pre-Screening Submission validation, banner, date field/picker, Next and Back (ticket 30) |
| `create.js` | SLIK Request Data, Create errors (ERR-010, ERR-020, ERR-026, accordion marker) and Create handing the case to the Team Lead (ticket 33); also needs SLIK_* BMV rows TP-1, PF-1, TC-1, SG-1 |
| `review.js` | Team Lead Pre Screening: form, errors, Tidak Sesuai, Notes banner on the returned Pre-Screening Submission, History Pre-screening Notes (ticket 08); also needs operator TL01 |
| `retry.js` | Sesuai starts the Pre-Screening Check; Dukcapil on a closed port fails four times; the Team Lead's Retry view (Gambar 35) and its Submit (ticket 09); needs `api/dukcapil/baseurl` set to a closed port and `DUKCAPIL_APIKEY` in the backend environment |
| `duu.js` | Document Underlying Upload: document list, Upload Document modal, document icon download, attachments panel, ERR-035 popup (ticket 07) |
| `slik.js` | Manual SLIK Entry after a SLIK timeout (set up through psql): ERR-036 banner on Create until the SLIK document is uploaded, then the Team Lead's Pre Screening with API Log History above History Pre-screening Notes and the decision (ticket 12); needs what `create.js`, `review.js` and `duu.js` need |
| `waitslik.js` | UI-023 Waiting SLIK (Gambar 44): Due, Check SLIK Status against a fake SFS on port 14555, one Message panel, two with a Spouse, no Submit (ticket 36); needs what `create.js` and `duu.js` need, `SLIK_APIKEY` in the backend environment and `api/slik/baseurl` = `http://127.0.0.1:14555` |
| `history.js` | History Details under the Team Lead's Pre Screening (log rows set up through psql): API Log History sort, filter popup, Clear filter, "Lihat semua" (UI-020, Gambar 36, 37) and Pre-Screening API Data tabs, Reject banner, SLIK descriptions and Pefindo results (UI-021, Gambar 38-40) (ticket 14); needs what `create.js`, `review.js` and `duu.js` need |
| `pipeline.js` | UI-024 Pipeline Data Entry (Gambar 45-47): case moved to Pipeline Data Entry through psql with test Jenis Pengajuan, products and banks; ERR-010 banner cleared by a change, Take Over with the Nama Bank autocomplete, Bukti Surat upload and its deletion when Take Over is picked again, Submit back to the Dashboard with Data Pribadi new, one column below 768 px (ticket 15); needs what `create.js` and `duu.js` need |
| `prospect.js` | UI-026 to UI-028 Prospect Data Entry Perorangan (Gambar 51-56): case with a Spouse moved to Pipeline Data Entry through psql with test master data (regions included); Data Pribadi errors, money field, sector autocomplete, region cascade with Dati II and Kode Pos, "sama dengan e-KTP"; the not-married Data Pasangan, Back, Kawin, Data Keluarga Dekat and Create to Financing Application (ticket 17); needs what `create.js` and `duu.js` need |
| `business.js` | UI-025 Prospect Data Entry Badan Usaha (Gambar 48-50): case made a Badan Usaha with one Pengurus through psql (the Pre-Screening form is ticket 31); layout, errors with "Pengurus <n> - ", two-line autocompletes, region cascade, Tambah and delete, Next to Financing Application; then a failed dropdown faked at the network boundary (Playwright route): ERR-038 item and ERR-039 popup (ticket 16); needs what `create.js` and `duu.js` need |
| `hotprospect.js` | UI-030 Data Usaha Nasabah and UI-031 Data Pembiayaan Lain (Gambar 71-73): case moved to Financing Application with a received customer SLIK of three facilities through psql; errors, Save, Next, the margin field, Isi Data dari SLIK (accordions, ticks, Submit), Tambah, delete, Next to Data Permohonan Fasilitas (ticket 18); needs what `create.js` and `duu.js` need |
| `facility.js` | UI-032 Data Permohonan Fasilitas (Gambar 74-77): case moved to an open Data Permohonan Fasilitas with product 794 through psql; empty state, values following the typed ones, read-only approval fields, the five-decimal margin, Effective and Flat, the Credit Simulation modal (Home, X), ERR-010, ERR-042, Next to QCA Data with the recommendation (ticket 19); needs what `create.js` and `duu.js` need |
| `qca.js` | UI-033 QCA Data (Gambar 79): case moved to an open QCA Data with its Data Permohonan Fasilitas recommendation and test QCA1-QCA10 options through psql; empty page, option order, ERR-010 on question 3, Save, Next to Collateral Data, reopened with an answer now Inactive (ticket 20); needs what `create.js` and `duu.js` need |

No UI check for ticket 13 either: its screens (UI-021 Pefindo) are ticket 14.

No UI check for ticket 11: it has no screen (the PBSResponse service is called by the RPA).

`duu.js` needs file storage: run `node fakegcs.js 14443 shots/gcs.json` (an in-memory stand-in for the GCS JSON API) and start the backend with `GCS_PROJECT_ID` and `GCS_BUCKET_NAME` set to any value, `GCS_CREDENTIAL` set to the written `shots/gcs.json`, and `STORAGE_EMULATOR_HOST=127.0.0.1:14443`. Its ERR-035 step edits the case through `docker exec fos-smoke-pg psql`; set `DOCKER` to docker's full path if `cmd` cannot find it.
