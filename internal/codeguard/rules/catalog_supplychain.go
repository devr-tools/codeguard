package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var supplyChainCatalog = map[string]core.RuleMetadata{
	"supply_chain.unpinned-dependency": {
		ID:               "supply_chain.unpinned-dependency",
		Section:          "Supply Chain",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Unpinned dependency",
		Description:      "Warns when a dependency declaration does not pin to a concrete version or digest.",
		HowToFix:         "Pin the dependency to a reviewed version or digest and commit the corresponding lockfile update.",
	},
	"supply_chain.missing-lockfile": {
		ID:               "supply_chain.missing-lockfile",
		Section:          "Supply Chain",
		DefaultLevel:     "fail",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Missing lockfile",
		Description:      "Fails when a supported package manifest is committed without its expected lockfile.",
		HowToFix:         "Generate and commit the lockfile that matches the manifest before merging the change.",
	},
	"supply_chain.lockfile-drift": {
		ID:               "supply_chain.lockfile-drift",
		Section:          "Supply Chain",
		DefaultLevel:     "fail",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Lockfile drift",
		Description:      "Fails when a manifest change is not reflected in the associated lockfile.",
		HowToFix:         "Regenerate the lockfile from the updated manifest and commit both files together.",
	},
	"supply_chain.denied-license": {
		ID:               "supply_chain.denied-license",
		Section:          "Supply Chain",
		DefaultLevel:     "fail",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Denied dependency license",
		Description:      "Fails when a dependency resolves to a license that violates configured policy.",
		HowToFix:         "Replace or upgrade the dependency, or adjust the license policy if the exception is intentional and approved.",
	},
}
