#!/bin/sh
# Tests install.sh against a local copy of a release (make install-test).
# Builds a release for this machine, serves it over HTTP on 127.0.0.1 and
# checks: a normal install, a tampered archive, and invalid input.
set -eu

cd "$(dirname "$0")/../.."
VERSION=v9.9.9
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in x86_64 | amd64) arch=amd64 ;; *) arch=arm64 ;; esac

work=$(mktemp -d)
server=""
trap 'kill "$server" 2>/dev/null || true; rm -rf "$work"' EXIT
fails=0
ok() { printf 'ok   %s\n' "$1"; }
bad() { printf 'FAIL %s\n' "$1"; fails=$((fails + 1)); }

make -s dist VERSION="$VERSION" PLATFORMS="$os/$arch" >/dev/null
mkdir -p "$work/site/download/$VERSION"
cp dist/*.tar.gz dist/checksums.txt "$work/site/download/$VERSION/"

port=$((20000 + $$ % 20000))
python3 -m http.server "$port" --bind 127.0.0.1 --directory "$work/site" >/dev/null 2>&1 &
server=$!
sleep 1
base="http://127.0.0.1:$port"

run() { # run <install dir> [args...]
  dir=$1
  shift
  BE_METHOD=binary BE_BASE_URL="$base" BE_INSTALL_DIR="$dir" sh install.sh "$@"
}

# 1. A normal install puts a working binary in the install dir.
if run "$work/bin" "$VERSION" 2>"$work/log" && [ "$("$work/bin/be" --version)" = "be $VERSION" ]; then
  ok "installs $VERSION and the binary runs"
else
  bad "install: $(cat "$work/log")"
fi
grep -q "checksum verified" "$work/log" && ok "verifies the checksum" || bad "no checksum message"

# 2. A tampered archive is rejected and nothing is installed.
printf 'x' >>"$work/site/download/$VERSION/be_${VERSION}_${os}_${arch}.tar.gz"
if run "$work/bin2" "$VERSION" 2>"$work/log"; then
  bad "tampered archive was installed"
elif grep -q "checksum mismatch" "$work/log" && [ ! -e "$work/bin2/be" ]; then
  ok "rejects a tampered archive"
else
  bad "tampered archive: $(cat "$work/log")"
fi

# 3. Invalid input fails early.
run "$work/bin3" "1.0" 2>"$work/log" && bad "accepted version 1.0" || ok "rejects an invalid version"
run "$work/bin3" "v0.0.1" 2>"$work/log" && bad "installed a missing version" || ok "fails on a missing release"
BE_METHOD=binary BE_BASE_URL="http://example.com" BE_INSTALL_DIR="$work/bin4" sh install.sh "$VERSION" 2>"$work/log" \
  && bad "downloaded over plain HTTP" \
  || { grep -q "insecure URL" "$work/log" && ok "refuses plain HTTP" || bad "http: $(cat "$work/log")"; }
BE_METHOD=nope sh install.sh 2>/dev/null && bad "accepted BE_METHOD=nope" || ok "rejects an unknown method"

[ "$fails" = 0 ] && echo "all install checks passed" || { echo "$fails install check(s) failed"; exit 1; }
