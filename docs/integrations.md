# Integrations

## Pre-commit

`codeguard` ships a `.pre-commit-hooks.yaml` file so repositories can install it directly:

```yaml
repos:
  - repo: https://github.com/devr-tools/codeguard
    rev: v0.2.0
    hooks:
      - id: codeguard
        args: ["-config", "codeguard.yaml", "-profile", "startup"]
```

The packaged hook runs `codeguard scan -mode diff -base-ref HEAD` by default.

## GitHub Action

This repository also ships the GitHub Marketplace action `Devr Codeguard` from `action.yml`:

```yaml
- name: Devr Codeguard
  uses: devr-tools/codeguard@v1.1.1
  with:
    config: codeguard.yaml
    profile: strict
    mode: diff
    base-ref: origin/main
    format: github
```

The action installs `github.com/devr-tools/codeguard/cmd/codeguard` and runs `codeguard scan`.

For pull request workflows, the action can also publish a sticky fix-oriented comment:

```yaml
- name: Devr Codeguard
  uses: devr-tools/codeguard@v1.1.1
  with:
    config: codeguard.yaml
    mode: diff
    base-ref: origin/main
    format: github
    comment-fix-mode: sticky
```

`format: github` preserves workflow annotations. `comment-fix-mode: sticky` adds or updates a PR comment with concrete fix suggestions derived from the same findings.

## Agent Hook Packs

`codeguard` now ships example hook packs under [examples/hooks](/Users/alex/Documents/GitHub/codeguard/examples/hooks/README.md:1).

Included packs:

- Claude Code
  - [examples/hooks/claude-code/pre-tool-use.sh](/Users/alex/Documents/GitHub/codeguard/examples/hooks/claude-code/pre-tool-use.sh:1)
  - [examples/hooks/claude-code/post-edit.sh](/Users/alex/Documents/GitHub/codeguard/examples/hooks/claude-code/post-edit.sh:1)
- Cursor
  - [examples/hooks/cursor/before-apply.sh](/Users/alex/Documents/GitHub/codeguard/examples/hooks/cursor/before-apply.sh:1)
  - [examples/hooks/cursor/after-edit.sh](/Users/alex/Documents/GitHub/codeguard/examples/hooks/cursor/after-edit.sh:1)
  - [examples/hooks/cursor/mcp.json.example](/Users/alex/Documents/GitHub/codeguard/examples/hooks/cursor/mcp.json.example:1)
- Devin
  - [examples/hooks/devin/mcp-http.json.example](/Users/alex/Documents/GitHub/codeguard/examples/hooks/devin/mcp-http.json.example:1)
  - [examples/hooks/devin/mcp-stdio.json.example](/Users/alex/Documents/GitHub/codeguard/examples/hooks/devin/mcp-stdio.json.example:1)
  - [examples/hooks/devin/run-http.sh](/Users/alex/Documents/GitHub/codeguard/examples/hooks/devin/run-http.sh:1)
  - [examples/hooks/devin/setup-snapshot.sh](/Users/alex/Documents/GitHub/codeguard/examples/hooks/devin/setup-snapshot.sh:1)

The packs are script-first on purpose:

- pre-write hooks call `codeguard validate-patch`
- post-edit hooks call `codeguard scan -mode diff`
- the Cursor pack also includes an MCP server example for `codeguard serve --mcp`
- the Devin pack provides HTTP and stdio MCP configs plus host scripts (see below)

Start with:

```bash
export CODEGUARD_CONFIG=codeguard.yaml
export CODEGUARD_PROFILE=ai-safe
export CODEGUARD_BASE_REF=origin/main
```

Then wire the scripts into your host's hook or workflow settings.

## MCP Server

`codeguard serve --mcp` runs an MCP server (hand-rolled JSON-RPC 2.0, no extra
dependencies). It advertises `tools`, `resources`, `prompts`, and `logging`
capabilities and negotiates protocol versions `2025-11-25` (current) and
`2025-06-18` (compatibility).

### Tools, resources, prompts

- Tools: `scan`, `validate_patch`, `validate_config`, `explain`, `list_rules`,
  `verify_fix`, `propose_fix` (read-only), and `apply_fix` (destructive — writes
  the working tree). `scan` streams a progress notification per check section.
- Resources: `codeguard://rules` (rule catalog), `codeguard://config` (active
  configuration), and the `codeguard://rules/{rule_id}` template (per-rule
  explanation).
- Prompts: `review-diff`, `triage-findings`, `explain-rule`.

### Verified auto-fix

- `verify_fix` takes a caller-supplied candidate `diff` plus the `finding` it
  addresses, applies it in an isolated workspace, re-scans the changed lines,
  runs the nearest inferred tests, and returns the result only if it passes.
- `propose_fix` generates the candidate first — via the client's LLM through MCP
  `sampling` when the client supports it (no API key needed), otherwise a
  configured AI provider — then runs the same verification.
