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
  uses: devr-tools/codeguard@v0.2.0
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
  uses: devr-tools/codeguard@v0.2.0
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

The packs are script-first on purpose:

- pre-write hooks call `codeguard validate-patch`
- post-edit hooks call `codeguard scan -mode diff`
- the Cursor pack also includes an MCP server example for `codeguard serve --mcp`

Start with:

```bash
export CODEGUARD_CONFIG=codeguard.yaml
export CODEGUARD_PROFILE=ai-safe
export CODEGUARD_BASE_REF=origin/main
```

Then wire the scripts into your host's hook or workflow settings.

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
