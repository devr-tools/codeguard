#!/usr/bin/env bash
#
# ONE-TIME bootstrap for npm trusted publishing.
#
# npm requires a package to already exist before you can configure a trusted
# publisher for it. This script publishes all five packages once from your
# machine using your local npm auth (run `npm login`, or set an npm token in
# ~/.npmrc, first). After it finishes, configure a trusted publisher for each
# package on npmjs.com (see the printed instructions); from then on the CI
# `publish-npm` job publishes automatically via OIDC — no token.
#
# Usage: bootstrap-publish.sh <tag> <version>
#   e.g. bootstrap-publish.sh v0.7.0 0.7.0
set -euo pipefail

tag="${1:?tag required, e.g. v0.7.0}"
version="${2:?version required, e.g. 0.7.0}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"$here/../extract-binaries.sh" "$tag" "$version"
"$here/build.sh" "$version"

# Platform packages first so the launcher's optional deps resolve on install.
for dir in "$here"/dist/codeguard-*/package "$here"/dist/codeguard/package; do
  name="$(node -p "require('$dir/package.json').name")"
  echo "==> publishing $name@$version"
  npm publish "$dir" --access public
done

cat <<'EOF'

Bootstrap publish complete.

Next: on npmjs.com, open each package's Settings -> Trusted Publisher and add:
  Publisher:         GitHub Actions
  Organization/user: devr-tools
  Repository:        codeguard
  Workflow filename: cd.yml
  Environment:       (leave blank)
  Allowed actions:   npm publish

Use cd.yml (NOT release.yml): the publish-npm job lives in the reusable
release.yml, but npm/PyPI trusted publishing matches the *top-level calling*
workflow, which is cd.yml (release-please -> stable-release -> release.yml).

Packages to configure:
  @devr-tools/codeguard
  @devr-tools/codeguard-darwin-x64
  @devr-tools/codeguard-darwin-arm64
  @devr-tools/codeguard-linux-x64
  @devr-tools/codeguard-linux-arm64

After that, remove NPM_TOKEN (if set) — CI publishes via OIDC.
EOF
