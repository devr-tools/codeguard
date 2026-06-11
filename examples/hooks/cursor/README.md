# Cursor Hook Pack

This pack is structured for Cursor workspaces that want two layers:

- a CLI gate around patch application and post-edit scans
- an MCP server entry so the agent can query `codeguard` directly

Files:

- `before-apply.sh`
  - validates a proposed unified diff with `codeguard validate-patch`
- `after-edit.sh`
  - scans the repository diff with `codeguard scan -mode diff`
- `mcp.json.example`
  - example MCP server entry for `codeguard serve --mcp`

Suggested environment:

```bash
export CODEGUARD_CONFIG=codeguard.yaml
export CODEGUARD_PROFILE=ai-safe
export CODEGUARD_BASE_REF=origin/main
```

Expected hook inputs:

- `before-apply.sh`
  - accepts a patch-file path as its first argument
  - if no path is provided, reads a unified diff from stdin
- `after-edit.sh`
  - does not require arguments

Suggested wiring:

1. Register `before-apply.sh` anywhere your Cursor workflow can intercept an about-to-be-applied patch.
2. Register `after-edit.sh` after file writes complete.
3. Add the `mcp.json.example` entry to your workspace MCP configuration so Cursor can call `scan`, `validate_patch`, `explain`, `validate_config`, and `list_rules`.

The scripts are host-agnostic on purpose: if Cursor changes how it stores local workflow config, the actual policy logic here remains valid and reusable.
