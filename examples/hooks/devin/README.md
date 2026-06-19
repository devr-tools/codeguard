# Devin Integration Pack

This pack connects [Devin](https://devin.ai) to the `codeguard` MCP server so a
Devin session can call `scan`, `validate_patch`, `explain`, `validate_config`,
and `list_rules` directly, and read the `codeguard://rules` / `codeguard://config`
resources and the `review-diff` / `triage-findings` / `explain-rule` prompts.

Devin adds custom MCP servers from **Settings → MCP Marketplace → Add a custom
MCP** (requires the *Manage MCP Servers* permission). It supports three
transports: **HTTP (Streamable HTTP, recommended)**, **SSE (legacy)**, and
**STDIO**. After saving, click **Test listing tools** — Devin runs `initialize`
+ `tools/list` against the server and should discover the five tools.

There are two ways to wire codeguard in. Pick one.

## Option A — HTTP (recommended)

Because Devin runs in the cloud, the simplest robust setup is to host the
codeguard MCP server at a URL Devin can reach and point Devin at it.

1. Host the server (see [`run-http.sh`](run-http.sh)):

   ```bash
   export CODEGUARD_MCP_AUTH_TOKEN="$(openssl rand -hex 24)"
   export CODEGUARD_CONFIG=codeguard.yaml
   export CODEGUARD_PROFILE=ai-safe
   ./run-http.sh           # listens on 0.0.0.0:8080/mcp by default
   ```

   Put it behind TLS / an ingress / a tunnel so Devin's cloud can reach it
   (e.g. `https://codeguard.your-org.dev/mcp`). The endpoint also exposes
   `GET /healthz` for load-balancer health checks.

2. In Devin, add a custom MCP server:

   - **Transport:** HTTP
   - **Server URL:** `https://codeguard.your-org.dev/mcp`
   - **Authentication:** Auth Header
     - Header key: `Authorization`
     - Header value: `Bearer <the CODEGUARD_MCP_AUTH_TOKEN value>`

   See [`mcp-http.json.example`](mcp-http.json.example) for the equivalent
   config payload. **Never commit the real token** — the examples use
   placeholders so codeguard's own prompt-governance checks don't flag them.

3. Click **Test listing tools** to confirm discovery.

> Set no auth token only if the endpoint is reachable solely over a private
> network; otherwise always set `CODEGUARD_MCP_AUTH_TOKEN`.

## Option B — STDIO (binary baked into Devin's machine)

If you prefer no hosted endpoint, install the `codeguard` binary into Devin's
machine snapshot and configure a STDIO server.

1. Add [`setup-snapshot.sh`](setup-snapshot.sh) to your Devin machine setup so
   the `codeguard` binary is on `PATH` in every session.
2. Add a custom MCP server with **Transport: STDIO** using
   [`mcp-stdio.json.example`](mcp-stdio.json.example):

   - Command: `codeguard`
   - Args: `serve --mcp -config codeguard.yaml -profile ai-safe`

The MCP server writes only valid JSON-RPC to stdout (diagnostics go to stderr),
which is what Devin's STDIO transport requires.

## Files

- `mcp-http.json.example` — custom MCP config for the HTTP transport
- `mcp-stdio.json.example` — custom MCP config for the STDIO transport
- `run-http.sh` — launch the codeguard MCP server over HTTP
- `setup-snapshot.sh` — install the codeguard binary for STDIO mode
