package checks_test

import (
	"context"
	"os"
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

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertSectionStatus(t *testing.T, report codeguard.Report, name string, want string) {
	t.Helper()
	for _, section := range report.Sections {
		if section.Name == name {
			if string(section.Status) != want {
				t.Fatalf("%s status = %q, want %q", name, section.Status, want)
			}
			return
		}
	}
	t.Fatalf("section %q not found", name)
}

func assertSectionFindingCountAtLeast(t *testing.T, report codeguard.Report, name string, min int) {
	t.Helper()
	for _, section := range report.Sections {
		if section.Name == name {
			if len(section.Findings) < min {
				t.Fatalf("%s findings = %d, want at least %d", name, len(section.Findings), min)
			}
			return
		}
	}
	t.Fatalf("section %q not found", name)
}
