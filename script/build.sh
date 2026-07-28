#!/usr/bin/env bash
# Cross-compiles gh-codeowners-ls for common gh extension platforms.
#
# This is invoked by `make dist`. Release CI (cli/gh-extension-precompile) is
# out of scope for now; this script exists so that integration can be added
# later without restructuring the build.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

BINARY="gh-codeowners-ls"
DIST="dist"

PLATFORMS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
)

rm -rf "$DIST"
mkdir -p "$DIST"

for platform in "${PLATFORMS[@]}"; do
  os="${platform%/*}"
  arch="${platform#*/}"
  out="$DIST/${BINARY}_${os}_${arch}"
  if [ "$os" = "windows" ]; then
    out="${out}.exe"
  fi
  echo "building $out"
  GOOS="$os" GOARCH="$arch" go build -o "$out" .
done
