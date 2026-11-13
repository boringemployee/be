# Releasing and deploying

## Web (Cloudflare Pages)

The web version is deployed by Cloudflare Pages' Git integration: every push to `main`
deploys production, and every other branch gets a preview URL. No GitHub workflow or
Cloudflare token is involved.

Project settings (Workers & Pages > your project > Settings > Build):

| Setting | Value |
| --- | --- |
| Production branch | `main` |
| Framework preset | None |
| Build command | `go test ./... && make web` |
| Build output directory | `web/dist` |
| Root directory | *(empty)* |

Environment variables (Settings > Variables and Secrets, for Production and Preview):

| Name | Value | Why |
| --- | --- | --- |
| `GOTOOLCHAIN` | `auto` | Lets the build image's Go download Go 1.27, as `go.mod` requires |
| `NODE_VERSION` | `22` | Node.js for `npm ci` (xterm.js) |

The build command runs the Go tests, including golden files (without the race detector, which
needs a C compiler; GitHub CI runs that), before building, so a failing test fails the deploy. Security
headers, including the CSP, come from `web/_headers`, which is copied into `web/dist`.

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
