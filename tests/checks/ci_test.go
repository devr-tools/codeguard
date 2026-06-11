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
	writeFile(t, filepath.Join(dir, ".github", "workflows", "ci.yml"), "name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n      - run: go test ./...\n")
	writeFile(t, filepath.Join(dir, ".github", "workflows", "cd.yml"), "name: cd\njobs:\n  release-please:\n    steps:\n      - uses: googleapis/release-please-action@v5\n      - run: echo RELEASE_PLEASE_TOKEN\n      - run: echo uses: ./.github/workflows/release.yml\n")
	writeFile(t, filepath.Join(dir, ".github", "workflows", "release.yml"), "name: release\njobs:\n  build-release:\n    steps:\n      - uses: goreleaser/goreleaser-action@v7\n      - run: echo sync-homebrew-formula\n      - run: echo Formula/codeguard.rb\n")
	writeFile(t, filepath.Join(dir, ".goreleaser.yaml"), "version: 2\n")
	writeFile(t, filepath.Join(dir, ".github", "release-please-config.json"), "{}\n")
	writeFile(t, filepath.Join(dir, ".release-please-manifest.json"), "{\n  \".\": \"0.1.0\"\n}\n")
	writeFile(t, filepath.Join(dir, "CHANGELOG.md"), "# Changelog\n")
	writeFile(t, filepath.Join(dir, "Dockerfile.release"), "FROM alpine:3.20\nCOPY codeguard /usr/bin/codeguard\n")
	writeFile(t, filepath.Join(dir, "Makefile"), "test:\n\tgo test ./...\n")
	writeFile(t, filepath.Join(dir, "scripts", "commit.sh"), "#!/usr/bin/env bash\n")

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

func TestCICheckFailsWhenWorkflowContentIsMissing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".github", "workflows", "ci.yml"), "name: ci\njobs:\n  test:\n    steps:\n      - run: echo hi\n")
	writeFile(t, filepath.Join(dir, ".goreleaser.yaml"), "version: 2\n")
	writeFile(t, filepath.Join(dir, "Makefile"), "test:\n\tgo test ./...\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "ci-content-missing"
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
	cfg.Checks.CIRules.WorkflowContentRules = []codeguard.WorkflowRuleConfig{{
		Path:             "buildkite.yml",
		RequiredContains: []string{"steps:"},
	}}
	cfg.Checks.CIRules.RequiredReleaseFiles = []string{"buildkite.yml"}
	cfg.Checks.CIRules.RequiredAutomationPaths = []string{"buildkite.yml"}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "CI/CD", "pass")
}

func TestCICheckAllowsEmptyReleaseFileOverride(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".github", "workflows", "ci.yml"), "name: ci\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v4\n      - run: make codeguard-ci\n")
	writeFile(t, filepath.Join(dir, ".github", "workflows", "cd.yml"), "name: cd\njobs:\n  release-please:\n    steps:\n      - uses: googleapis/release-please-action@v5\n      - run: echo RELEASE_PLEASE_TOKEN\n      - run: echo uses: ./.github/workflows/release.yml\n")
	writeFile(t, filepath.Join(dir, ".github", "workflows", "release.yml"), "name: release\njobs:\n  build-release:\n    steps:\n      - uses: goreleaser/goreleaser-action@v7\n      - run: echo sync-homebrew-formula\n      - run: echo Formula/codeguard.rb\n")
	writeFile(t, filepath.Join(dir, "Makefile"), "codeguard-ci:\n\tgo test ./...\n")
	writeFile(t, filepath.Join(dir, "scripts", "commit.sh"), "#!/usr/bin/env bash\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "ci-empty-release-override"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.CI = true
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CIRules.RequiredReleaseFiles = []string{}
	cfg.Checks.CIRules.WorkflowContentRules = []codeguard.WorkflowRuleConfig{{
		Path:             ".github/workflows/ci.yml",
		RequiredContains: []string{"actions/checkout", "make codeguard-ci"},
	}, {
		Path:             ".github/workflows/cd.yml",
		RequiredContains: []string{"googleapis/release-please-action", "uses: ./.github/workflows/release.yml", "RELEASE_PLEASE_TOKEN"},
	}, {
		Path:             ".github/workflows/release.yml",
		RequiredContains: []string{"goreleaser/goreleaser-action@v7", "sync-homebrew-formula", "Formula/codeguard.rb"},
	}}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "CI/CD", "pass")
}
