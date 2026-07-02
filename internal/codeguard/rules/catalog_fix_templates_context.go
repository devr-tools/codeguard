package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// contextFixTemplates covers the agent-context legibility rule family.
var contextFixTemplates = map[string]core.FixTemplate{
	"context.agent-docs-missing":     {Kind: guided, Text: "Create a CLAUDE.md (or AGENTS.md) at the repository root that tells an agent how to work in this repo.\n\nBefore:\n(no CLAUDE.md, AGENTS.md, .cursorrules, or .github/copilot-instructions.md)\n\nAfter:\n# CLAUDE.md\n\n## Build & test\n- make build\n- make test\n\n## Layout\n- cmd/: CLI entrypoints\n- internal/: implementation packages\n\n## Conventions\n- run make fmt before committing"},
	"context.agent-docs-drift":       {Kind: guided, Text: "Point the agent doc at things that exist: fix the path, target, or script it references, or delete the stale instruction.\n\nBefore:\n# CLAUDE.md\nRun `make deploy-all` and edit `internal/server/router.go`.\n\nAfter:\n# CLAUDE.md\nRun `make deploy` and edit `internal/api/router.go`."},
	"context.readme-drift":           {Kind: guided, Text: "Update the README's command examples to match the current scripts and make targets.\n\nBefore:\n```bash\n./scripts/setup.sh\nmake bootstrap\n```\n\nAfter:\n```bash\n./scripts/dev-setup.sh\nmake build\n```"},
	"context.oversized-context-unit": {Kind: guided, Text: "Split the file into cohesive units that each fit an agent's working context.\n\nBefore:\n// handlers.go: 2400 lines mixing auth, billing, and admin endpoints\n\nAfter:\n// handlers_auth.go, handlers_billing.go, handlers_admin.go\n// each under the configured line budget, grouped by responsibility"},
	"context.ambiguous-symbol":       {Kind: guided, Text: "Rename duplicated basenames so search results identify a file unambiguously.\n\nBefore:\napi/utils.ts, billing/utils.ts, auth/utils.ts, admin/utils.ts\n\nAfter:\napi/http_helpers.ts, billing/invoice_math.ts, auth/token_helpers.ts, admin/audit_format.ts"},
}
