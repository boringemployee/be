#!/bin/sh
# Installs be. Read it before running it; it is short on purpose.
#
#   curl -fsSL https://be.boringemployee.com/install.sh | sh
#   curl -fsSL https://be.boringemployee.com/install.sh | sh -s -- v0.1.0
#
# With Go 1.21 or newer installed, it runs "go install". Otherwise it downloads
# the release archive for this OS and CPU, checks its SHA-256 against the
# release's checksums.txt (and that file's signature, if cosign is installed)
# and copies the binary into a bin folder.
#
# Settings (environment variables):
#   BE_METHOD=auto|go|binary   how to install (default: auto)
#   BE_INSTALL_DIR=<dir>       where to put the binary (default: ~/.local/bin)
#   BE_BASE_URL=<url>          release download base (default: GitHub Releases)
set -eu

REPO="boringemployee/be"
MODULE="github.com/${REPO}/cmd/be"
VERSION="${1:-${BE_VERSION:-latest}}"
METHOD="${BE_METHOD:-auto}"
BASE_URL="${BE_BASE_URL:-https://github.com/${REPO}/releases}"
INSTALL_DIR="${BE_INSTALL_DIR:-${HOME}/.local/bin}"

say() { printf 'be-install: %s\n' "$*" >&2; }
fail() { say "error: $*"; exit 1; }
has() { command -v "$1" >/dev/null 2>&1; }

case "$VERSION" in
  latest | v[0-9]*.[0-9]*.[0-9]*) ;;
  *) fail "version must look like v1.2.3 or 'latest', got '$VERSION'" ;;
esac

# go_ok reports whether a Go toolchain new enough to fetch Go 1.27 is installed.
go_ok() {
  has go || return 1
  v=$(go env GOVERSION 2>/dev/null | sed -n 's/^go1\.\([0-9][0-9]*\).*/\1/p')
  [ -n "$v" ] && [ "$v" -ge 21 ]
}

install_with_go() {
  say "installing ${MODULE}@${VERSION} with $(go env GOVERSION)"
  GOTOOLCHAIN=auto go install "${MODULE}@${VERSION}"
  bin="$(go env GOBIN)"
  [ -n "$bin" ] || bin="$(go env GOPATH)/bin"
  done_msg "$bin"
}

fetch() { # fetch <url> <file>; HTTPS only, except a local test server
  case "$1" in
    https://*) secure=1 ;;
    http://127.0.0.1:* | http://localhost:*) secure=0 ;;
    *) fail "refusing to download over an insecure URL: $1" ;;
  esac
  if has curl; then
    if [ "$secure" = 1 ]; then
      curl -fsSL --proto '=https' --proto-redir '=https' --tlsv1.2 -o "$2" "$1"
    else
      curl -fsSL -o "$2" "$1"
    fi
  elif has wget; then
    if [ "$secure" = 1 ]; then wget -q --https-only -O "$2" "$1"; else wget -q -O "$2" "$1"; fi
  else
    fail "curl or wget is required"
  fi
}

sha256() {
  if has sha256sum; then sha256sum "$1" | cut -d' ' -f1
  elif has shasum; then shasum -a 256 "$1" | cut -d' ' -f1
  else fail "sha256sum or shasum is required to verify the download"
  fi
}

latest_version() {
  # GitHub redirects /releases/latest to /releases/tag/<version>.
  has curl || fail "curl is required to find the latest version; pass a version instead"
  url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "${BASE_URL}/latest") || fail "cannot reach ${BASE_URL}"
  v=${url##*/}
  case "$v" in v[0-9]*) echo "$v" ;; *) fail "no release found at ${BASE_URL}" ;; esac
}

install_binary() {
  case "$(uname -s)" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) fail "unsupported OS $(uname -s); download a release from https://github.com/${REPO}/releases" ;;
  esac
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) fail "unsupported CPU $(uname -m)" ;;
  esac

  [ "$VERSION" = latest ] && VERSION=$(latest_version)
  name="be_${VERSION}_${os}_${arch}"
  url="${BASE_URL}/download/${VERSION}"
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT

  say "downloading ${name}.tar.gz"
  fetch "${url}/${name}.tar.gz" "${tmp}/${name}.tar.gz" || fail "download failed: ${url}/${name}.tar.gz"
  fetch "${url}/checksums.txt" "${tmp}/checksums.txt" || fail "download failed: ${url}/checksums.txt"

  if has cosign; then
    fetch "${url}/checksums.txt.sigstore.json" "${tmp}/checksums.txt.sigstore.json" || fail "signature download failed"
    cosign verify-blob "${tmp}/checksums.txt" \
      --bundle "${tmp}/checksums.txt.sigstore.json" \
      --certificate-identity-regexp "^https://github.com/${REPO}/\.github/workflows/release\.yml@refs/tags/v" \
      --certificate-oidc-issuer https://token.actions.githubusercontent.com >/dev/null 2>&1 \
      || fail "checksums.txt signature verification failed"
    say "signature of checksums.txt verified with cosign"
  fi

  want=$(awk -v f="${name}.tar.gz" '$2 == f { print $1 }' "${tmp}/checksums.txt")
  [ -n "$want" ] || fail "${name}.tar.gz is not listed in checksums.txt"
  got=$(sha256 "${tmp}/${name}.tar.gz")
  [ "$want" = "$got" ] || fail "checksum mismatch for ${name}.tar.gz; not installing"
  say "checksum verified"

  tar -xzf "${tmp}/${name}.tar.gz" -C "$tmp"
  mkdir -p "$INSTALL_DIR"
  cp "${tmp}/${name}/be" "${INSTALL_DIR}/be.tmp"
  chmod 0755 "${INSTALL_DIR}/be.tmp"
  mv "${INSTALL_DIR}/be.tmp" "${INSTALL_DIR}/be"
  done_msg "$INSTALL_DIR"
}

done_msg() {
  say "installed $("$1/be" --version) to $1/be"
  case ":${PATH}:" in
    *":$1:"*) say "run: be claude" ;;
    *) say "add it to your PATH, e.g.: export PATH=\"$1:\$PATH\"" ;;
  esac
}

case "$METHOD" in
  go) go_ok || fail "Go 1.21 or newer is required for BE_METHOD=go"; install_with_go ;;
  binary) install_binary ;;
  auto) if go_ok; then install_with_go; else install_binary; fi ;;
  *) fail "BE_METHOD must be auto, go or binary" ;;
esac
