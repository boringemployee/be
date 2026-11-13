// Web frontend for be. All session logic runs in be.wasm (the same Go code as
// the CLI); this file only connects it to xterm.js and the page.
//
// Share URLs: ?agent=<name>&topic=<a,b>&seed=<digits>. Only these parameters
// are read, and each is checked against an allowlist before use.
"use strict";

(async () => {
  const FRAME_MS = 100;
  const CWD = "~/work/app";
  const termEl = document.getElementById("terminal");
  const select = document.getElementById("agent");
  const title = document.getElementById("title");

  const go = new Go();
  const wasm = await WebAssembly.instantiateStreaming(fetch("be.wasm"), go.importObject)
    .catch(async () => {
      // Some static servers send the wrong MIME type for .wasm.
      const bytes = await (await fetch("be.wasm")).arrayBuffer();
      return WebAssembly.instantiate(bytes, go.importObject);
    });
  go.run(wasm.instance); // registers globalThis.be
  const be = globalThis.be;

  const agents = be.agents().split(",");
  const topics = be.topics().split(",");

  // Read and allowlist the share URL parameters.
  const params = new URLSearchParams(location.search);
  let agent = agents.includes(params.get("agent")) ? params.get("agent") : agents[0];
  const runTopics = (params.get("topic") || "").split(",").filter((t) => topics.includes(t));
  const seed = /^[0-9]{1,19}$/.test(params.get("seed") || "") ? params.get("seed") : "";

  for (const name of agents) {
    const opt = document.createElement("option");
    opt.value = name;
    opt.textContent = name;
    select.append(opt);
  }
  select.value = agent;

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

  // Size the terminal to its container from the measured character cell.
  function fit() {
    const probe = document.createElement("span");
    probe.textContent = "W".repeat(50);
    probe.style.font = `${term.options.fontSize}px ${term.options.fontFamily}`;
    probe.style.position = "absolute";
    probe.style.visibility = "hidden";
    document.body.append(probe);
    const cellW = probe.getBoundingClientRect().width / 50;
    probe.remove();
    const cellH = term.options.fontSize * term.options.lineHeight;
    const box = termEl.getBoundingClientRect();
    const cols = Math.max(20, Math.floor((box.width - 16) / cellW));
    const rows = Math.max(5, Math.floor((box.height - 12) / cellH));
    term.resize(cols, rows);
    be.resize(cols);
    return cols;
  }

  function start(topicList) {
    term.reset();
    const cols = fit();
    title.textContent = `${CWD} — ${agent} — ${cols}×${term.rows}`;
    term.write(be.start(agent, topicList.join(","), seed, cols, CWD));
    term.focus();
  }

  term.onData((data) => {
    if (be.exited()) {
      if (data === "\r") start([]);
      return;
    }
    term.write(be.input(data));
    if (be.exited()) term.write("\r\nPress Enter to start a new session.\r\n");
  });

  setInterval(() => {
    const out = be.frame();
    if (out) term.write(out);
  }, FRAME_MS);

  let resizeTimer;
  addEventListener("resize", () => {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(() => {
      const cols = fit();
      title.textContent = `${CWD} — ${agent} — ${cols}×${term.rows}`;
    }, 100);
  });

  select.addEventListener("change", () => {
    agent = select.value;
    const url = new URL(location.href);
    url.search = "";
    url.searchParams.set("agent", agent);
    history.replaceState(null, "", url);
    start([]);
  });

  start(runTopics);
})();
