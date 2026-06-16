#!/bin/sh

set -eu

codeguard_bin() {
	if [ -n "${CODEGUARD_BIN:-}" ]; then
		printf '%s\n' "$CODEGUARD_BIN"
		return
	fi
	printf '%s\n' "codeguard"
}

require_codeguard() {
	if ! command -v "$(codeguard_bin)" >/dev/null 2>&1; then
		echo "codeguard hook: unable to find $(codeguard_bin) in PATH" >&2
		exit 127
	fi
}

codeguard_args() {
	if [ -n "${CODEGUARD_CONFIG:-}" ]; then
		printf -- ' -config %s' "$CODEGUARD_CONFIG"
	fi
	if [ -n "${CODEGUARD_PROFILE:-}" ]; then
		printf -- ' -profile %s' "$CODEGUARD_PROFILE"
	fi
}

default_base_ref() {
	if [ -n "${CODEGUARD_BASE_REF:-}" ]; then
		printf '%s\n' "$CODEGUARD_BASE_REF"
		return
	fi
	if git rev-parse --verify origin/main >/dev/null 2>&1; then
		printf '%s\n' "origin/main"
		return
	fi
	if git rev-parse --verify main >/dev/null 2>&1; then
		printf '%s\n' "main"
		return
	fi
	printf '%s\n' "HEAD~1"
}

ensure_patch_file() {
	if [ $# -gt 0 ] && [ -n "$1" ] && [ -f "$1" ]; then
		printf '%s\n' "$1"
		return
	fi

	tmp="${TMPDIR:-/tmp}/codeguard-hook-patch-$$.diff"
	cat >"$tmp"
	printf '%s\n' "$tmp"
}

validate_patch_file() {
	patch_file="$1"
	format="${CODEGUARD_PATCH_FORMAT:-json}"
	require_codeguard
	# shellcheck disable=SC2086
	"$(codeguard_bin)" validate-patch $(codeguard_args) -format "$format" <"$patch_file"
}

scan_repo_diff() {
	base_ref="$(default_base_ref)"
	format="${CODEGUARD_SCAN_FORMAT:-text}"
	require_codeguard
	# shellcheck disable=SC2086
	"$(codeguard_bin)" scan $(codeguard_args) -mode diff -base-ref "$base_ref" -format "$format"
}

print_explain_hint() {
	echo "Hint: inspect a rule with 'codeguard explain -format agent <rule-id>'." >&2
}
