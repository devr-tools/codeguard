package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var miscCatalog = map[string]core.RuleMetadata{
	"prompts.secret-interpolation": {
		ID:             "prompts.secret-interpolation",
		Section:        "AI Prompts",
		DefaultLevel:   "fail",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "Prompt secret interpolation",
		Description:    "Fails when prompt assets interpolate likely secret values.",
		HowToFix:       "Remove secret placeholders from prompt assets and inject secrets outside the prompt text.",
	},
	"prompts.unsafe-instructions": {
		ID:             "prompts.unsafe-instructions",
		Section:        "AI Prompts",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "Unsafe prompt instructions",
		Description:    "Warns when prompt assets contain instruction-injection or system prompt exfiltration patterns.",
		HowToFix:       "Rewrite the prompt to remove instruction override or prompt exfiltration language.",
	},
	"ci.required-workflow-dir": {
		ID:             "ci.required-workflow-dir",
		Section:        "CI/CD",
		DefaultLevel:   "fail",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "Workflow directory",
		Description:    "Fails when the configured workflow directory is required but missing.",
		HowToFix:       "Add the required workflow directory or disable the policy explicitly.",
	},
	"ci.required-file": {
		ID:             "ci.required-file",
		Section:        "CI/CD",
		DefaultLevel:   "fail",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "Required CI file",
		Description:    "Fails when required workflow, release, or automation files are missing.",
		HowToFix:       "Add the required file or remove the policy requirement if it does not apply.",
	},
	"ci.workflow-content": {
		ID:             "ci.workflow-content",
		Section:        "CI/CD",
		DefaultLevel:   "fail",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "Workflow content",
		Description:    "Fails when required workflow file markers are absent.",
		HowToFix:       "Update the workflow file so the required steps or markers are present.",
	},
	"ci.test-file-location": {
		ID:             "ci.test-file-location",
		Section:        "CI/CD",
		DefaultLevel:   "fail",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "Test file location",
		Description:    "Fails when language-specific test files live outside the configured test directories.",
		HowToFix:       "Move the test file under the configured test path or update the CI policy if the layout is intentional.",
	},
}
