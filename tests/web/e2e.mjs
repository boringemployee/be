// Browser tests for the web build (run with: make e2e).
//
// 1. The WebAssembly build reproduces every content golden file byte for byte, which
//    proves the CLI and the web share identical behavior.
// 2. An interactive session works: run, Esc, Ctrl+C and restart.
// 3. Share URL parameters are allowlisted.
// 4. The setup dialog appears on a first visit and writes the choice to the URL.
// 5. The page runs under its production CSP with no violations or errors.
//
// Set SCREENSHOT_DIR to also save screenshots.
import { readFile, readdir, mkdir } from "node:fs/promises";
import { join } from "node:path";
import { createRequire } from "node:module";
import { serve } from "./serve.mjs";

// require() finds a globally installed Playwright, which ESM import does not.
const { chromium } = createRequire(import.meta.url)("playwright");

const GOLDEN = "tests/golden/testdata/content";
const shots = process.env.SCREENSHOT_DIR;
let failures = 0;

function check(ok, message) {
  console.log(`${ok ? "ok  " : "FAIL"} ${message}`);
  if (!ok) failures++;
}

const server = await serve("web/dist", 0);
const base = `http://127.0.0.1:${server.address().port}/`;
// CHROMIUM_PATH runs the tests with an existing Chromium instead of the one
// Playwright downloads (useful when versions differ locally).
const browser = await chromium.launch(process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH } : {});
const page = await browser.newPage({ viewport: { width: 1100, height: 720 }, deviceScaleFactor: shots ? 2 : 1 });

const problems = [];
page.on("pageerror", (e) => problems.push(`page error: ${e.message}`));
page.on("console", (m) => {
  if (m.type() === "error" || /Content Security Policy/i.test(m.text())) problems.push(`console: ${m.text()}`);
});

// screen returns the visible terminal text.
const screen = () =>
  page.evaluate(() => {
    const b = globalThis.beTerminal.buffer.active;
    const lines = [];
    for (let i = 0; i < globalThis.beTerminal.rows; i++) lines.push(b.getLine(b.viewportY + i)?.translateToString(true) ?? "");
    return lines.join("\n");
  });
const open = async (query) => {
  await page.goto(base + query);
  await page.waitForFunction(() => globalThis.be && globalThis.beTerminal);
  await page.waitForTimeout(300);
};
const shot = async (name) => {
  if (!shots) return;
  await mkdir(shots, { recursive: true });
  await page.screenshot({ path: join(shots, name) });
};

