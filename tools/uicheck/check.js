const { chromium } = require("playwright-core");
const out = process.argv[2];
(async () => {
  const browser = await chromium.launch({ executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe" });
  const run = async (user, tag) => {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 720 }, extraHTTPHeaders: { "X-Forwarded-Proto": "https" } });
    const page = await ctx.newPage();
    await page.goto("http://localhost:13000/PRWebDB1");
    await page.fill('input[name="username"]', user);
    await page.fill('input[name="password"]', "Rahasia#1");
    await page.click('button[type="submit"]');
    await page.waitForURL("**/app/fossyariah");
    await page.waitForSelector("nav");
    await page.mouse.move(700, 400);
    await page.screenshot({ path: `${out}/${tag}-1-collapsed.png` });
    const labelsCollapsed = await page.locator("nav .menu-label, nav button span").allInnerTexts();
    await page.hover("nav");
    await page.waitForTimeout(300);
    await page.screenshot({ path: `${out}/${tag}-2-expanded.png` });
    const navText = await page.locator("nav").innerText();
    const create = page.getByRole("button", { name: "Create" });
    const hasCreate = (await create.count()) > 0;
    let submenu = [];
    if (hasCreate) {
      await create.click();
      await page.waitForTimeout(200);
      submenu = await page.locator("nav ul ul button").allInnerTexts();
      await page.hover("text=Gold Monitoring");
      await page.waitForTimeout(150);
      await page.screenshot({ path: `${out}/${tag}-3-create-open-hover.png` });
    }
    await page.locator("nav [aria-haspopup=menu]").click();
    await page.waitForTimeout(200);
    await page.screenshot({ path: `${out}/${tag}-4-avatar-popup.png` });
    const popup = await page.locator("[role=menu]").innerText();
    await page.getByRole("menuitem", { name: "Log off" }).click();
    await page.waitForURL("**/PRWebDB1");
    const afterLogout = page.url();
    await page.goto("http://localhost:13000/app/fossyariah");
    await page.waitForURL("**/PRWebDB1");
    console.log(JSON.stringify({ tag, hasCreate, submenu, navText: navText.split("\n").filter(Boolean), popup, afterLogout, protectedAfterLogout: page.url() }));
    await ctx.close();
  };
  await run("pebrianto", "ao");
  await run("tl01", "tl");
  await browser.close();
})().catch((e) => { console.error(e); process.exit(1); });
