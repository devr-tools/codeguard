package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard"
)

func TestSecurityCheckFailsForHardcodedSecret(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.go"), "package main\nconst apiKey = \"super-secret-token\"\n")

	report, err := codeguard.Run(context.Background(), codeguard.Config{
		Name: "security-test",
		Targets: []codeguard.TargetConfig{{
			Name:     "repo",
			Path:     dir,
			Language: "go",
		}},
		Checks: codeguard.CheckConfig{
			Security: true,
		},
		Output: codeguard.OutputConfig{Format: "text"},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "fail")
}

func TestSecurityCheckWarnsForShellExecution(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "exec.go"), "package main\nimport \"os/exec\"\nfunc main(){exec.Command(\"sh\")}\n")

	report, err := codeguard.Run(context.Background(), codeguard.Config{
		Name: "security-warn-test",
		Targets: []codeguard.TargetConfig{{
			Name:     "repo",
			Path:     dir,
			Language: "go",
		}},
		Checks: codeguard.CheckConfig{
			Security: true,
		},
		Output: codeguard.OutputConfig{Format: "text"},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "warn")
}

func TestSecurityCheckFailsWhenGovulncheckIsRequiredButMissing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\nfunc main() {}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "govulncheck-required"
	cfg.Targets = []codeguard.TargetConfig{{
		Name:     "repo",
		Path:     dir,
		Language: "go",
	}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SecurityRules.GovulncheckMode = "required"
	cfg.Checks.SecurityRules.GovulncheckCommand = "missing-govulncheck-binary"

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "fail")
}

func TestSecurityCheckWarnsWhenGovulncheckIsAutoButMissing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\nfunc main() {}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "govulncheck-auto"
	cfg.Targets = []codeguard.TargetConfig{{
		Name:     "repo",
		Path:     dir,
		Language: "go",
	}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SecurityRules.GovulncheckMode = "auto"
	cfg.Checks.SecurityRules.GovulncheckCommand = "missing-govulncheck-binary"

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "warn")
}

func TestSecurityCheckSurfacesStructuredGovulncheckFindings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\nfunc main() {}\n")
	script := filepath.Join(dir, "fake-govulncheck.sh")
	writeFile(t, script, "#!/bin/sh\necho 'Vulnerability #1: GO-2024-0001'\necho '  Found in: example.com/module@v1.0.0'\necho '  Fixed in: example.com/module@v1.0.1'\necho ''\necho 'Vulnerability #2: GO-2024-0002'\necho '  Found in: example.com/other@v0.9.0'\nexit 1\n")
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatalf("chmod script: %v", err)
	}

	cfg := codeguard.ExampleConfig()
	cfg.Name = "govulncheck-structured"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SecurityRules.GovulncheckMode = "required"
	cfg.Checks.SecurityRules.GovulncheckCommand = script

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Security", "fail")
	assertSectionFindingCountAtLeast(t, report, "Security", 2)
}
