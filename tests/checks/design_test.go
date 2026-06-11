package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestDesignCheckFailsWhenServiceImportsInternal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pkg", "publicapi", "service.go"), "package publicapi\n\nimport _ \"github.com/devr-tools/codeguard/internal/cli\"\n")

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
	writeFile(t, filepath.Join(dir, "cmd", "tool", "main.go"), "package main\n\nimport _ \"github.com/devr-tools/codeguard/pkg/codeguard\"\n")

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
	writeFile(t, filepath.Join(dir, "internal", "cli", "run.go"), "package cli\n\nimport _ \"github.com/devr-tools/codeguard/pkg/codeguard\"\n")
	writeFile(t, filepath.Join(dir, "pkg", "codeguard", "sdk.go"), "package codeguard\n")

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
	writeFile(t, filepath.Join(dir, "cmd", "tool", "main.go"), "package main\n\nimport _ \"github.com/devr-tools/codeguard/pkg/codeguard\"\n")

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
	writeFile(t, filepath.Join(dir, "pkg", "codeguard", "util.go"), "package util\n")

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
	writeFile(t, filepath.Join(dir, "pkg", "codeguard", "service.go"), "package codeguard\n\ntype Service struct{}\n\nfunc (Service) A(){}\nfunc (Service) B(){}\nfunc (Service) C(){}\n")

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
	writeFile(t, filepath.Join(dir, "pkg", "codeguard", "ports.go"), "package codeguard\n\ntype Client interface {\n\tA()\n\tB()\n\tC()\n}\n")

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

func TestDesignCheckFailsForConfiguredTypeScriptCommand(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "index.ts"), "export const answer = 42;\n")
	script := filepath.Join(dir, "fake-design-check.sh")
	writeExecutableFile(t, script, "#!/bin/sh\necho 'src/index.ts imports forbidden layer'\nexit 1\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-typescript-command"
	cfg.Targets = []codeguard.TargetConfig{{Name: "web", Path: dir, Language: "typescript"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.DesignRules.LanguageCommands = map[string][]codeguard.CommandCheckConfig{
		"typescript": {{
			Name:    "depcruise",
			Command: script,
		}},
	}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
	assertFindingRulePresent(t, report, "Design Patterns", "design.command-check")

	findings := report.Sections[0].Findings
	if len(findings) == 0 {
		t.Fatal("expected command finding")
	}
	if !strings.Contains(findings[0].Message, "depcruise") {
		t.Fatalf("expected command name in message, got %q", findings[0].Message)
	}
	if findings[0].Title == "" {
		t.Fatal("expected runtime metadata title for design.command-check")
	}
}

func TestDesignCheckFailsForConfiguredDiffCommand(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/contractdiff\n\ngo 1.23.0\n")
	writeFile(t, filepath.Join(dir, "api.go"), "package contractdiff\n\nfunc Stable() {}\n")

	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	runGit(t, dir, "checkout", "-b", "feature")

	writeFile(t, filepath.Join(dir, "api.go"), "package contractdiff\n\nfunc Replacement() {}\n")

	script := filepath.Join(dir, "api-diff-check.sh")
	writeExecutableFile(t, script, "#!/bin/sh\nif grep -q 'func Stable' \"$CODEGUARD_DIFF_BASE_DIR/api.go\" && ! grep -q 'func Stable' \"$CODEGUARD_DIFF_HEAD_DIR/api.go\"; then\n  echo 'exported symbol Stable removed'\n  exit 1\nfi\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-go-diff-command"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.DesignRules.LanguageDiffCommands = map[string][]codeguard.CommandCheckConfig{
		"go": {{
			Name:    "api-diff",
			Command: script,
		}},
	}

	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Design Patterns", "fail")
	assertFindingRulePresent(t, report, "Design Patterns", "design.diff-command-check")

	findings := report.Sections[0].Findings
	if len(findings) == 0 {
		t.Fatal("expected diff command finding")
	}
	if !strings.Contains(findings[0].Message, "Stable removed") {
		t.Fatalf("expected diff command output in message, got %q", findings[0].Message)
	}
}
