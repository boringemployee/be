# Adding an agent

An agent is one YAML file. No Go code is needed.

## 1. Pick names

- **Command name** (`name`): what users type, e.g. `be gemini`. It may name the tool being
  parodied, lowercase, and must match the file name: `content/agents/gemini.yaml`.
- **UI name** (`title`): always a parody, never the real product name. Same for model names.
  Existing examples: Clock Code, Codecks CLI, Geminy CLI, Kiwi CLI, Groan CLI.

## 2. Start from the closest existing agent

| Agent | Welcome | Input | Tools |
| --- | --- | --- | --- |
| `claude` | header with art + notice | ruled | `boxed` |
| `codex` | card + list | bare with placeholder | `bullet` |
| `gemini` | gradient banner + numbered list | boxed | `check` |
| `kiro` | header with art + notice + text | bare under a rule | `inline` |
| `grok` | banner + text + list | ruled | `boxed` |

```sh
cp content/agents/codex.yaml content/agents/myagent.yaml
```

Then edit it. Every field and block is described in [layout.md](layout.md).

## 3. Preview it

```sh
go run ./cmd/be preview myagent            # sample screen
go run ./cmd/be preview myagent devops     # with another topic
go run ./cmd/be myagent                    # the real interactive session
```

Try a narrow terminal too: everything is truncated to fit, and footer parts that do not fit
are dropped.

## 4. Test and record golden files

```sh
make golden   # records preview, commands and a run in render/, and a mixed run in content/
make test
```

Review the new files in `tests/golden/testdata/render/` and `content/`; the `.events` files
are readable text.

## Rules (CI enforces them)

- Original art only. No real logos or mascots, and no verbatim text from real products.
- No control characters, URLs, IP addresses, encoded blobs or dangerous commands.
- Unknown fields or blocks fail. If the blocks cannot express what you need, open an issue:
  a new block type is added once in Go and every agent can use it.
- Keep it office-safe, and make it look legitimate: UI text must read like a real tool.
  Words like "fake", "pretend" or "parody" fail the golden tests.
