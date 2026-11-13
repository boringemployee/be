# Adding a topic

A topic is what `run <topic>` pretends to work on. It is one YAML file in
`content/topics/<name>.yaml`. No Go code is needed.

## Fields

All lists need at least one item (at most 100); every string is at most 200 characters.

| Field | Used for | Example |
| --- | --- | --- |
| `name` | `run <name>`; must match the file name | `devops` |
| `description` | Shown by `topics` | `Pipelines, infrastructure and on-call heroics` |
| `tasks` | Task titles; may use `{file}`, `{module}`, `{test}` | `Bump the base image for {module}` |
| `files` | Files the agent "reads" and "edits" | `deploy/k8s/api-deployment.yaml` |
| `modules` | Fills `{module}` | `billing-worker` |
| `tests` | Fills `{test}`; shown as `PASS`/`FAIL` lines | `smoke-test-api` |
| `commands` | Commands the agent "runs" | `terraform plan` |
| `logs` | Command output; may use `{file}`, `{module}`, `{test}` | `Plan: 2 to add, 1 to change, 0 to destroy.` |
| `edits` | Code lines shown in diffs (`-` removed, `+` added) | `replicas: 3` |

Use at least 4 edits and 4 logs so diffs and outputs vary.

## How a task is generated

Each task: pick a title, think, read 1 to 3 files, think, edit a file, run a command, done.
About one task in four fails first, then gets "fixed" and passes. Tasks repeat forever,
and the same `--seed` always produces the same sequence.

## Try it

```sh
go run ./cmd/be preview claude mytopic
go run ./cmd/be claude          # then: run mytopic
make golden && make test
```

## Rules (CI enforces them)

- Commands must look harmless. These are rejected: `curl`, `wget`, `sudo`, `rm -rf`,
  piping into a shell or interpreter, `base64 -d`, and similar.
- No URLs, IP addresses (use host names or plain words), encoded blobs or control characters.
- Unknown placeholders and unknown fields fail.
- Keep it office-safe: fake work, real-sounding but harmless.
