package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var contractsCatalog = map[string]core.RuleMetadata{
	"contracts.go-exported-breaking": {
		ID:               "contracts.go-exported-breaking",
		Section:          "API Contracts",
		DefaultLevel:     "fail",
		ExecutionModel:   core.RuleExecutionModelGoNative,
		LanguageCoverage: core.FixedRuleLanguageCoverage(core.RuleLanguageGo),
		Title:            "Go exported API breaking change",
		Description:      "Fails in diff mode when exported Go functions, methods, types, or consts are removed or renamed, or when an exported function signature changes against the base ref.",
		HowToFix:         "Restore the exported declaration or signature, or ship the break deliberately with a deprecation path and a major version bump.",
	},
	"contracts.cpp-public-breaking": {
		ID:               "contracts.cpp-public-breaking",
		Section:          "API Contracts",
		DefaultLevel:     "fail",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(core.RuleLanguageCPP),
		Title:            "C++ public-header breaking change",
		Description:      "Fails in diff mode when a declaration in an include, public, or api header is removed, renamed, or changes signature against the base ref.",
		HowToFix:         "Keep the old declaration available with a compatibility implementation, or deliberately version the public C++ API and ABI.",
	},
	"contracts.openapi-breaking": {
		ID:               "contracts.openapi-breaking",
		Section:          "API Contracts",
		DefaultLevel:     "fail",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "OpenAPI breaking change",
		Description:      "Fails in diff mode when an OpenAPI document removes paths, operations, or response codes, or makes request parameters or body fields newly required against the base ref.",
		HowToFix:         "Keep removed paths and operations available, or version the API so existing clients keep a working contract.",
	},
	"contracts.proto-breaking": {
		ID:               "contracts.proto-breaking",
		Section:          "API Contracts",
		DefaultLevel:     "fail",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Protobuf breaking change",
		Description:      "Fails in diff mode when a .proto file removes messages, services, or rpcs, or removes, renumbers, or retypes message fields against the base ref.",
		HowToFix:         "Reserve removed field numbers instead of reusing them, and deprecate rather than delete messages, services, and rpcs.",
	},
	"contracts.migration-destructive": {
		ID:               "contracts.migration-destructive",
		Section:          "API Contracts",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Destructive database migration",
		Description:      "Warns when migration files contain destructive operations such as DROP TABLE, DROP COLUMN, TRUNCATE, or ALTER ... NOT NULL without a DEFAULT.",
		HowToFix:         "Confirm the data loss is intended, back up affected data first, and prefer additive or reversible migrations.",
	},
}
