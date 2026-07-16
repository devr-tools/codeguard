package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// contextReadinessFixTemplates covers the AI-and-human-readiness extension of
// the agent-context family.
var contextReadinessFixTemplates = map[string]core.FixTemplate{
	"context.undocumented-commands": {Kind: guided, Text: "Name the canonical command in an agent doc (or the README) so agents can discover it.\n\nBefore:\n# CLAUDE.md\n## Build & test\n- make build\n\nAfter:\n# CLAUDE.md\n## Build & test\n- make build\n- make test — run the unit suite\n- make lint — golangci-lint, CI-blocking"},
	"context.oversized-agent-doc":   {Kind: guided, Text: "Trim the agent doc to essentials and link out to reference material.\n\nBefore:\n# CLAUDE.md (900 lines: commands, style guide, API reference, changelog)\n\nAfter:\n# CLAUDE.md (essentials only)\n## Build & test\n- make build / make test\n## Deep dives\n- docs/style-guide.md\n- docs/api-reference.md"},
	"context.doc-link-rot":          {Kind: guided, Text: "Repoint the broken link at the file's current location, or drop it.\n\nBefore:\n- [Architecture](docs/design/architecture.md)\n\nAfter:\n- [Architecture](docs/architecture.md)"},
}
