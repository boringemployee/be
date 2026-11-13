# Agent layout reference

An agent's look is defined entirely in its YAML file (`content/agents/<name>.yaml`).
No Go code is needed. Check your work with:

```sh
go run ./cmd/be preview <agent> [topic]
```

## Top-level fields

| Field | Required | Description |
| --- | --- | --- |
| `name` | yes | Command name (`be <name>`), lowercase, must match the file name |
| `title` | yes | Parody product name shown in the UI (never a real product name) |
| `version` | yes | Fake version, like `v0.4.2` |
| `accent` | yes | Main color, `#rrggbb` |
| `welcome` | yes | 1–8 blocks drawn once at the start |
| `live` | yes | Blocks drawn under the output and redrawn in place |
| `tools` | yes | How tool calls look: `boxed`, `inline`, `bullet`, `check` |
| `spinner` | yes | Spinner frames, e.g. `["◐", "◓", "◑", "◒"]` |
| `thinking_words` | yes | Words shown while "thinking" |
| `done_messages` | yes | Shown when a task finishes |
| `stopped_message` | yes | Shown after Esc |

Text fields may use `{title}`, `{version}` (shown dim) and `{cwd}` (the current folder).

## Welcome blocks

Each block is a one-key map. Blocks are separated by a blank line.

```yaml
welcome:
  - header:                     # small art on the left, lines on the right
      art: [" ▄▀▀▀▄ ", "█  █▄ █", " ▀▄▄▄▀ "]   # optional, max 6 rows x 16 columns
      lines: ["{title} {version}", "model line", "{cwd}"]   # first line bold
  - banner:                     # big block letters (A-Z 0-9 space . ! -, max 12)
      text: "GEMINY"
      gradient: ["#4796E4", "#C3677F"]          # optional, up to 6 colors
  - card: ["{title} ({version})", "model: fake-5", "directory: {cwd}"]  # rounded box
  - notice: ["Bold first line", "dim lines with an accent bar, wrapped"]
  - list: { title: "Tips:", items: ["one", "two"], numbered: true }
  - text: ["plain dim lines"]
  - rule: {}                    # horizontal line
```

## Live blocks

Exactly one `status` and one `input`; optional `footer` and `rule`s; at most 6 blocks.

```yaml
live:
  - status: { idle: "○ low · /effort" }    # spinner while working, else idle text (right-aligned)
  - input:
      style: ruled          # bare | ruled (rules above/below) | boxed (rounded box)
      marker: "❯"
      placeholder: "Type your message"     # optional, shown dim when empty
  - footer:
      highlight: "▸▸ fake mode on"         # bright, first
      left: "esc to stop"                  # dim
      center: "no sandbox"                 # optional, centered
      right: "100% context left"           # optional, right-aligned
```

## Tool styles

| `tools` | Read | Edit | Run |
| --- | --- | --- | --- |
| `boxed` | `● Read(a.go)` | `● Update(a.go)` | `● Run(make)` |
| `inline` | `→ read a.go` | `→ edit a.go` | `→ run make` |
| `bullet` | `• Read a.go` | `• Edited a.go` | `• Ran make` |
| `check` | `✓ ReadFile a.go` | `✓ Edit a.go` | `✓ Shell make` |

## Rules

All text passes the safety checks in [AGENTS.md](../AGENTS.md): no control characters,
URLs, IPs, encoded blobs or dangerous commands. Unknown fields or blocks are rejected.
Use original art and parody names only.

Need something the blocks cannot express? Open an issue: a new block type is added once in
`internal/render` and then every agent can use it.
