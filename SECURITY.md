# Security Policy

`be` is designed to do nothing real: it never executes commands, makes network requests,
or reads or writes user files. A change that breaks that promise is a security bug.

## Reporting a vulnerability

Please report privately through
[GitHub private vulnerability reporting](https://github.com/boringemployee/be/security/advisories/new).
Do not open a public issue.

Examples of what we want to hear about:

- Any way to make `be` run a command, access the network, or touch files.
- Terminal escape sequences that reach the screen from content or input
  (clipboard writes, hyperlinks, title changes, ...).
- Weaknesses in the build, release, signing, or web deploy pipeline.
- Script injection or CSP bypasses in the web playground.

We aim to reply within 7 days.

## Supported versions

Only the latest release is supported.

## Verifying releases

Releases are built by GitHub Actions from a tagged commit, never by hand. Each release has
`checksums.txt`, a keyless cosign signature for it, and GitHub build provenance for every
archive.

```sh
# 1. The checksums file was signed by this repository's release workflow.
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/boringemployee/be/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com

# 2. Your archive matches the signed checksums.
sha256sum --ignore-missing -c checksums.txt

# 3. Optional: provenance shows which commit and workflow built it.
gh attestation verify be_v1.2.3_linux_amd64.tar.gz --repo boringemployee/be
```

## How the pipeline is protected

- Actions are pinned to commit SHAs; workflows default to read-only permissions and never
  use `pull_request_target` (a test in `internal/guard` enforces this).
- Releases run in the `release` environment; the web deploy runs in `cloudflare-pages`,
  which holds the only secrets and is limited to the `main` branch.
- Dependabot proposes dependency updates weekly; CI and code owner review apply to each.
