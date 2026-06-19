# Agent-Native Features

This document is a short status brief for `codeguard` features aimed at AI agents and editor-hosted tool use.

## Current status

### Implemented

- `codeguard serve --mcp`
  - exposes MCP tools for `scan`, `validate_patch`, and `explain`
  - also exposes `validate_config` and `list_rules`
  - exposes verified auto-fix tools `verify_fix` (verify a caller-supplied diff), `propose_fix` (generate a fix, then verify it), and `apply_fix` (verify then write to the working tree) — see below
  - all tools carry read-only / non-destructive annotations and output schemas
  - exposes `resources`: `codeguard://rules`, `codeguard://config`, and the `codeguard://rules/{rule_id}` template
  - exposes `prompts`: `review-diff`, `triage-findings`, `explain-rule`
  - declares a `logging` capability (accepts `logging/setLevel`)
  - consumes the client's `sampling` and `roots` capabilities when advertised (server→client requests; see below)
  - `scan` streams a progress notification per check section as it completes
  - supports `initialize`, `tools/list`, `tools/call`, `ping`, progress notifications, and cancellation
  - covered by CLI compatibility tests and host-shaped smoke tests in `tests/cli/` and `tests/mcp/`

- Verified auto-fix
  - `verify_fix` applies a caller-supplied candidate diff in an isolated workspace, re-scans the changed lines, runs the nearest inferred tests, and returns the result only if it passes (fails closed)
  - `propose_fix` first generates the candidate, then runs the same verification. The generator is the client's LLM via MCP `sampling` when the client supports it (no API key needed), otherwise a configured AI provider
  - `verify_fix`/`propose_fix` wrap `service.VerifyFix` / `service.GenerateVerifiedFix`; verification test execution requires the server to run with command trust enabled (`CODEGUARD_ALLOW_CONFIG_COMMANDS=1`), otherwise it fails closed
  - `verify_fix`/`propose_fix` do not mutate the working tree; on verification failure they return `isError` with `structuredContent` (attempted diff + remaining findings) so an agent can iterate
  - `apply_fix` verifies first and only then writes the diff to the working tree (the one destructive tool); when the client supports `elicitation` it asks the user to confirm before writing

- Server→client requests (`sampling`, `roots`, `elicitation`)
  - the server issues `sampling/createMessage` (fix generation), `roots/list` (workspace folders), and `elicitation/create` (confirm before `apply_fix` writes) over both transports — on HTTP via the `GET` SSE stream and a session registry
  - client roots are cached per connection and invalidated on `notifications/roots/list_changed`
  - advertised client `roots` are added to the allowed set for caller-supplied `config_path` confinement; a caller-supplied `config_path` is always confined to the server's config dir, the working directory, and any client roots, and is rejected otherwise

- `codeguard serve --mcp --http`
  - serves the same MCP server over Streamable HTTP for remote / cloud-hosted hosts (e.g. Devin)
  - `tools/call` streams progress over SSE; other methods return a single JSON response
  - optional static bearer auth (`--auth-token` / `$CODEGUARD_MCP_AUTH_TOKEN`, `--auth-header`), `GET /healthz`, request-size and concurrency limits, and graceful shutdown
  - covered by `internal/cli/mcp_http_test.go`

- Devin integration pack
  - HTTP and stdio MCP configs plus host scripts under [examples/hooks/devin](/Users/alex/Documents/GitHub/codeguard/examples/hooks/devin/README.md:1)

- Patch validation API
  - `codeguard validate-patch` accepts a unified diff on stdin
  - `codeguard.RunPatch(ctx, cfg, diffText)` is available in the public Go SDK
  - validation runs against synthesized patched content and does not mutate the working tree

- Verified auto-fix SDK flow
  - `codeguard.VerifyFix(...)` verifies a proposed diff in an isolated workspace
  - `codeguard.GenerateVerifiedFix(ctx, req)` composes patch generation with the same verifier
  - the verifier reruns `codeguard` against the proposed diff, executes inferred nearest tests, and fails closed when tests cannot be inferred or do not pass

