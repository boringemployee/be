# AGENTS.md

Guidelines for AI coding agents working in this repository.

## Project

`be` ("boring employee") is a fun CLI and web app that pretends to be an AI coding agent
(`be claude`, `be codex`, `be kiro`, ...). Inside a session, `run <topic>` fakes work on that
topic forever; **Esc** stops the task but stays in the session, **Ctrl+C** exits.
It is a parody: it never does real work.

## Architecture

- Go 1.27. One engine, two thin frontends: native CLI (`cmd/be`) and
  WebAssembly (`cmd/wasm`, rendered with xterm.js in `web/`).
- The engine takes input (keys, lines, size) and emits events. It never touches a terminal.
- `internal/render` turns events into ANSI text and is the **only** place that creates
  escape sequences. Both frontends display that same text.
- Agents and topics are YAML data files in `content/`, embedded with `go:embed`. Adding an
  agent or topic should need no Go code.
- Runs are deterministic from a seed and a fake clock.
- CLI terminal handling uses `golang.org/x/term` only; no TUI frameworks.

```
cmd/be  cmd/wasm  internal/{engine,generator,render,profile,adapter/{tty,wasm}}
content/{agents,topics/<name>}  web/  tests/golden/  docs/
```

## Guardrails (never break these)

1. **Fake only.** Shared packages (`engine`, `generator`, `render`, `profile`, `content`)
   must not import `os/exec`, `net`, `net/*`, `syscall`, `unsafe`, `plugin`, or write files.
   Nothing ever executes commands, touches the network, or reads/writes user files.
2. **Content is data.** YAML strings must not contain control characters (ESC, BEL, ...),
   real URLs, IP addresses, base64 blobs, or copy-pasteable dangerous commands
   (`curl | sh`, `rm -rf`, ...). Schemas are strict: unknown fields fail.
3. **Parody, not copy.** Mimic layouts, but never real logos, mascots, ASCII art, or verbatim
   product text. The UI uses parody names (e.g. "Clock Code", "Magnum Opus 5"), never real
   product or model names. CLI commands (`be claude`) may name the tool being parodied.
   Office-safe.
4. **Minimal dependencies.** Ask before adding one. Pin GitHub Actions by commit SHA,
   default workflow permissions to read-only, never use `pull_request_target`, and keep
   secrets (e.g. `CLOUDFLARE_API_TOKEN`) behind a protected environment.
5. **Web stays static.** Strict CSP, no third-party runtime scripts, no analytics,
   cookies, or backend. Share URL params are allowlisted (`agent`, `topic`, `seed`).

## Testing

- Shared packages require **100% coverage**; CI fails below it.
- Use a fake clock, never real sleeps. Use table-driven, property, and fuzz tests.
- Golden files in `tests/golden/` are the source of truth; CLI and web must match them.
  Update them only intentionally with `make golden`.
- Run `make test` before every commit.

## Commits

- One commit per phase/feature, clear imperative subject.
- End every commit message with exactly this trailer and nothing else:

  ```
  Co-Authored-By: Boring Employee <be@boringemployee.com>
  ```

  Never add any other co-author line, and never include AI session links or IDs.
