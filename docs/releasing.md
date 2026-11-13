# Releasing and deploying

## Web (Cloudflare Pages)

The web version is deployed by Cloudflare Pages' Git integration: every push to `main`
deploys production. Preview builds are turned off. No GitHub workflow or Cloudflare token
is involved.

Project settings (Workers & Pages > your project > Settings > Build):

| Setting | Value |
| --- | --- |
| Production branch | `main` |
| Framework preset | None |
| Build command | `make web` |
| Build output directory | `web/dist` |
| Root directory | *(empty)* |
| Preview branches (Settings > Builds > Branch control) | None |

Environment variables (Settings > Variables and Secrets, for Production and Preview):

| Name | Value | Why |
| --- | --- | --- |
| `GOTOOLCHAIN` | `auto` | Lets the build image's Go download Go 1.27, as `go.mod` requires |
| `NODE_VERSION` | `22` | Node.js for `npm ci` (xterm.js) |

Cloudflare only builds; it does not test. Tests are enforced before anything reaches
`main` (see below), so every commit Cloudflare deploys has already passed CI. Security
headers, including the CSP, come from `web/_headers`, which is copied into `web/dist`.

## Protecting main

`main` only changes through pull requests that pass CI. One-time setup in GitHub,
**Settings > Rules > Rulesets > New ruleset > New branch ruleset**:

| Setting | Value |
| --- | --- |
| Ruleset name | `protect-main` |
| Enforcement status | Active |
| Bypass list | *(empty, so the rules apply to admins too)* |
| Target branches | Add target > Include default branch |
| Restrict deletions | on |
| Block force pushes | on |
| Require a pull request before merging | on; required approvals: 1 if you have reviewers, otherwise 0; require review from Code Owners if approvals > 0 |
| Require status checks to pass | on; "Require branches to be up to date before merging" on |
| Status checks | `Go (lint, tests, coverage, fuzz, vulnerabilities)` and `Web (WebAssembly build and browser tests)` (source: GitHub Actions) |

Direct pushes to `main` are then rejected; every change, including documentation and
AGENTS.md, goes through a pull request. Pull requests are squash-merged: one pull request
per phase or feature becomes one commit on `main`. When merging, edit the squash message so
it ends with the `Co-Authored-By` trailer from AGENTS.md exactly once.

## CLI releases

1. Make sure `main` is green.
2. Create an environment named `release` (Settings > Environments); restrict it to tags
   matching `v*` and, optionally, require a reviewer.
3. Tag and push:

   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```

The release workflow tests, builds archives for Linux, macOS and Windows (amd64, arm64),
signs `checksums.txt` with cosign, attests build provenance and publishes a GitHub Release.
Users verify releases as described in [SECURITY.md](../SECURITY.md#verifying-releases).
