package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// baseExampleConfig is the rich, opinionated starter configuration that
// `codeguard init` writes and that ExampleConfig exposes. It is the
// conservative default baseline (baseDefaultConfig) plus a starter policy
// overlay.
//
// The split matters because ApplyDefaults resolves omitted keys against
// baseDefaultConfig, NOT against this function. Before the split there was one
// constructor serving both roles, so every opinion written for the starter
// config — codeguard's own workflow filenames, release artifacts, `go test
// ./...` marker and tests/ layout — was silently the effective global default
// for anyone who omitted those keys. Anything added here is a suggestion the
// user reads and edits in their own config file; anything added to
// baseDefaultConfig is policy enforced on every user.
func baseExampleConfig() core.Config {
	cfg := baseDefaultConfig()
	applyStarterPolicy(&cfg)
	return cfg
}

// applyStarterPolicy layers codeguard's own house conventions onto the
// conservative baseline. Everything set here is written into the generated
// config file, so the user can see it and change it; none of it reaches a
// repository that does not have a config, or that omits the key.
func applyStarterPolicy(cfg *core.Config) {
	cfg.Targets = starterTargets()
	applyStarterSectionActivation(&cfg.Checks)
	applyStarterCIPolicy(&cfg.Checks.CIRules)
	applyStarterDesignLayering(&cfg.Checks.DesignRules)
}

func starterTargets() []core.TargetConfig {
	return []core.TargetConfig{{
		Name:        "repository",
		Path:        ".",
		Language:    "go",
		Entrypoints: []string{"cmd/codeguard"},
	}}
}

// applyStarterSectionActivation turns on the recommended section set. The
// plain-bool sections get no defaulting in applyCheckDefaults (an omitted key
// means off), so a generated config has to state them.
func applyStarterSectionActivation(checks *core.CheckConfig) {
	checks.Quality = true
	checks.Design = true
	checks.Security = true
	checks.Prompts = true
	checks.CI = true
}

// applyStarterCIPolicy writes codeguard's own pipeline conventions. These are
// starter suggestions, not defaults: a repository with a differently named
// workflow, a different release tool, no Makefile, or co-located tests should
// edit or delete them. See baseDefaultCIRules for why none of this is a
// default.
func applyStarterCIPolicy(rules *core.CIRulesConfig) {
	rules.RequiredWorkflowFiles = []string{".github/workflows/ci.yml"}
	rules.RequiredGates = []string{"test"}
	rules.WorkflowContentRules = []core.WorkflowRuleConfig{{
		Path:             ".github/workflows/ci.yml",
		RequiredContains: []string{"actions/checkout", "go test ./..."},
	}}
	rules.RequiredReleaseFiles = []string{".goreleaser.yaml"}
	rules.RequiredAutomationPaths = []string{"Makefile"}
	rules.AllowedTestPaths = []string{"tests/**"}
}

// applyStarterDesignLayering writes the cmd/ -> internal/cli and pkg/ facade
// layering rules. They describe this repository's structure; see
// baseDefaultDesignRules for why they are off by default.
func applyStarterDesignLayering(rules *core.DesignRulesConfig) {
	rules.RequireCmdThroughInternalCLI = boolPtr(true)
	rules.ForbidInternalImportCmd = boolPtr(true)
	rules.ForbidServiceImportInternal = boolPtr(true)
	rules.ForbidServiceImportCmd = boolPtr(true)
}
