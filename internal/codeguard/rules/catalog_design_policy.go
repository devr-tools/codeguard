package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var designPolicyCatalog = map[string]core.RuleMetadata{
	"design.unreachable-module": {
		ID:               "design.unreachable-module",
		Section:          "Design Patterns",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: designGraphLanguageCoverage(),
		Title:            "Module unreachable from approved entrypoints",
		Description:      "Warns when a production module cannot be reached from any configured application or package entrypoint.",
		HowToFix:         "Remove dead code, add the intended entrypoint, or connect the module through the application dependency graph.",
	},
	"design.stability-direction": {
		ID:               "design.stability-direction",
		Section:          "Design Patterns",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: designGraphLanguageCoverage(),
		Title:            "Stable module depends on volatile module",
		Description:      "Warns when a widely depended-on stable module imports a substantially less stable module, reversing the desired dependency direction.",
		HowToFix:         "Invert the dependency through a stable contract or move volatile behavior behind an adapter owned by the less stable module.",
	},
}

func designGraphLanguageCoverage() core.RuleLanguageCoverage {
	return core.FixedRuleLanguageCoverage(
		core.RuleLanguageGo,
		core.RuleLanguagePython,
		core.RuleLanguageTypeScript,
		core.RuleLanguageJavaScript,
		core.RuleLanguageRust,
		core.RuleLanguageJava,
		core.RuleLanguageCPP,
	)
}
