# Contributing to be

Thanks for helping boring employees everywhere look busy.

## Quick start

Requirements: Go 1.27+ and `make`.

```sh
git clone https://github.com/boringemployee/be
cd be
make test   # run all tests
make cli    # build bin/be
make web    # build the web version into web/dist (needs Node.js)
make serve  # serve it on http://localhost:8080
make e2e    # browser tests (needs Playwright)
```

## Easiest contributions: agents and topics

A new agent or topic is **one YAML file** in `content/`, with no Go code needed.
Agent layouts are described in [docs/layout.md](docs/layout.md); check yours with
`go run ./cmd/be preview <agent>`. Step-by-step guides:

- [docs/adding-an-agent.md](docs/adding-an-agent.md)
- [docs/adding-a-topic.md](docs/adding-a-topic.md)

Every YAML file is automatically checked against a strict schema and the safety rules below.

## Testing your change

```sh
make test
./bin/be <agent> --seed 42   # try it with a fixed seed
make golden                  # update expected output, then review the diff
```

Golden files in `tests/golden/` show exactly how the output changed. Reviewers read them.

## Rules

- **Fake only.** `be` never runs commands, touches the network, or reads/writes user files.
  Shared packages may not import `os/exec`, `net`, `syscall`, `unsafe` or `plugin`. CI enforces it.
- **No escape codes in content.** Only `internal/render` produces ANSI sequences.
  YAML containing control characters, real URLs, IPs, base64 blobs or dangerous commands
  (`curl | sh`, `rm -rf`, ...) is rejected.
- **Parody, not copy.** No real logos, ASCII art, or verbatim product text. Keep it office-safe.
- **Engine stays terminal-free.** Change the adapters (`internal/adapter/*`) only when needed.
- **Tests.** Shared packages must keep 100% coverage.
- **Dependencies.** Open an issue before adding one.

See [AGENTS.md](AGENTS.md) and [docs/DESIGN.md](docs/DESIGN.md) for the full picture.

## Pull requests

Fill in the pull request template. For visual changes, include a screenshot or short GIF.

## Security

Please do not open public issues for vulnerabilities. See [SECURITY.md](SECURITY.md).
