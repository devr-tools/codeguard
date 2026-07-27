package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var deliveryCatalog = map[string]core.RuleMetadata{
	"delivery.missing-rollback-strategy": {
		ID:               "delivery.missing-rollback-strategy",
		Section:          "Delivery",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Missing rollback strategy",
		Description:      "Warns when deployment or destructive migration evidence appears without rollback, revert, or restore instructions.",
		HowToFix:         "Document or automate rollback steps next to the rollout workflow, migration, or runbook before shipping the change.",
	},
	"delivery.unsafe-migration-order": {
		ID:               "delivery.unsafe-migration-order",
		Section:          "Delivery",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Unsafe migration order",
		Description:      "Warns when a destructive migration lacks expand/backfill/contract, concurrent, or rollback sequencing evidence.",
		HowToFix:         "Split the migration into compatible phases, backfill safely, verify readers, then remove old schema in a later rollout.",
	},
	"delivery.high-risk-change-without-kill-switch": {
		ID:               "delivery.high-risk-change-without-kill-switch",
		Section:          "Delivery",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(core.RuleLanguageGo, core.RuleLanguagePython, core.RuleLanguageTypeScript, core.RuleLanguageJavaScript, core.RuleLanguageCPP),
		Title:            "High-risk change without kill switch",
		Description:      "Warns when critical payment, auth, checkout, billing, or migration behavior changes without feature flag or kill-switch evidence.",
		HowToFix:         "Gate the high-risk behavior behind a feature flag or operational kill switch and document the rollback path.",
	},
	"delivery.missing-post-deploy-verification": {
		ID:               "delivery.missing-post-deploy-verification",
		Section:          "Delivery",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Missing post-deploy verification",
		Description:      "Warns when a deployment workflow has no smoke, health, synthetic, SLO, or equivalent verification evidence after rollout.",
		HowToFix:         "Add a post-deploy smoke test, health check, synthetic probe, or SLO verification step to the deployment workflow.",
	},
}
