package ci

import (
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

type ciRules struct {
	requireWorkflowDir    bool
	requiredWorkflowFiles []string
	workflowContentRules  []core.WorkflowRuleConfig
	requiredReleaseFiles  []string
	requiredAutomation    []string
}

func resolveCIRules(cfg core.CIRulesConfig) ciRules {
	rules := ciRules{
		requireWorkflowDir:    true,
		requiredWorkflowFiles: []string{".github/workflows/ci.yml", ".github/workflows/cd.yml", ".github/workflows/release.yml"},
		workflowContentRules: []core.WorkflowRuleConfig{
			{
				Path:             ".github/workflows/ci.yml",
				RequiredContains: []string{"actions/checkout", "go test ./..."},
			},
			{
				Path:             ".github/workflows/cd.yml",
				RequiredContains: []string{"googleapis/release-please-action", "uses: ./.github/workflows/release.yml", "RELEASE_PLEASE_TOKEN"},
			},
			{
				Path:             ".github/workflows/release.yml",
				RequiredContains: []string{"goreleaser/goreleaser-action@v7", "sync-homebrew-formula", "Formula/codeguard.rb"},
			},
		},
		requiredReleaseFiles: []string{".goreleaser.yaml", "Dockerfile.release", ".github/release-please-config.json", ".release-please-manifest.json", "CHANGELOG.md"},
		requiredAutomation:   []string{"Makefile", "scripts/commit.sh"},
	}
	rules.requireWorkflowDir = boolValue(cfg.RequireWorkflowDir, rules.requireWorkflowDir)
	if cfg.RequiredWorkflowFiles != nil {
		rules.requiredWorkflowFiles = normalizePaths(cfg.RequiredWorkflowFiles)
	}
	if cfg.WorkflowContentRules != nil {
		rules.workflowContentRules = normalizeWorkflowRules(cfg.WorkflowContentRules)
	}
	if cfg.RequiredReleaseFiles != nil {
		rules.requiredReleaseFiles = normalizePaths(cfg.RequiredReleaseFiles)
	}
	if cfg.RequiredAutomationPaths != nil {
		rules.requiredAutomation = normalizePaths(cfg.RequiredAutomationPaths)
	}
	return rules
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func normalizePaths(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func normalizeWorkflowRules(values []core.WorkflowRuleConfig) []core.WorkflowRuleConfig {
	out := make([]core.WorkflowRuleConfig, 0, len(values))
	for _, value := range values {
		path := strings.TrimSpace(value.Path)
		if path == "" {
			continue
		}
		out = append(out, core.WorkflowRuleConfig{
			Path:             path,
			RequiredContains: normalizePaths(value.RequiredContains),
		})
	}
	return out
}
