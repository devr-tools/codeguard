package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// ruleExamples holds the bad/good snippet pairs `codeguard explain` shows.
// tests/rules/examples_test.go scans every pair: the bad snippet must trigger
// the rule and the good one must not, so an example can never drift into
// fiction. Entries live in catalog_examples_*.go, split by rule family.
var ruleExamples = mergeRuleExamples(
	securityExamples,
	promptExamples,
	ciWorkflowExamples,
	qualityExamples,
	designExamples,
)

// ruleConfidence records the confidence a rule's findings typically carry.
// Only rules with verified examples are listed: the examples test asserts the
// bad snippet's findings carry exactly this confidence (an unspecified
// confidence counts as medium, as it does everywhere else).
var ruleConfidence = map[string]string{
	"ci.workflow-dangerous-trigger":         core.ConfidenceHigh,
	"ci.workflow-fork-trigger-secrets":      core.ConfidenceHigh,
	"ci.workflow-missing-permissions":       core.ConfidenceHigh,
	"ci.workflow-remote-script-pipe":        core.ConfidenceHigh,
	"ci.workflow-script-injection":          core.ConfidenceHigh,
	"ci.workflow-write-all-permissions":     core.ConfidenceHigh,
	"defensive.bounds-assumption":           core.ConfidenceMedium,
	"defensive.integer-overflow":            core.ConfidenceMedium,
	"defensive.invalid-state-representable": core.ConfidenceMedium,
	"defensive.missing-schema-validation":   core.ConfidenceMedium,
	"defensive.unchecked-type-assertion":    core.ConfidenceHigh,
	"defensive.unvalidated-boundary-input":  core.ConfidenceMedium,
	"design.max-decls-per-file":             core.ConfidenceMedium,
	"error.context-lost":                    core.ConfidenceMedium,
	"error.partial-failure-hidden":          core.ConfidenceMedium,
	"function.command-query-mix":            core.ConfidenceMedium,
	"function.excessive-parameters":         core.ConfidenceHigh,
	"function.hidden-mutation":              core.ConfidenceMedium,
	"function.inconsistent-return-contract": core.ConfidenceMedium,
	"function.mixed-abstraction-level":      core.ConfidenceMedium,
	"naming.boolean-not-predicate":          core.ConfidenceMedium,
	"prompts.agent-config-literal-secret":   core.ConfidenceMedium,
	"prompts.agent-dangerous-instructions":  core.ConfidenceHigh,
	"prompts.agent-standing-permissions":    core.ConfidenceHigh,
	"prompts.hidden-unicode":                core.ConfidenceHigh,
	"prompts.mcp-config-risk":               core.ConfidenceHigh,
	"prompts.mcp-insecure-transport":        core.ConfidenceHigh,
	"prompts.mcp-remote-unauthenticated":    core.ConfidenceLow,
	"prompts.mcp-unpinned-server":           core.ConfidenceHigh,
	"prompts.secret-interpolation":          core.ConfidenceMedium,
	"prompts.unsafe-instructions":           core.ConfidenceMedium,
	"quality.boolean-argument":              core.ConfidenceHigh,
	"quality.cyclomatic-complexity":         core.ConfidenceMedium,
	"quality.max-file-lines":                core.ConfidenceMedium,
	"quality.max-parameters":                core.ConfidenceMedium,
	"quality.mutable-global-state":          core.ConfidenceHigh,
	"security.javascript.taint-flow":        core.ConfidenceMedium,
	"security.ssrf.cpp":                     core.ConfidenceHigh,
	"security.ssrf.go":                      core.ConfidenceHigh,
	"security.ssrf.python":                  core.ConfidenceHigh,
	"security.taint.cpp":                    core.ConfidenceHigh,
	"security.taint.go":                     core.ConfidenceHigh,
	"security.taint.python":                 core.ConfidenceHigh,
	"security.typescript.taint-flow":        core.ConfidenceMedium,
}

func mergeRuleExamples(parts ...map[string][]core.RuleExample) map[string][]core.RuleExample {
	merged := map[string][]core.RuleExample{}
	for _, part := range parts {
		for id, examples := range part {
			merged[id] = append(merged[id], examples...)
		}
	}
	return merged
}

// goExample, pythonExample, and friends keep the per-family tables terse.
// Only Go threshold examples need overrides today.
func goExample(path, bad, good string, overrides ...string) core.RuleExample {
	return core.RuleExample{Language: "go", Path: path, Bad: bad, Good: good, Overrides: overrides}
}

func pythonExample(path, bad, good string) core.RuleExample {
	return core.RuleExample{Language: "python", Path: path, Bad: bad, Good: good}
}

func cppExample(path, bad, good string) core.RuleExample {
	return core.RuleExample{Language: "cpp", Path: path, Bad: bad, Good: good}
}

func typescriptExample(path, bad, good string) core.RuleExample {
	return core.RuleExample{Language: "typescript", Path: path, Bad: bad, Good: good}
}

