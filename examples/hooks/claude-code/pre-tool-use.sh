#!/bin/sh

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=../lib/codeguard-hook-lib.sh
. "$SCRIPT_DIR/../lib/codeguard-hook-lib.sh"

patch_file=$(ensure_patch_file "${1:-}")

if ! validate_patch_file "$patch_file"; then
	echo "codeguard hook: patch rejected before tool execution" >&2
	print_explain_hint
	exit 1
fi

echo "codeguard hook: patch policy passed" >&2