- Machine-first explain output
  - `codeguard explain -format agent <rule-id>` returns JSON for agent consumption
  - current fields include `id`, `title`, `section`, `level`, `execution_model`, `language_coverage`, `description`, `why`, `how_to_fix`, and `fix_template`

- Prompt governance
  - current prompt checks cover secret interpolation and unsafe instruction patterns
  - the `ai-safe` profile expands prompt discovery toward files with `agent` and `policy` naming patterns

- Hook packs for agent harnesses
  - Claude Code hook pack shipped under [examples/hooks/claude-code](/Users/alex/Documents/GitHub/codeguard/examples/hooks/claude-code/README.md:1)
  - Cursor hook and MCP pack shipped under [examples/hooks/cursor](/Users/alex/Documents/GitHub/codeguard/examples/hooks/cursor/README.md:1)
  - shared shell helpers shipped under [examples/hooks/lib](/Users/alex/Documents/GitHub/codeguard/examples/hooks/lib/codeguard-hook-lib.sh:1)
- GitHub Action comment-fix mode
  - the composite action keeps `format: github` annotation output
  - `comment-fix-mode: sticky` adds or updates a pull request comment with fix-oriented markdown generated from findings

- Expanded agent-config governance
  - `CLAUDE.md`, `AGENTS.md`, and `.cursorrules` are scanned as governed prompt assets
  - MCP config files such as `mcp.json`, `.mcp.json`, `mcp.yaml`, `mcp.yml`, and `claude_desktop_config.json` are scanned
  - current detections cover:
    - secret interpolation in agent config files
    - dangerous instructions that bypass approvals, sandboxing, or policy
    - standing wildcard permissions or effectively unrestricted tool access
    - risky MCP shell-wrapped command patterns

## File map

- MCP server core: [internal/cli/mcp_run.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/mcp_run.go:1), [internal/cli/mcp_dispatch.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/mcp_dispatch.go:1), [internal/cli/mcp_tools.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/mcp_tools.go:1), [internal/cli/mcp_protocol.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/mcp_protocol.go:1)
- MCP HTTP transport: [internal/cli/mcp_http.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/mcp_http.go:1), session/SSE registry [internal/cli/mcp_http_session.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/mcp_http_session.go:1)
- MCP resources and prompts: [internal/cli/mcp_resources.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/mcp_resources.go:1), [internal/cli/mcp_prompts.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/mcp_prompts.go:1)
- MCP verified auto-fix tools: [internal/cli/mcp_fix.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/mcp_fix.go:1)
- MCP server→client requests (sampling/roots), progress + path confinement: [internal/cli/mcp_client.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/mcp_client.go:1)
- Devin pack: [examples/hooks/devin/README.md](/Users/alex/Documents/GitHub/codeguard/examples/hooks/devin/README.md:1)
- Patch validation CLI: [internal/cli/commands.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/commands.go:101)
- Patch validation SDK: [pkg/codeguard/sdk_run.go](/Users/alex/Documents/GitHub/codeguard/pkg/codeguard/sdk_run.go:29)
- Agent explain output: [internal/cli/info.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/info.go:33)
- Prompt checks: [internal/codeguard/checks/prompts/prompts.go](/Users/alex/Documents/GitHub/codeguard/internal/codeguard/checks/prompts/prompts.go:1)
- Integration docs: [docs/integrations.md](/Users/alex/Documents/GitHub/codeguard/docs/integrations.md:1)
- Hook packs: [examples/hooks/README.md](/Users/alex/Documents/GitHub/codeguard/examples/hooks/README.md:1)
- GitHub Action comment publisher: [cmd/codeguard-action-comment/main.go](/Users/alex/Documents/GitHub/codeguard/cmd/codeguard-action-comment/main.go:1), [internal/githubaction/comment_client.go](/Users/alex/Documents/GitHub/codeguard/internal/githubaction/comment_client.go:1)

## Recommended next work

1. Broaden agent-config governance from pattern matching into richer contradictory-instruction and permission-model analysis.
2. Add end-to-end CI coverage for the composite action flow, not just unit coverage for the comment publisher and markdown formatter.
3. Add OAuth resource-server support to the HTTP transport (currently static bearer only) if hosts require Devin's OAuth mode.
