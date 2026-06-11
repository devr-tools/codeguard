# Integrations

## Pre-commit

`codeguard` ships a `.pre-commit-hooks.yaml` file so repositories can install it directly:

```yaml
repos:
  - repo: https://github.com/devr-tools/codeguard
    rev: v0.1.0
    hooks:
      - id: codeguard
        args: ["-config", "codeguard.yaml", "-profile", "startup"]
```

The packaged hook runs `codeguard scan -mode diff -base-ref HEAD` by default.

## GitHub Action

This repository also ships a composite action at `action.yml`:

```yaml
- name: CodeGuard
  uses: devr-tools/codeguard@v0.1.0
  with:
    config: codeguard.yaml
    profile: strict
    mode: diff
    base-ref: origin/main
    format: github
```

The action installs `github.com/devr-tools/codeguard/cmd/codeguard` and runs `codeguard scan`.

## MCP Smoke Harness

`codeguard serve --mcp` is covered by a host-shaped smoke harness in `tests/mcp/testdata/transcripts/` and `tests/mcp/smoke_test.go`.

The harness launches the local MCP server, replays NDJSON transcripts that model real host request flows, and validates the returned JSON-RPC/MCP responses. The current profiles target:

- `editor-current`
- `editor-compat`
- `review-agent`
- `scan-agent`

Run it with:

```bash
GOROOT=/opt/homebrew/opt/go/libexec GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/mcp -run TestMCPHostSmokeProfiles
```

Current scope:

- validates host-like `initialize`, `tools/list`, `tools/call`, `ping`, and config/patch/explain flows
- validates the server's current newline-delimited stdio JSON-RPC transport

Out of scope:

- automating the actual desktop/editor hosts themselves
- `Content-Length` framed stdio compatibility
- prompts/resources/task APIs
