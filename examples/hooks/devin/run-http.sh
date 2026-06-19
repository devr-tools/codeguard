#!/bin/sh

# Launch the codeguard MCP server over Streamable HTTP for Devin (Option A).
#
# Environment:
#   CODEGUARD_BIN              override the codeguard binary (default: codeguard)
#   CODEGUARD_MCP_ADDR         listen address          (default: 0.0.0.0:8080)
#   CODEGUARD_MCP_PATH         MCP endpoint path        (default: /mcp)
#   CODEGUARD_MCP_AUTH_TOKEN   static bearer token; if set, required on requests
#   CODEGUARD_MCP_AUTH_HEADER  header carrying the token (default: Authorization)
#   CODEGUARD_CONFIG           policy config path (passed as -config)
#   CODEGUARD_PROFILE          policy profile     (passed as -profile)

set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=../lib/codeguard-hook-lib.sh
. "$SCRIPT_DIR/../lib/codeguard-hook-lib.sh"

require_codeguard

addr="${CODEGUARD_MCP_ADDR:-0.0.0.0:8080}"
path="${CODEGUARD_MCP_PATH:-/mcp}"
auth_header="${CODEGUARD_MCP_AUTH_HEADER:-Authorization}"

if [ -z "${CODEGUARD_MCP_AUTH_TOKEN:-}" ]; then
	echo "run-http.sh: warning: CODEGUARD_MCP_AUTH_TOKEN is unset; the endpoint will accept unauthenticated requests" >&2
fi

# shellcheck disable=SC2086
exec "$(codeguard_bin)" serve --mcp --http \
	--addr "$addr" \
	--mcp-path "$path" \
	--auth-token "${CODEGUARD_MCP_AUTH_TOKEN:-}" \
	--auth-header "$auth_header" \
	$(codeguard_args)
