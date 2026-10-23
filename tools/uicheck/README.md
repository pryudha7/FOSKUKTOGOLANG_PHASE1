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

No UI check for ticket 13 either: its screens (UI-021 Pefindo) are ticket 14.

No UI check for ticket 11: it has no screen (the PBSResponse service is called by the RPA).

`duu.js` needs file storage: run `node fakegcs.js 14443 shots/gcs.json` (an in-memory stand-in for the GCS JSON API) and start the backend with `GCS_PROJECT_ID` and `GCS_BUCKET_NAME` set to any value, `GCS_CREDENTIAL` set to the written `shots/gcs.json`, and `STORAGE_EMULATOR_HOST=127.0.0.1:14443`. Its ERR-035 step edits the case through `docker exec fos-smoke-pg psql`; set `DOCKER` to docker's full path if `cmd` cannot find it.
