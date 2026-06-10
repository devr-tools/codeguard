package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard"
)

func TestCICheckFailsWhenRequiredAssetsAreMissing(t *testing.T) {
	dir := t.TempDir()

	cfg := codeguard.ExampleConfig()
	cfg.Name = "ci-missing"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.CI = true
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "CI/CD", "fail")
}

func TestCICheckPassesWhenRequiredAssetsExist(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".github", "workflows", "ci.yml"), "name: ci\n")
	writeFile(t, filepath.Join(dir, ".goreleaser.yaml"), "version: 2\n")
	writeFile(t, filepath.Join(dir, "Makefile"), "test:\n\tgo test ./...\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "ci-pass"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.CI = true
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "CI/CD", "pass")
}

func TestCICheckAllowsRuleOverride(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "buildkite.yml"), "steps: []\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "ci-override"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.CI = true
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	disabled := false
	cfg.Checks.CIRules.RequireWorkflowDir = &disabled
	cfg.Checks.CIRules.RequiredWorkflowFiles = []string{"buildkite.yml"}
	cfg.Checks.CIRules.RequiredReleaseFiles = []string{"buildkite.yml"}
	cfg.Checks.CIRules.RequiredAutomationPaths = []string{"buildkite.yml"}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "CI/CD", "pass")
}
