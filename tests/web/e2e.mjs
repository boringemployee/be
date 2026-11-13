// Browser tests for the web build (run with: make e2e).
//
// 1. The WebAssembly build reproduces every golden file byte for byte, which
//    proves the CLI and the web share identical behavior.
// 2. An interactive session works: run, Esc, Ctrl+C and restart.
// 3. Share URL parameters are allowlisted.
// 4. The page runs under its production CSP with no violations or errors.
//
// Set SCREENSHOT_DIR to also save screenshots.
import { readFile, readdir, mkdir } from "node:fs/promises";
import { join } from "node:path";
import { createRequire } from "node:module";
import { serve } from "./serve.mjs";

// require() finds a globally installed Playwright, which ESM import does not.
const { chromium } = createRequire(import.meta.url)("playwright");

const GOLDEN = "tests/golden/testdata";
const shots = process.env.SCREENSHOT_DIR;
let failures = 0;

function check(ok, message) {
  console.log(`${ok ? "ok  " : "FAIL"} ${message}`);
  if (!ok) failures++;
}

const server = await serve("web/dist", 0);
const base = `http://127.0.0.1:${server.address().port}/`;
const browser = await chromium.launch();
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
  check(await page.$eval("#agent", (s) => s.value) === "gemini", "share URL selects the agent");
  await shot("web-gemini.png");

  await open("?agent=%3Cimg%20src%3Dx%20onerror%3Dalert(1)%3E&topic=%3Cscript%3E&seed=1e9");
  text = await screen();
  check(await page.$eval("#agent", (s) => s.value) === "claude" && !text.includes("❯ run"), "unknown or malicious parameters are ignored");

  // 4. CSP and errors.
  check(problems.length === 0, `no CSP violations or page errors ${problems.slice(0, 3).join(" | ")}`);
} finally {
  await browser.close();
  server.close();
}

console.log(failures ? `\n${failures} check(s) failed` : "\nall browser checks passed");
process.exit(failures ? 1 : 0);
