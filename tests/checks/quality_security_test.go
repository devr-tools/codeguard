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
