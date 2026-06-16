# Agent Hook Packs

This directory ships reusable `codeguard` hook assets for editor-hosted agents.

Current packs:

- `claude-code/`
  - `pre-tool-use.sh`
  - `post-edit.sh`
- `cursor/`
  - `before-apply.sh`
  - `after-edit.sh`
  - `mcp.json.example`
- `lib/`
  - shared shell helpers used by both packs

Each script is designed to work with the existing `codeguard` CLI:

- `codeguard validate-patch`
- `codeguard scan -mode diff`
- `codeguard serve --mcp`
- `codeguard explain -format agent`

See the per-pack READMEs for install examples and expected environment variables.
