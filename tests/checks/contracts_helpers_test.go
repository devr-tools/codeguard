package checks_test

import (
	"context"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func initContractsRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "CodeGuard Test")
	return dir
}

//nolint:unparam // general-purpose test helper; message is part of its API shape
func commitAll(t *testing.T, dir string, message string) {
	t.Helper()
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", message)
}

func contractsTestConfig(dir string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = "contracts-test"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cacheOff := false
	cfg.Cache.Enabled = &cacheOff
	return cfg
}

func runContractsDiff(t *testing.T, cfg codeguard.Config) codeguard.Report {
	t.Helper()
	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("diff scan: %v", err)
	}
	return report
}

func contractsRuleFindings(report codeguard.Report, ruleID string) []codeguard.Finding {
	findings := make([]codeguard.Finding, 0)
	for _, section := range report.Sections {
		if section.ID != "contracts" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID {
				findings = append(findings, finding)
			}
		}
	}
	return findings
}

func contractsRuleMessages(report codeguard.Report, ruleID string) []string {
	messages := make([]string, 0)
	for _, finding := range contractsRuleFindings(report, ruleID) {
		messages = append(messages, finding.Message)
	}
	return messages
}

func assertMessageContaining(t *testing.T, messages []string, needle string) {
	t.Helper()
	for _, message := range messages {
		if strings.Contains(message, needle) {
			return
		}
	}
	t.Fatalf("expected a finding message containing %q, got %v", needle, messages)
}
