package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard"
)

func TestDesignCheckFailsWhenServiceImportsInternal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "codeguard", "service.go"), "package codeguard\n\nimport _ \"github.com/devr-tools/codeguard/internal/cli\"\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-service-internal"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
}

func TestDesignCheckFailsWhenCmdImportsServiceDirectly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cmd", "tool", "main.go"), "package main\n\nimport _ \"github.com/devr-tools/codeguard/codeguard/runner\"\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-cmd-service"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
}

func TestDesignCheckPassesForLayeredLayout(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cmd", "tool", "main.go"), "package main\n\nimport _ \"github.com/devr-tools/codeguard/internal/cli\"\n")
	writeFile(t, filepath.Join(dir, "internal", "cli", "run.go"), "package cli\n\nimport _ \"github.com/devr-tools/codeguard/codeguard/runner\"\n")
	writeFile(t, filepath.Join(dir, "codeguard", "runner", "runner.go"), "package runner\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-pass"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "pass")
}

func TestDesignCheckAllowsDisabledRuleOverride(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cmd", "tool", "main.go"), "package main\n\nimport _ \"github.com/devr-tools/codeguard/codeguard/runner\"\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-override"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	disabled := false
	cfg.Checks.DesignRules.RequireCmdThroughInternalCLI = &disabled

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "pass")
}

func TestDesignCheckWarnsForGenericPackageName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "codeguard", "util.go"), "package util\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-package-name"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "warn")
}

func TestDesignCheckWarnsForTooManyMethodsOnType(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "codeguard", "service.go"), "package codeguard\n\ntype Service struct{}\n\nfunc (Service) A(){}\nfunc (Service) B(){}\nfunc (Service) C(){}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-srp"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.DesignRules.MaxMethodsPerType = 2

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "warn")
}

func TestDesignCheckWarnsForLargeInterface(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "codeguard", "ports.go"), "package codeguard\n\ntype Client interface {\n\tA()\n\tB()\n\tC()\n}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-isp"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.DesignRules.MaxInterfaceMethods = 2

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "warn")
}
