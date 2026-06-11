# Claude Code Hook Pack

This pack gives Claude Code a lightweight policy gate before disk writes and a diff scan after edits.

Files:

- `pre-tool-use.sh`
  - validates a proposed unified diff with `codeguard validate-patch`
- `post-edit.sh`
  - scans the repository diff with `codeguard scan -mode diff`

Suggested environment:

```bash
export CODEGUARD_CONFIG=codeguard.yaml
export CODEGUARD_PROFILE=ai-safe
export CODEGUARD_BASE_REF=origin/main
```

Expected hook inputs:

- `pre-tool-use.sh`
  - accepts a patch-file path as its first argument
  - if no path is provided, reads a unified diff from stdin
- `post-edit.sh`
  - does not require arguments

Suggested wiring:

1. Register `pre-tool-use.sh` for write-producing tools such as edit, multi-edit, or patch application.
2. Register `post-edit.sh` after successful file edits.
3. Keep `codeguard serve --mcp` available separately so the agent can also call `scan`, `validate_patch`, and `explain` directly when the host supports MCP.

Behavior:

- on patch-policy failure, the pre-tool hook exits non-zero and prints the `validate-patch` findings
- on post-edit failure, the after-edit hook exits non-zero and prints diff-scan findings
- both hooks stay CLI-only and do not mutate the repository
