package contextgen

import (
	"context"
	"fmt"

	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/runner"
)

// CheckResult is the agent-context section's verdict on a repository.
type CheckResult struct {
	// Score is the repo_legibility score, 0-100.
	Score int
	// Components is the score's breakdown.
	Components []core.RepoLegibilityComponent
	// Findings are every agent-context finding the scan reported.
	Findings []core.Finding
}

// Check runs only the agent-context section against root, with rules as the
// section's tuning (the zero value means built-in defaults). The finding
// cache and legibility history are disabled: a post-write verification must
// neither read stale results nor add a point to the repository's trend.
func Check(ctx context.Context, root string, rules core.ContextRulesConfig) (CheckResult, error) {
	off, on := false, true
	rules.LegibilityHistory = &off
	cfg := core.Config{
		Name:    "codeguard-context",
		Targets: []core.TargetConfig{{Name: "repository", Path: root}},
		Cache:   core.CacheConfig{Enabled: &off},
		Checks: core.CheckConfig{
			Context:      &on,
			ContextRules: rules,
			Disabled:     []string{"quality", "performance", "design", "security", "prompts", "ci", "supply_chain", "delivery", "reliability", "data", "change", "contracts"},
		},
	}
	config.ApplyDefaults(&cfg)
	report, err := runner.Run(ctx, cfg)
	if err != nil {
		return CheckResult{}, err
	}
	result := CheckResult{Score: -1}
	for _, section := range report.Sections {
		if section.ID == "context" {
			result.Findings = append(result.Findings, section.Findings...)
		}
	}
	for _, artifact := range report.Artifacts {
		if artifact.Kind == "repo_legibility" && artifact.RepoLegibility != nil {
			result.Score = artifact.RepoLegibility.Score
			result.Components = artifact.RepoLegibility.Components
		}
	}
	if result.Score < 0 {
		return CheckResult{}, fmt.Errorf("agent-context scan of %s produced no repo_legibility score", root)
	}
	return result, nil
}
