#!/usr/bin/env bash
#
# Assemble the publishable npm packages from staged binaries.
#
# Layout produced under packaging/npm/dist/:
#   @devr-tools/codeguard                 main launcher package (optionalDependencies)
#   @devr-tools/codeguard-darwin-x64      platform package (binary payload)
#   @devr-tools/codeguard-darwin-arm64
#   @devr-tools/codeguard-linux-x64
#   @devr-tools/codeguard-linux-arm64
#
# Usage: build.sh <version> [staging_dir]
set -euo pipefail

version="${1:?version required}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
staging="${2:-"$here/../.staging"}"
dist="$here/dist"

# npm-platform-key  npm-os  npm-cpu  staging-subdir
platforms=(
  "darwin-x64   darwin  x64    darwin_amd64"
  "darwin-arm64 darwin  arm64  darwin_arm64"
  "linux-x64    linux   x64    linux_amd64"
  "linux-arm64  linux   arm64  linux_arm64"
)

rm -rf "$dist"
mkdir -p "$dist"

optional_deps=""

for entry in "${platforms[@]}"; do
  read -r key npm_os npm_cpu subdir <<<"$entry"
  pkgname="@devr-tools/codeguard-${key}"
  pkgdir="$dist/codeguard-${key}/package"
  mkdir -p "$pkgdir/bin"

  src="$staging/$subdir/codeguard"
  if [ ! -f "$src" ]; then
    echo "missing staged binary: $src" >&2
    exit 1
  fi
  cp "$src" "$pkgdir/bin/codeguard"
  chmod +x "$pkgdir/bin/codeguard"

  cat >"$pkgdir/package.json" <<JSON
{
  "name": "$pkgname",
  "version": "$version",
  "description": "The $key binary for codeguard.",
  "homepage": "https://github.com/devr-tools/codeguard",
  "repository": { "type": "git", "url": "git+https://github.com/devr-tools/codeguard.git" },
  "license": "Apache-2.0",
  "os": ["$npm_os"],
  "cpu": ["$npm_cpu"],
  "files": ["bin/codeguard"]
}
JSON

  optional_deps="${optional_deps}    \"$pkgname\": \"$version\",\n"
done

# Trim trailing comma+newline from the assembled optionalDependencies block.
optional_deps="$(printf "%b" "$optional_deps" | sed '$ s/,$//')"

# Main launcher package.
maindir="$dist/codeguard/package"
mkdir -p "$maindir/bin"
cp "$here/launcher/bin/codeguard" "$maindir/bin/codeguard"
chmod +x "$maindir/bin/codeguard"
cp "$here/launcher/README.md" "$maindir/README.md"

cat >"$maindir/package.json" <<JSON
{
  "name": "@devr-tools/codeguard",
  "version": "$version",
  "description": "Repository checks across code quality, design boundaries, security, CI/CD hygiene, and AI prompt governance.",
  "homepage": "https://github.com/devr-tools/codeguard",
  "repository": { "type": "git", "url": "git+https://github.com/devr-tools/codeguard.git" },
  "license": "Apache-2.0",
  "keywords": ["cli", "linter", "static-analysis", "security", "code-quality", "ai"],
  "bin": { "codeguard": "bin/codeguard" },
  "files": ["bin/codeguard", "README.md"],
  "optionalDependencies": {
$optional_deps
  }
}
JSON

echo "npm packages built in $dist"
