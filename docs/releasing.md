# Releasing and deploying

## Web (Cloudflare Pages)

Every push to `main` builds and deploys `web/dist` via `.github/workflows/deploy-web.yml`.
One-time setup by a maintainer:

1. In Cloudflare, create a **Pages** project using **Direct Upload** (no Git integration),
   e.g. `be`.
2. Create an API token with only **Account > Cloudflare Pages > Edit**.
3. In GitHub, **Settings > Environments > New environment** named `cloudflare-pages`:
   - Deployment branches: **Selected branches**, add `main`.
   - Optional: **Required reviewers** to approve each deploy.
   - Secrets: `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`.
   - Variables: `CLOUDFLARE_PAGES_PROJECT` = your project name.
4. Optional: add a custom domain to the Pages project.

Until `CLOUDFLARE_PAGES_PROJECT` is set, the workflow succeeds and skips the deploy.
Security headers, including the CSP, come from `web/_headers`.

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
