package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var operationsCatalog = map[string]core.RuleMetadata{
	"operations.missing-owner":   operationsRule("operations.missing-owner", "warn", "Missing service owner", "Warns when critical production code has no CODEOWNERS, service catalog, or configured ownership metadata.", "Add CODEOWNERS or service metadata that maps the service or path to an accountable team."),
	"operations.missing-runbook": operationsRule("operations.missing-runbook", "warn", "Missing runbook", "Warns when critical systems lack runbook or operations documentation evidence.", "Add a runbook link or local runbook covering deploy verification, common failures, escalation, and rollback."),
}

func operationsRule(id string, level string, title string, description string, howToFix string) core.RuleMetadata {
	return core.RuleMetadata{
		ID:             id,
		Section:        "Operations",
		DefaultLevel:   level,
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(
			core.RuleLanguageGo,
			core.RuleLanguageTypeScript,
			core.RuleLanguageJavaScript,
			core.RuleLanguagePython,
			core.RuleLanguageCPP,
		),
		Title:       title,
		Description: description,
		HowToFix:    howToFix,
	}
}
