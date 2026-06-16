#!/bin/sh

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=../lib/codeguard-hook-lib.sh
. "$SCRIPT_DIR/../lib/codeguard-hook-lib.sh"

if ! scan_repo_diff; then
	echo "codeguard hook: diff scan failed after edit" >&2
	print_explain_hint
	exit 1
fi

echo "codeguard hook: diff scan passed" >&2
