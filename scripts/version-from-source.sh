#!/usr/bin/env bash

set -euo pipefail

version="$(awk -F'"' '/^const defaultNumber = / { print $2; exit }' internal/version/version.go)"
if [[ -z "$version" ]]; then
  echo "failed to extract compiled default from internal/version/version.go" >&2
  exit 1
fi

printf '%s\n' "$version"