- `apply_fix` verifies and, only on success, writes the diff to the working
  tree; when the client supports `elicitation` it asks the user to confirm
  first. It is the one destructive tool.
- All fail closed. Verification test execution is trust-gated, so run the
  server with `CODEGUARD_ALLOW_CONFIG_COMMANDS=1` to let the inferred tests run;
  otherwise verification fails closed. On failure, `verify_fix`/`propose_fix`
  return `structuredContent` with the attempted diff and remaining findings.

### Server→client requests (sampling, roots, elicitation)

The server consumes the client's `sampling`, `roots`, and `elicitation`
capabilities when the client advertises them at `initialize`. These are
server-initiated JSON-RPC requests: over stdio they interleave on the existing
streams; over Streamable HTTP the client must open the `GET {mcp-path}` SSE
stream (using the `Mcp-Session-Id` returned by `initialize`) over which the
server sends `sampling/createMessage`, `roots/list`, and `elicitation/create`,
and answer them on subsequent POSTs. Advertised `roots` widen the allowed set
for `config_path` confinement and are cached per connection (invalidated on
`notifications/roots/list_changed`); `elicitation` is used by `apply_fix` to
confirm before writing.

### Security

A caller-supplied `config_path` is confined to the server's config directory,
the working directory, and any client-advertised roots; paths outside are
rejected with a generic error so the HTTP transport is not a filesystem oracle.

### Transports

**stdio (default)** — for local subprocess hosts (Claude Code, Cursor):

```bash
codeguard serve --mcp -config codeguard.yaml -profile ai-safe
```

**Streamable HTTP** — for remote / cloud-hosted hosts (e.g. Devin):

```bash
codeguard serve --mcp --http \
  --addr 0.0.0.0:8080 \
  --mcp-path /mcp \
  --auth-token "$CODEGUARD_MCP_AUTH_TOKEN" \
  --auth-header Authorization \
  -config codeguard.yaml -profile ai-safe
```

The HTTP endpoint:

- accepts JSON-RPC over `POST {mcp-path}`; `tools/call` streams progress as
  `text/event-stream` (SSE), other methods return a single `application/json`
  response, and notifications return `202`.
- enforces an optional static bearer token (constant-time compared); a blank
  token disables auth and should only be used behind a private network. The
  token also falls back to `$CODEGUARD_MCP_AUTH_TOKEN`.
- exposes `GET /healthz` for health checks, caps request body size, limits
  concurrent tool executions, and drains in-flight requests on SIGINT/SIGTERM.
- mints an `Mcp-Session-Id` on `initialize`; `GET {mcp-path}` returns `405`
  (no server-initiated stream), `DELETE` acknowledges session termination.

### Devin

Devin connects to a custom MCP server over HTTP (recommended), SSE, or stdio.
See the [Devin pack README](/Users/alex/Documents/GitHub/codeguard/examples/hooks/devin/README.md:1)
for the full walkthrough. In short:

1. Host the server with `examples/hooks/devin/run-http.sh` behind a reachable
   URL (TLS/ingress/tunnel).
2. In Devin → Settings → MCP Marketplace → Add a custom MCP: Transport `HTTP`,
   Server URL `https://…/mcp`, Authentication `Auth Header` →
   `Authorization: Bearer <token>`.
3. Click **Test listing tools** — Devin runs `initialize` + `tools/list` and
   should discover the five tools.

For a binary-in-snapshot setup instead, use `setup-snapshot.sh` and the stdio
config in `mcp-stdio.json.example`.

## MCP Smoke Harness

`codeguard serve --mcp` is covered by a host-shaped smoke harness in `tests/mcp/testdata/transcripts/` and `tests/mcp/smoke_test.go`.

The harness launches the local MCP server, replays NDJSON transcripts that model real host request flows, and validates the returned JSON-RPC/MCP responses. The current profiles target:

- `editor-current`
- `editor-compat`
- `review-agent`
- `scan-agent`
- `resource-agent`
- `prompt-agent`
- `streaming-agent` (per-section progress)
- `verify-fix-agent` (verified fix fail-closed)

Bidirectional server→client flows (sampling, roots) and the HTTP transport are
covered by `tests/mcp/sampling_test.go` and `tests/mcp/http_test.go`, which act
as an MCP client and answer the server's `sampling/createMessage` and
`roots/list` requests.

Run it with:

```bash
GOROOT=/opt/homebrew/opt/go/libexec GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/mcp -run TestMCPHostSmokeProfiles
```

Current scope:

- validates host-like `initialize`, `tools/list`, `tools/call`, `ping`, config/patch/explain, and `resources`/`prompts` flows
- validates the server's newline-delimited stdio JSON-RPC transport
- the Streamable-HTTP transport is covered by `internal/cli/mcp_http_test.go` (initialize, auth, SSE tool calls, resources/prompts, health)

Out of scope:

- automating the actual desktop/editor hosts themselves
- `Content-Length` framed stdio compatibility
- OAuth-based HTTP authentication (static bearer only)
