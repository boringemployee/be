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

From Phase 6, release binaries come with `checksums.txt`, a cosign signature, and GitHub
build provenance. Instructions will be added here with the first release.
