// Web frontend for be. All session logic runs in be.wasm (the same Go code as
// the CLI); this file only connects it to xterm.js and the page, and shows
// the New session dialog and a loading indicator while be.wasm downloads.
//
// Share URLs: ?agent=<name>&topic=<a,b>&seed=<digits>. Only these parameters
// are read, and each is checked against an allowlist before use. Without a
// query the "New session" dialog opens; starting a session writes the choice
// into the URL, so reloading or sharing it skips the dialog.
"use strict";

(async () => {
  const FRAME_MS = 100;
  const CWD = "~/work/app";
  const $ = (id) => document.getElementById(id);
  const termEl = $("terminal");
  const title = $("title");
  const dialog = $("setup");

  // --- Loading ---------------------------------------------------------------
  // The page shows something right away: catalog.json (a few KB) is enough for
  // the New session dialog, while be.wasm (several MB) downloads in parallel
  // behind a progress indicator.

  const loading = $("loading");
  const loadingBar = $("loading-bar");
  function showLoading(text) {
    $("loading-text").textContent = text;
    loading.hidden = false;
  }
  function progress(fraction) {
    loadingBar.parentElement.classList.toggle("indeterminate", fraction == null);
    loadingBar.style.width = fraction == null ? "" : `${Math.round(fraction * 100)}%`;
  }
  let failed = false;
  function loadFailed() {
    failed = true;
    if (dialog.open) dialog.close();
    loading.classList.add("error");
    showLoading("Couldn't load the session. Reload the page to try again.");
  }

  // loadEngine downloads and starts be.wasm, reporting progress when the size
  // is known (not for compressed responses, whose length differs).
  async function loadEngine() {
    const res = await fetch("be.wasm");
    if (!res.ok) throw new Error(`be.wasm: HTTP ${res.status}`);
    const total = res.headers.get("Content-Encoding") ? 0 : Number(res.headers.get("Content-Length")) || 0;
    progress(total ? 0 : null);
    const reader = res.body.getReader();
    const chunks = [];
    let received = 0;
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      chunks.push(value);
      received += value.length;
      if (total) progress(Math.min(received / total, 1));
    }
    const bytes = new Uint8Array(received);
    let offset = 0;
    for (const c of chunks) {
      bytes.set(c, offset);
      offset += c.length;
    }
    const go = new Go();
    const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(instance); // registers globalThis.be
    return globalThis.be;
  }

  let be = null; // set once the engine is ready
  const engine = loadEngine().then((b) => (be = b));
  engine.catch(loadFailed);

  // The catalog comes from catalog.json; if that fails, from the engine.
  const catalog = await fetch("catalog.json")
    .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`catalog.json: HTTP ${r.status}`))))
    .catch(() => engine.then((b) => b.catalog()))
    .catch(() => null);
  if (!catalog) return; // loadFailed has already shown the error
  const agentNames = catalog.agents.map((a) => a.name);
  const topicNames = catalog.topics.map((t) => t.name);

  // Session choice, from the URL when it has a query, otherwise from the dialog.
  const params = new URLSearchParams(location.search);
  const hasQuery = ["agent", "topic", "seed"].some((k) => params.has(k));
  let agent = agentNames.includes(params.get("agent")) ? params.get("agent") : agentNames[0];
  let runTopics = (params.get("topic") || "").split(",").filter((t) => topicNames.includes(t));
  const seed = /^[0-9]{1,19}$/.test(params.get("seed") || "") ? params.get("seed") : "";

  const term = new Terminal({
    cursorBlink: true,
    fontFamily: getComputedStyle(document.documentElement).getPropertyValue("--font"),
    fontSize: 14,
    lineHeight: 1.0,
    scrollback: 5000,
    theme: { background: "#1b1c1e", foreground: "#d4d4d4", cursor: "#d4d4d4", selectionBackground: "#3a3d41" },
  });
  term.open(termEl);
  globalThis.beTerminal = term; // used by the browser tests to read the screen
  let started = false;

  // Size the terminal to its container. The cell size is measured from what
  // xterm.js actually renders (font metrics make rows taller than fontSize),
  // so every row, including the input box and footer, stays inside the window.
  function fit() {
    const rendered = termEl.querySelector(".xterm-screen").getBoundingClientRect();
    const cellW = rendered.width / term.cols;
    const cellH = rendered.height / term.rows;
    const box = termEl.getBoundingClientRect();
    const style = getComputedStyle(termEl);
    const innerW = box.width - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
    const innerH = box.height - parseFloat(style.paddingTop) - parseFloat(style.paddingBottom);
    const cols = Math.max(20, Math.floor(innerW / cellW) - 1); // keep room for the scrollbar
    const rows = Math.max(5, Math.floor(innerH / cellH));
    term.resize(cols, rows);
    if (be) be.resize(cols);
    title.textContent = `${CWD} — ${agent} — ${cols}×${rows}`;
    return cols;
  }

  // start begins a session, first waiting for the engine if it is still
  // downloading.
  async function start(topics) {
    if (failed) return; // keep the error message visible
    if (!be) {
      showLoading("Starting session…");
      try {
        await engine;
      } catch {
        return; // loadFailed has already shown the error
      }
    }
    loading.hidden = true;
    started = true;
    term.reset();
    const cols = fit();
    term.write(be.start(agent, topics.join(","), seed, cols, CWD));
    term.focus();
  }

  // --- New session dialog -------------------------------------------------

  function option(type, name, value, label, desc, accent) {
    const el = document.createElement("label");
    el.className = "option";
    const input = document.createElement("input");
    Object.assign(input, { type, name, value });
    const text = document.createElement("span");
    const strong = document.createElement("span");
    strong.className = "name";
    if (accent) {
      const swatch = document.createElement("span");
      swatch.className = "swatch";
      swatch.style.background = accent; // validated #rrggbb from the agent YAML
      strong.append(swatch);
    }
    strong.append(label);
    const small = document.createElement("span");
    small.className = "desc";
    small.textContent = desc;
    text.append(strong, small);
    el.append(input, text);
    if (accent) el.style.setProperty("--accent", accent);
    return el;
  }

  for (const a of catalog.agents) $("agent-options").append(option("radio", "agent", a.name, a.title, `be ${a.name}`, a.accent));
  for (const t of catalog.topics) $("topic-options").append(option("checkbox", "topic", t.name, t.name, t.description));

  function openSetup() {
    for (const el of document.querySelectorAll('input[name="agent"]')) el.checked = el.value === agent;
    for (const el of document.querySelectorAll('input[name="topic"]')) el.checked = runTopics.includes(el.value);
    $("setup-cancel").hidden = !started;
    dialog.showModal();
    document.querySelector('input[name="agent"]:checked')?.focus();
  }

  $("setup-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    agent = document.querySelector('input[name="agent"]:checked')?.value || agentNames[0];
    runTopics = [...document.querySelectorAll('input[name="topic"]:checked')].map((el) => el.value);
    // Names come from the allowlist, so the query needs no escaping and the
    // comma stays readable in shared links.
    const query = `?agent=${agent}` + (runTopics.length ? `&topic=${runTopics.join(",")}` : "");
    history.replaceState(null, "", location.pathname + query);
    // If the engine is still downloading, keep the dialog open with a busy
    // button, then start as soon as it is ready.
    const button = $("setup-start");
    if (!be) {
      button.disabled = true;
      button.textContent = "Starting…";
      try {
        await engine;
      } catch {
        return;
      } finally {
        button.disabled = false;
        button.textContent = "Start session";
      }
    }
    dialog.close();
    start(runTopics);
  });
  $("setup-cancel").addEventListener("click", () => dialog.close());
  // Esc closes the dialog only when a session is already running behind it.
  dialog.addEventListener("cancel", (e) => { if (!started) e.preventDefault(); });
  dialog.addEventListener("close", () => term.focus());
  $("new-session").addEventListener("click", openSetup);

  // --- Terminal I/O ---------------------------------------------------------

  term.onData((data) => {
    if (!started || !be) return;
    if (be.exited()) {
      if (data === "\r") start([]);
      return;
    }
    term.write(be.input(data));
    if (be.exited()) term.write("\r\nPress Enter to start a new session.\r\n");
  });

  setInterval(() => {
    if (!started || !be) return;
    const out = be.frame();
    if (out) term.write(out);
  }, FRAME_MS);

  let resizeTimer;
  addEventListener("resize", () => {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(fit, 100);
  });

  if (hasQuery) start(runTopics);
  else {
    loading.hidden = true; // the dialog shows instead; the engine keeps loading
    fit();
    openSetup();
  }
})();
