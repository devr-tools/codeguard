package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var contextCatalog = map[string]core.RuleMetadata{
	"context.agent-docs-missing": {
		ID:               "context.agent-docs-missing",
		Section:          "Agent Context",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Missing agent instructions",
		Description:      "Warns when the repository root has no agent instruction file (CLAUDE.md, AGENTS.md, .cursorrules, or .github/copilot-instructions.md), leaving AI agents without documented conventions.",
		HowToFix:         "Add a CLAUDE.md or AGENTS.md at the repository root describing how to build, test, and navigate the codebase, plus any conventions an agent must follow.",
	},
	"context.agent-docs-drift": {
		ID:               "context.agent-docs-drift",
		Section:          "Agent Context",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Agent instructions drift",
		Description:      "Warns when an agent instruction file references a path, make target, or npm script that provably no longer exists, so agents inherit stale instructions.",
		HowToFix:         "Update the agent instruction file to match the current repository layout and commands, or restore the file, target, or script it references.",
	},
	"context.readme-drift": {
		ID:               "context.readme-drift",
		Section:          "Agent Context",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "README command drift",
		Description:      "Warns when a fenced shell block in the root README invokes a script, make target, or repo-relative path that provably does not exist.",
		HowToFix:         "Update the README's command examples to the current entrypoints, or restore the script, target, or path they invoke.",
	},
	"context.oversized-context-unit": {
		ID:               "context.oversized-context-unit",
		Section:          "Agent Context",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Oversized context unit",
		Description:      "Warns when a source file exceeds the agent context budget (context_rules.max_file_lines, default 1500): a file that large crowds out the rest of an AI agent's working context.",
		HowToFix:         "Split the file into smaller, focused units so an agent can load only the part relevant to its task; extract cohesive sections into their own files.",
	},
	"context.ambiguous-symbol": {
		ID:               "context.ambiguous-symbol",
		Section:          "Agent Context",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Ambiguous file basename",
		Description:      "Warns when the same source-file basename appears in many directories (context_rules.ambiguous_symbol_threshold, default 4), defeating filename search and grep-based navigation for AI agents.",
		HowToFix:         "Rename the duplicated files to distinct, descriptive names that encode their module or role (for example user_routes.ts instead of a fourth routes.ts).",
	},
}
