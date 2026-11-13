# Adding a topic

A topic is what `run <topic>` pretends to work on. It is a folder of YAML files in
`content/topics/<name>/`. No Go code is needed.

```
content/topics/devops/
├── topic.yaml        # description, plus lists shared by every pack
├── kubernetes.yaml   # a pack
└── terraform.yaml    # another pack (can be partial)
```

## Packs

Each file other than `topic.yaml` is a **pack**: a coherent set of work for **one kind of
file**, such as Go code, Kubernetes manifests or CSS. Every task picks one pack and uses only that pack's lists, so a task never
mixes Go files with Python edits or Terraform logs.

- A pack may leave lists out. Missing lists come from `topic.yaml`.
- If `topic.yaml` has `tasks`, it is also a pack.
- To enrich a topic, add a new pack file. To add variety to an existing pack, add lines to it.

## topic.yaml

```yaml
description: Pipelines, infrastructure and on-call heroics   # shown by "topics"
modules: [api-gateway, billing-worker]   # optional shared lists, used by packs that omit them
tests: [smoke-test-api, canary-checkout]
```

The folder name is the topic name (`run devops`): lowercase letters, digits and dashes.

## Pack fields

After falling back to `topic.yaml`, every pack must have at least one item in each list
(at most 100). Every string is at most 200 characters.

| Field | Used for | Example |
| --- | --- | --- |
| `tasks` | Task titles; may use `{file}`, `{module}`, `{test}` | `Bump the base image for {module}` |
| `files` | Files the agent "reads" and "edits" | `deploy/k8s/api-deployment.yaml` |
| `modules` | Fills `{module}` | `billing-worker` |
| `tests` | Fills `{test}`; shown as `PASS`/`FAIL` lines | `smoke-test-api` |
| `commands` | Commands the agent "runs" | `terraform plan` |
| `logs` | Command output; may use `{file}`, `{module}`, `{test}` | `Plan: 2 to add, 1 to change, 0 to destroy.` |
| `edits` | Changes shown in diffs: `from` (optional, removed) becomes `to` (added) | `{from: "replicas: 2", to: "replicas: 3"}` |

Use at least 4 edits and 4 logs per pack so diffs and outputs vary. Every edit must make
sense in every file of its pack: if a line only fits some files, move them to their own pack.

A partial pack that reuses the shared `modules` and `tests`:

```yaml
# content/topics/devops/terraform.yaml
tasks: ["Split the {module} state file", "Fix drift in {file}"]
files: [infra/terraform/main.tf, infra/terraform/outputs.tf]
commands: [terraform validate, terraform plan]
logs: ["Success! The configuration is valid.", "formatting {file}"]
edits:
  - from: 'version = "~> 4.0"'
    to: 'version = "~> 5.0"'
  - to: "lifecycle { prevent_destroy = true }"   # no from: a pure addition
```

## How a task is generated

Each task: pick a pack, pick a title, think, read 1 to 3 files, think, edit a file, run a
command, done. About one task in four fails first, then gets "fixed" and passes. Tasks
repeat forever, and the same `--seed` always produces the same sequence.

## Try it

```sh
go run ./cmd/be preview claude mytopic
go run ./cmd/be preview claude mytopic --seed 7   # other seeds show other packs
go run ./cmd/be claude                            # then: run mytopic
make golden && make test
```

## Rules (CI enforces them)

- Commands must look harmless. These are rejected: `curl`, `wget`, `sudo`, `rm -rf`,
  piping into a shell or interpreter, `base64 -d`, and similar.
- No URLs, IP addresses (use host names or plain words), encoded blobs or control characters.
- Unknown placeholders and unknown fields fail; packs cannot have a `description`.
- Keep it office-safe and realistic: it should read like real work, with no giveaway words
  such as "fake" or "pretend" (the golden tests check this).
