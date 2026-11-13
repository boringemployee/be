# Design

## Goals

- `be <agent>` opens a convincing fake AI agent session in the terminal.
- `run <topic>` fakes work on that topic forever; Esc stops the task, Ctrl+C exits.
- The same experience runs in the browser, with identical behavior.
- Adding an agent or topic is a YAML file, not code.
- It is impossible for `be` to do anything real.

## Architecture

```
            keys, lines, size                events
  ┌──────────┐ ───────────▶ ┌──────────────┐ ───────▶ ┌──────────────┐
  │ adapter  │              │    engine    │          │    render    │
  │ tty/wasm │ ◀─────────── │ (state, gen) │          │ events→ANSI  │
  └──────────┘   ANSI text  └──────────────┘          └──────────────┘
       ▲                           ▲                         │
       │                    content/*.yaml                   │
       └─────────────────────────────────────────────────────┘
```

| Package               | Role                                                         |
| --------------------- | ------------------------------------------------------------ |
| `internal/engine`     | Session state machine; input in, events out. No I/O.          |
| `internal/generator`  | Seeded fake work: tasks, files, diffs, logs.                  |
| `internal/render`     | Events → ANSI text. The only producer of escape codes.        |
| `internal/profile`    | Loads and validates agent/topic YAML.                         |
| `internal/adapter/tty`| Raw mode (`golang.org/x/term`), key parsing, stdout.          |
| `internal/adapter/wasm`| `syscall/js` glue to xterm.js.                               |
| `content/`            | Embedded YAML: `agents/*.yaml`, `topics/<name>/*.yaml` (packs). |
| `cmd/be`, `cmd/wasm` | Entrypoints; only wiring.                                     |

### Key decisions

- **Go native + WebAssembly** from one codebase, so CLI and web cannot drift.
- **Events, not terminal calls.** The engine is pure and easy to test.
- **Shared ANSI renderer.** xterm.js and real terminals interpret the same bytes, so
  golden files compare CLI and web byte for byte.
- **`x/term` instead of a TUI framework.** Agent CLIs are mostly scrolling text with a live
  line at the bottom; a full-screen framework adds dependencies without benefit.
- **Layouts are data.** Each agent's look is a list of blocks in YAML (header, banner, card,
  notice, list, text, rule; status, input, footer) interpreted by `internal/render`. A new
  agent needs no Go code; a new block type is written once and reusable by every agent.
  See [layout.md](layout.md).
- **Deterministic runs** from a seed and a fake clock: reproducible tests and share URLs
  (`?agent=claude&topic=engineering&seed=42`).

### Session states

```
 idle ──run <topic>──▶ running ──Esc──▶ idle
   │                      │
   └──────Ctrl+C──────────┴──▶ exit
```

## Security model

See [AGENTS.md](../AGENTS.md#guardrails-never-break-these) and [SECURITY.md](../SECURITY.md).
In short: banned imports in shared packages, content treated as untrusted data, a single
escape-code producer, minimal dependencies, pinned CI, signed releases, and a static web
app with a strict CSP.

## Testing

- 100% coverage on shared packages, enforced in CI.
- Unit tests with a fake clock; table-driven state tests; property and fuzz tests.
- Golden files per agent × topic × seed, shared by CLI and web.
- CLI end-to-end via a pseudo-terminal; web end-to-end via Playwright.

## Phases

1. **Foundation**: module, Makefile, docs, templates. ✅
2. **Core engine**: engine, generator, profile, first YAML, 100% coverage. ✅
3. **Renderer + CLI (v0.1)**: `be claude`, `run engineering`, golden tests. ✅
4. **More agents and topics (v0.2)** ✅
   - 4a. Layout blocks, `be preview`, layout reference. ✅
   - 4b. Codex, Gemini, Kiro and Grok parodies as YAML only; more topics; guides. ✅
5. **Web (v0.3)**: WASM build, xterm.js page, shared golden tests.
6. **CI/CD**: tests, coverage gate, signed releases, Cloudflare Pages deploy.
