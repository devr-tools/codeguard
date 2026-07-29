package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var observabilityCatalog = map[string]core.RuleMetadata{
	"observability.unstructured-log":             observabilityRule("observability.unstructured-log", "warn", "Unstructured log", "Warns when production code writes raw logs without structured context fields.", "Use the repository's structured logger and include stable operation fields instead of raw console/print output."),
	"observability.error-without-context":        observabilityRule("observability.error-without-context", "warn", "Error log without context", "Warns when an error is logged without operation, request, or safe business context.", "Log the operation name and safe request or resource identifiers; avoid logging secrets or raw payloads."),
	"observability.sensitive-log-data":           observabilityRule("observability.sensitive-log-data", "fail", "Sensitive log data", "Fails when log calls include sensitive names such as tokens, passwords, authorization headers, cookies, or PII-like fields.", "Remove the value from logs, hash/redact it, or replace it with a non-sensitive correlation identifier."),
	"observability.high-cardinality-label":       observabilityRule("observability.high-cardinality-label", "warn", "High-cardinality metric label", "Warns when metric labels use identifiers or raw paths that can explode cardinality.", "Use bounded labels such as route templates, operation names, status classes, or stable enum-like dimensions."),
	"observability.critical-path-uninstrumented": observabilityRule("observability.critical-path-uninstrumented", "warn", "Critical path without instrumentation", "Warns when handlers, consumers, jobs, or other critical paths lack visible logging, metrics, or tracing evidence.", "Add a span, metric, or structured log at the critical path boundary and record failures with safe context."),
	"observability.log-and-ignore":               observabilityRule("observability.log-and-ignore", "warn", "Logged and ignored failure", "Warns when code logs a failure and then continues or returns success without surfacing the error.", "Return or aggregate the error, mark the operation as partially failed, or document why the failure is safely ignorable."),
	"observability.shallow-health-check":         observabilityRule("observability.shallow-health-check", "warn", "Shallow health check", "Warns when a health/readiness endpoint returns a static OK response while dependency evidence exists nearby.", "Make readiness check critical dependencies or split liveness from readiness so deploys verify meaningful service health."),
}

func observabilityRule(id string, level string, title string, description string, howToFix string) core.RuleMetadata {
	return core.RuleMetadata{
		ID:             id,
		Section:        "Observability",
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
