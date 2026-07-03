#!/usr/bin/env bash
#
# Download the GoReleaser archives for a release tag and extract each
# `codeguard` binary into a stable staging layout that the npm and PyPI
# builders consume:
#
#   <staging>/<goos>_<goarch>/codeguard
#
# Usage: extract-binaries.sh <tag> <version> [staging_dir]
#
#   tag         release tag, e.g. v0.7.0 or codeguard-v0.7.0
#   version     bare semver used in asset names, e.g. 0.7.0
#   staging_dir output dir (default: packaging/.staging)
#
# Requires: gh (authenticated), tar.
set -euo pipefail

tag="${1:?tag required}"
version="${2:?version required}"
staging="${3:-"$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/.staging"}"

# goos goarch -> archive suffix. Keep in sync with .goreleaser.yaml build matrix.
# codeguard builds darwin/linux on amd64/arm64 only (no windows), all tar.gz.
targets=(
  "darwin amd64 tar.gz"
  "darwin arm64 tar.gz"
  "linux amd64 tar.gz"
  "linux arm64 tar.gz"
)

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

rm -rf "$staging"
mkdir -p "$staging"

for target in "${targets[@]}"; do
  read -r goos goarch ext <<<"$target"
  # Matches .goreleaser.yaml name_template:
  #   "{{ .ProjectName }}_v{{ .Version }}_{{ .Os }}_{{ .Arch }}"
  asset="codeguard_v${version}_${goos}_${goarch}.${ext}"
  dest="$staging/${goos}_${goarch}"
  mkdir -p "$dest"

  echo "==> $asset"
  gh release download "$tag" --repo devr-tools/codeguard --pattern "$asset" --dir "$workdir" --clobber

  tar -xzf "$workdir/$asset" -C "$dest" codeguard
  chmod +x "$dest/codeguard"
done

echo "Binaries staged in $staging"
