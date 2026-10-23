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
