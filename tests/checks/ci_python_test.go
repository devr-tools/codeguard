package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestCICheckFailsWhenPythonTestsLiveOutsideAllowedPaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pkg", "test_sample.py"), "def test_sample():\n    assert True\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "ci-python-test-location-fail"
	cfg.Targets = []codeguard.TargetConfig{{Name: "api", Path: dir, Language: "python"}}
	cfg.Checks.CI = true
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	disabled := false
	cfg.Checks.CIRules.RequireWorkflowDir = &disabled
	cfg.Checks.CIRules.RequiredWorkflowFiles = []string{}
	cfg.Checks.CIRules.RequiredReleaseFiles = []string{}
	cfg.Checks.CIRules.RequiredAutomationPaths = []string{}
	cfg.Checks.CIRules.WorkflowContentRules = []codeguard.WorkflowRuleConfig{}
	cfg.Checks.CIRules.AllowedTestPaths = []string{"tests/**"}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "CI/CD", "fail")
}

func TestCICheckPassesWhenPythonTestsLiveUnderAllowedPaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tests", "test_sample.py"), "def test_sample():\n    assert True\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "ci-python-test-location-pass"
	cfg.Targets = []codeguard.TargetConfig{{Name: "api", Path: dir, Language: "python"}}
	cfg.Checks.CI = true
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	disabled := false
	cfg.Checks.CIRules.RequireWorkflowDir = &disabled
	cfg.Checks.CIRules.RequiredWorkflowFiles = []string{}
	cfg.Checks.CIRules.RequiredReleaseFiles = []string{}
	cfg.Checks.CIRules.RequiredAutomationPaths = []string{}
	cfg.Checks.CIRules.WorkflowContentRules = []codeguard.WorkflowRuleConfig{}
	cfg.Checks.CIRules.AllowedTestPaths = []string{"tests/**"}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "CI/CD", "pass")
}
