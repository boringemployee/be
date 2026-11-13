# be — boring employee

> Look busy. Let a fake AI agent do the fake work.

`be` is a parody CLI (and soon a web playground) that pretends to be an AI coding agent.
Start a session, give it a topic, and it will "think", "edit files" and "run tests" forever,
while you stare thoughtfully at the screen.

```
$ be claude
> run engineering
✻ Refactoring session cache to remove circular imports…
```

- **Esc** stops the current task but keeps you in the session.
- **Ctrl+C** exits completely.

> `be` never runs commands, touches your files, or uses the network. Everything is fake.

## Status

Early development. See the [phases in the design doc](docs/DESIGN.md#phases).

| Command          | Status   |
| ---------------- | -------- |
| `be --version`   | ✅       |
| `be claude`      | ✅       |
| `be codex`, `be kiro` | Phase 4 |
| Web playground   | Phase 5  |

## Install

```sh
go install github.com/boringemployee/be/cmd/be@latest
# or build from source:
make cli && ./bin/be --help
```

Signed release binaries will be published from Phase 6.

## Contributing

New agents and topics are just YAML files. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Disclaimer

`be` is a parody. It is not affiliated with, endorsed by, or connected to Anthropic, OpenAI,
Amazon, Google, or any other company. Product names belong to their owners.
Please use responsibly; your manager may also have read this README.

## License

[MIT](LICENSE)