func javascriptExample(path, bad, good string) core.RuleExample {
	return core.RuleExample{Language: "javascript", Path: path, Bad: bad, Good: good}
}

func yamlExample(path, bad, good string) core.RuleExample {
	return core.RuleExample{Language: "yaml", Path: path, Bad: bad, Good: good}
}

func jsonExample(path, bad, good string) core.RuleExample {
	return core.RuleExample{Language: "json", Path: path, Bad: bad, Good: good}
}

func markdownExample(path, bad, good string) core.RuleExample {
	return core.RuleExample{Language: "markdown", Path: path, Bad: bad, Good: good}
}

// explicitRuleConfigKeys lists the rule-specific keys beyond the
// checks.<section> toggle every rule gets by convention. Enable keys are
// verified by the examples test (turning one off must silence the bad
// snippet); every key is checked against the config structs' yaml tags.
var explicitRuleConfigKeys = map[string][]core.RuleConfigKey{
	"security.taint.go":              {enableKey("checks.security_rules.taint_go")},
	"security.ssrf.go":               {enableKey("checks.security_rules.taint_go")},
	"security.taint.python":          {enableKey("checks.security_rules.taint_python")},
	"security.ssrf.python":           {enableKey("checks.security_rules.taint_python")},
	"security.taint.cpp":             {enableKey("checks.security_rules.taint_cpp")},
	"security.ssrf.cpp":              {enableKey("checks.security_rules.taint_cpp")},
	"security.typescript.taint-flow": {tuneKey("checks.security_rules.typescript_taint_max_depth")},
	"security.javascript.taint-flow": {tuneKey("checks.security_rules.typescript_taint_max_depth")},

	"prompts.secret-interpolation": {
		enableKey("checks.prompt_rules.forbid_secret_interpolation"),
		tuneKey("checks.prompt_rules.path_contains"),
		tuneKey("checks.prompt_rules.file_extensions"),
	},
	"prompts.unsafe-instructions": {
		enableKey("checks.prompt_rules.forbid_unsafe_instructions"),
		tuneKey("checks.prompt_rules.path_contains"),
		tuneKey("checks.prompt_rules.file_extensions"),
	},
	"prompts.mcp-remote-unauthenticated": {enableKey("checks.prompt_rules.require_mcp_remote_auth")},

	"quality.max-file-lines":        {tuneKey("checks.quality_rules.max_file_lines")},
	"quality.max-function-lines":    {tuneKey("checks.quality_rules.max_function_lines")},
	"quality.max-parameters":        {tuneKey("checks.quality_rules.max_parameters")},
	"quality.cyclomatic-complexity": {tuneKey("checks.quality_rules.max_cyclomatic_complexity")},
	"quality.duplicate-code":        {tuneKey("checks.quality_rules.clone_token_threshold")},
	"design.max-decls-per-file":     {tuneKey("checks.design_rules.max_decls_per_file")},
	"design.max-methods-per-type":   {tuneKey("checks.design_rules.max_methods_per_type")},
	"design.max-interface-methods":  {tuneKey("checks.design_rules.max_interface_methods")},
	"design.god-module":             {enableKey("checks.design_rules.detect_god_modules"), tuneKey("checks.design_rules.god_module_threshold")},
	"design.import-cycle":           {enableKey("checks.design_rules.detect_import_cycles")},

	"function.excessive-parameters": {localPrecisionKey, tuneKey("checks.quality_rules.max_parameters")},
}

// localPrecisionKey gates the local-precision family (function.*, error.*,
// defensive.*, naming.* and several quality.* design-smell rules).
var localPrecisionKey = enableKey("checks.quality_rules.local_precision")

// localPrecisionRules lists the rules verified (by their examples) to be
// switched off with quality_rules.local_precision=false.
var localPrecisionRules = []string{
	"function.inconsistent-return-contract",
	"function.hidden-mutation",
	"function.mixed-abstraction-level",
	"function.command-query-mix",
	"quality.mutable-global-state",
	"quality.boolean-argument",
	"naming.boolean-not-predicate",
	"error.context-lost",
	"error.partial-failure-hidden",
	"defensive.unvalidated-boundary-input",
	"defensive.missing-schema-validation",
	"defensive.invalid-state-representable",
	"defensive.integer-overflow",
	"defensive.bounds-assumption",
	"defensive.unchecked-type-assertion",
}

func init() {
	for _, id := range localPrecisionRules {
		explicitRuleConfigKeys[id] = append([]core.RuleConfigKey{localPrecisionKey}, explicitRuleConfigKeys[id]...)
	}
}

func enableKey(key string) core.RuleConfigKey {
	return core.RuleConfigKey{Key: key, Role: core.RuleConfigKeyRoleEnable}
}

func tuneKey(key string) core.RuleConfigKey {
	return core.RuleConfigKey{Key: key, Role: core.RuleConfigKeyRoleTune}
}
