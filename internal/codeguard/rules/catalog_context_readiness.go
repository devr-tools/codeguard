package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// contextReadinessCatalog covers the "AI and human ready" extension of the
// agent-context family: commands the docs never mention, agent docs too large
// for the context they manage, and markdown links that rot.
var contextReadinessCatalog = map[string]core.RuleMetadata{
	"context.undocumented-commands": {
		ID:               "context.undocumented-commands",
		Section:          "Agent Context",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Undocumented dev command",
		Description:      "Warns when a high-signal Makefile target or package.json script (build, check, dev, fmt, lint, run, start, test) is mentioned by no agent instruction file and not even the root README, so agents cannot discover the repo's canonical entrypoints. Silent when the repo has no agent docs at all.",
		HowToFix:         "Mention the command in CLAUDE.md/AGENTS.md (or the README) alongside when to use it, or remove the target/script if it is no longer part of the canonical workflow.",
	},
	"context.oversized-agent-doc": {
		ID:               "context.oversized-agent-doc",
		Section:          "Agent Context",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Oversized agent instructions",
		Description:      "Warns when an agent instruction file exceeds context_rules.max_agent_doc_lines (default 600): agent docs are loaded into every session verbatim, so an oversized one consumes the context window it exists to save.",
		HowToFix:         "Keep the agent instruction file to essentials — build/test commands, layout, conventions — and move reference material into separate docs the agent can load on demand.",
	},
	"context.doc-link-rot": {
		ID:               "context.doc-link-rot",
		Section:          "Agent Context",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Documentation link rot",
		Description:      "Warns when a markdown link in an agent instruction file or the root README points at a repository file or directory that does not exist. External URLs are never checked (no network I/O) and anchors are ignored.",
		HowToFix:         "Point the link at the file's current location, or remove the link if the document it referenced is gone.",
	},
}
