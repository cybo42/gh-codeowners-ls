#!/usr/bin/env bash
# Cross-compiles gh-codeowners-ls into dist/ for every platform that
# `gh extension install` can consume. Invoked by `make dist` and by CI.
#
# Asset names MUST end in "{goos}-{goarch}" (plus ".exe" on Windows): that
# suffix is what `gh extension install` matches when picking an asset from
# repos/{owner}/{repo}/releases/latest. Underscore-separated names are never
# selected.
#
# The platform list and build flags mirror cli/gh-extension-precompile's
# defaults, so a green `make dist` means the release build will be green too.
# Real releases are cut by .github/workflows/release.yml, which runs that
# action's own build.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

BINARY="gh-codeowners-ls"
DIST="dist"

PLATFORMS=(
  darwin-amd64 darwin-arm64
  freebsd-386 freebsd-amd64 freebsd-arm64
  linux-386 linux-amd64 linux-arm linux-arm64
  windows-386 windows-amd64 windows-arm64
)

supported="$(go tool dist list)"

rm -rf "$DIST"
mkdir -p "$DIST"

for platform in "${PLATFORMS[@]}"; do
  os="${platform%-*}"
  arch="${platform#*-}"
  if ! grep -qx "${os}/${arch}" <<<"$supported"; then
    echo "skipping unsupported platform $platform" >&2
    continue
  fi
  ext=""
  [ "$os" = "windows" ] && ext=".exe"
  out="$DIST/${BINARY}-${platform}${ext}"
  echo "building $out"
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
    go build -trimpath -ldflags="-s -w" -o "$out" .
done
