# be — boring employee

> Look busy. Let a fake AI agent do the fake work.

`be` is a parody CLI (and soon a web playground) that pretends to be an AI coding agent.
Start a session, give it a topic, and it will "think", "edit files" and "run tests" forever,
while you stare thoughtfully at the screen.

```
$ be claude
> run engineering
● Refactoring session cache to remove circular imports…
```

- **Esc** stops the current task but keeps you in the session.
- **Ctrl+C** exits completely.

> `be` never runs commands, touches your files, or uses the network. Everything is fake.

## Status

Early development. See the [phases in the design doc](docs/DESIGN.md#phases).

| Command | UI name | Status |
| --- | --- | --- |
| `be claude` | Clock Code | ✅ |
| `be codex` | Codecks CLI | ✅ |
| `be gemini` | Geminy CLI | ✅ |
| `be kiro` | Kiwi CLI | ✅ |
| `be grok` | Groan CLI | ✅ |
| `be preview <agent>` | sample screen | ✅ |
| Web version | all agents | ✅ |

Topics: `data`, `devops`, `engineering`, `frontend`, `security` (type `topics` in a session).
Mix several with `run devops engineering frontend`: each task picks one of them.

## Web version

The same engine compiled to WebAssembly, in a browser terminal (xterm.js):

```sh
make serve    # builds web/dist and serves http://localhost:8080
```

On a first visit a **New session** dialog asks for an agent and optional topics. Starting
writes the choice into the URL (e.g. `?agent=gemini&topic=devops,frontend`), so reloading or
sharing the link skips the dialog. `seed` can be added for a reproducible session. Only these
three parameters are read, and each is checked against an allowlist.

## Install

Install the latest version on Linux or macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/boringemployee/be/main/install.sh | sh
```

With Go 1.21+ installed it runs `go install`; otherwise it downloads the release for your OS
and CPU, verifies its checksum (and signature, if `cosign` is installed) and installs `be`
into `~/.local/bin`. [Read the script](install.sh) first; it is short.

Install a specific version:

```sh
curl -fsSL https://raw.githubusercontent.com/boringemployee/be/main/install.sh | sh -s -- v0.1.0
```

Download the release binary even when Go is installed:

```sh
curl -fsSL https://raw.githubusercontent.com/boringemployee/be/main/install.sh | BE_METHOD=binary sh
```

Install into another folder, such as `/usr/local/bin`:

```sh
curl -fsSL https://raw.githubusercontent.com/boringemployee/be/main/install.sh | BE_INSTALL_DIR=/usr/local/bin sudo -E sh
```

Install with Go directly:

```sh
go install github.com/boringemployee/be/cmd/be@latest
```

Build from source:

```sh
make cli
```

Then run `./bin/be --help`.

On Windows, download the `.zip` from [Releases](https://github.com/boringemployee/be/releases).
Releases are signed; see [SECURITY.md](SECURITY.md#verifying-releases) to verify them.

## Contributing

New agents and topics are just YAML files. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Disclaimer

`be` is a parody. It is not affiliated with, endorsed by, or connected to Anthropic, OpenAI,
Amazon, Google, or any other company. Product names belong to their owners.
All names, mascots and text shown in the UI are original parodies (e.g. "Clock Code").
If you represent a trademark owner and want something changed or removed, please open an
issue and we will act on it promptly.

Please use responsibly; your manager may also have read this README.

## License

[MIT](LICENSE)