try {
  // 1. Golden files.
  await open("?agent=claude&seed=42");
  const recordings = await page.evaluate(() => globalThis.be.recordings());
  const files = new Set((await readdir(GOLDEN)).filter((f) => !f.startsWith(".")));
  let identical = 0;
  for (const { file, content } of recordings) {
    files.delete(file);
    const want = await readFile(join(GOLDEN, file), "utf8").catch(() => null);
    if (want === content) identical++;
    else check(false, `web output differs from golden file ${file}`);
  }
  check(identical === recordings.length, `web build matches ${identical}/${recordings.length} golden files byte for byte`);
  check(files.size === 0, `no golden files without a web recording ${[...files].join(", ")}`);

  // 2. Interactive session.
  check((await screen()).includes("Clock Code"), "welcome screen is shown");
  await page.keyboard.type("run engineering");
  await page.keyboard.press("Enter");
  await page.waitForTimeout(4000);
  let text = await screen();
  check(text.includes("❯ run engineering") && text.includes("●"), "run engineering starts a task");
  check(/Esc to stop/.test(text), "status line shows while running");
  await shot("web-running.png");

  // Let output fill the screen several times over, then check that the whole
  // terminal, including the live area at the bottom, is inside the window.
  await page.waitForTimeout(20000);
  const fits = await page.evaluate(() => {
    const screenBox = document.querySelector(".xterm-screen").getBoundingClientRect();
    const container = document.getElementById("terminal").getBoundingClientRect();
    return screenBox.bottom <= container.bottom + 0.5 && screenBox.right <= container.right + 0.5;
  });
  check(fits, "every terminal row fits inside the window");
  text = await screen();
  check(text.includes("accept edits on") && text.split("\n").slice(-6).join("\n").includes("accept edits on"), "footer is still visible at the bottom after a long run");
  await shot("web-long-run.png");

  await page.keyboard.press("Escape");
  await page.waitForTimeout(300);
  text = await screen();
  check(text.includes("Interrupted"), "Esc stops the task and stays in the session");
  check(await page.evaluate(() => !globalThis.be.exited()), "session is still open after Esc");

  await page.keyboard.press("Control+C");
  await page.waitForTimeout(200);
  text = await screen();
  check(text.includes("Session ended.") && text.includes("Press Enter"), "Ctrl+C ends the session");
  await page.keyboard.press("Enter");
  await page.waitForTimeout(300);
  check(await page.evaluate(() => !globalThis.be.exited()), "Enter starts a new session");

  // 3. Share URLs.
  await open("?agent=gemini&topic=devops,nope&seed=7");
  await page.waitForTimeout(2500);
  text = await screen();
  check(text.includes("> run devops") && !text.includes("nope"), "share URL runs allowlisted topics only");
  check((await page.textContent("#title")).includes("gemini"), "share URL selects the agent");
  check(!(await page.$eval("#setup", (d) => d.open)), "a URL with a query skips the setup dialog");
  await shot("web-gemini.png");

  await open("?agent=%3Cimg%20src%3Dx%20onerror%3Dalert(1)%3E&topic=%3Cscript%3E&seed=1e9");
  text = await screen();
  check((await page.textContent("#title")).includes("claude") && !text.includes("❯ run"), "unknown or malicious parameters are ignored");

  // 4. Setup dialog on a first visit (no query).
  await page.goto(base);
  await page.waitForFunction(() => globalThis.be && document.getElementById("setup").open);
  check(true, "first visit without a query opens the setup dialog");
  await page.keyboard.press("Escape");
  check(await page.$eval("#setup", (d) => d.open), "Esc cannot dismiss the dialog before a session exists");
  await page.click('label.option:has(input[value="codex"])');
  await page.click('label.option:has(input[value="devops"])');
  await page.click('label.option:has(input[value="frontend"])');
  await shot("web-setup.png");
  await page.click("#setup-start");
  await page.waitForTimeout(1500);
  const url = new URL(page.url());
  check(url.search === "?agent=codex&topic=devops,frontend", `starting writes the choice into the URL (${url.search})`);
  check((await screen()).includes("› run devops frontend"), "the chosen agent runs the chosen topics");
  await page.reload();
  await page.waitForFunction(() => globalThis.be && globalThis.beTerminal);
  await page.waitForTimeout(300);
  check(!(await page.$eval("#setup", (d) => d.open)) && (await screen()).includes("Codecks"), "reloading the URL skips the dialog");
  await page.click("#new-session");
  check(await page.$eval("#setup", (d) => d.open) && await page.isVisible("#setup-cancel"), "New session reopens the dialog with Cancel");
  await page.keyboard.press("Escape");
  check(!(await page.$eval("#setup", (d) => d.open)), "Esc closes the dialog while a session runs");

  // 5. Loading on a slow connection: the dialog appears before the engine,
  //    a loading indicator shows while waiting, and failures are reported.
  const slow = async (route) => {
    await new Promise((ok) => setTimeout(ok, 2500));
    await route.continue();
  };
  await page.route("**/be.wasm", slow);
  await page.goto(base);
  await page.waitForSelector("#setup[open]", { timeout: 1500 });
  check(await page.evaluate(() => !globalThis.be), "first visit shows the dialog before the engine has loaded");
  await page.click("#setup-start");
  check((await page.textContent("#setup-start")) === "Starting…" && await page.isDisabled("#setup-start"), "Start shows Starting… while the engine loads");
  await page.waitForFunction(() => globalThis.be && !document.getElementById("setup").open, null, { timeout: 10000 });
  await page.waitForTimeout(300);
  check((await screen()).includes("Clock Code") && await page.isHidden("#loading"), "the session starts once the engine is ready");

  await page.goto(base + "?agent=kiro");
  check(await page.isVisible("#loading") && (await page.textContent("#loading-text")) === "Starting session…", "a shared link shows a loading indicator while the engine loads");
  await shot("web-loading.png");
  await page.waitForFunction(() => globalThis.be, null, { timeout: 10000 });
  await page.waitForTimeout(300);
  check(await page.isHidden("#loading") && (await screen()).includes("Kiwi CLI"), "the indicator disappears when the session starts");
  await page.unroute("**/be.wasm", slow);

  await page.route("**/be.wasm", (route) => route.fulfill({ status: 404, body: "" }));
  await page.goto(base + "?agent=claude");
  await page.waitForSelector(".loading.error", { timeout: 5000 });
  check((await page.textContent("#loading-text")).includes("Reload the page"), "a failed download shows an error instead of a blank page");
  await page.unrouteAll();
  problems.length = 0; // the 404 above is expected

  // The install script is served from the site, as text.
  const install = await page.request.get(base + "install.sh");
  check(install.ok() && (await install.text()).startsWith("#!/bin/sh") && install.headers()["content-type"].startsWith("text/plain"), "install.sh is served as text from the site");

  // 6. CSP and errors.
  check(problems.length === 0, `no CSP violations or page errors ${problems.slice(0, 3).join(" | ")}`);
} finally {
  await browser.close();
  server.close();
}

console.log(failures ? `\n${failures} check(s) failed` : "\nall browser checks passed");
process.exit(failures ? 1 : 0);
