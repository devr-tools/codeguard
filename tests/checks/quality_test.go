package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard"
)

func TestQualityCheckFailsForUnformattedGoFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\nfunc main(){println(\"hi\")}\n")

	report, err := codeguard.Run(context.Background(), codeguard.Config{
		Name: "quality-test",
		Targets: []codeguard.TargetConfig{{
			Name:     "repo",
			Path:     dir,
			Language: "go",
		}},
		Checks: codeguard.CheckConfig{
			Quality: true,
		},
		Output: codeguard.OutputConfig{Format: "text"},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "fail")
}

func TestQualityCheckWarnsForMaintainabilityThresholds(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc sample(a, b int) int {\n\treturn a + b\n}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-threshold-test"
	cfg.Targets = []codeguard.TargetConfig{{
		Name:     "repo",
		Path:     dir,
		Language: "go",
	}}
	cfg.Checks.Quality = true
	cfg.Checks.QualityRules.MaxFunctionLines = 1
	cfg.Checks.QualityRules.MaxParameters = 1
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
}

func TestQualityCheckWarnsForCyclomaticComplexity(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc sample(a int) int {\n\tif a > 0 {\n\t\ta++\n\t}\n\tif a > 1 {\n\t\ta++\n\t}\n\tif a > 2 {\n\t\ta++\n\t}\n\treturn a\n}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-complexity-test"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = 2
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
}

func TestQualityCheckWarnsForDependencyDirection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lib.go"), "package sample\n\nimport cli \"github.com/devr-tools/codeguard/internal/cli\"\n\nvar _ = cli.Run\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-deps-test"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = true
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Code Quality", "warn")
}
