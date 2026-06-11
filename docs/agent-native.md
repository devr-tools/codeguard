# Agent-Native Features

This document is a short status brief for `codeguard` features aimed at AI agents and editor-hosted tool use.

## Current status

### Implemented

- `codeguard serve --mcp`
  - exposes MCP tools for `scan`, `validate_patch`, and `explain`
  - also exposes `validate_config` and `list_rules`
  - supports `initialize`, `tools/list`, `tools/call`, `ping`, progress notifications, and cancellation
  - covered by CLI compatibility tests and host-shaped smoke tests in `tests/cli/` and `tests/mcp/`

- Patch validation API
  - `codeguard validate-patch` accepts a unified diff on stdin
  - `codeguard.RunPatch(ctx, cfg, diffText)` is available in the public Go SDK
  - validation runs against synthesized patched content and does not mutate the working tree

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

- MCP server: [internal/cli/mcp_run.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/mcp_run.go:1), [internal/cli/mcp_tools.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/mcp_tools.go:1), [internal/cli/mcp_protocol.go](/Users/alex/Documents/GitHub/codeguard/internal/cli/mcp_protocol.go:1)
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
