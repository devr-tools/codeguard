package codeguard_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestValidateConfigRejectsBlankTargetPath(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: "", Language: "go"}}

	err := codeguard.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "target path is required") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateConfigRejectsOverlappingSupplyChainLicensePolicy(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.Checks.SupplyChainRules.AllowedLicenses = []string{"MIT"}
	cfg.Checks.SupplyChainRules.DeniedLicenses = []string{"mit"}

	err := codeguard.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "allowed_licenses and denied_licenses must not overlap") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateConfigRejectsSupplyChainLicenseCommandWithoutName(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.Checks.SupplyChainRules.LicenseCommands = map[string]codeguard.CommandCheckConfig{
		"npm": {Command: "./resolve-licenses.sh"},
	}

	err := codeguard.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "supply_chain_rules.license_commands[npm].name is required") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestWriteReportTextIncludesSummary(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	report := codeguard.Report{
		Name:        "sample",
		GeneratedAt: "2026-06-10T19:30:00Z",
		Sections: []codeguard.SectionResult{
			{
				Name:   "Code Quality",
				Status: "warn",
				Findings: []codeguard.Finding{{
					RuleID:      "quality.max-function-lines",
					Level:       "warn",
					Path:        "main.go",
					Line:        12,
					Message:     "function is too long",
					Severity:    "warn",
					Fingerprint: "abc123",
				}},
			},
		},
		Summary: codeguard.ReportSummary{
			WarnedSections: 1,
			TotalFindings:  1,
		},
	}

	var out bytes.Buffer
	if err := codeguard.WriteReport(&out, report, "text"); err != nil {
		t.Fatalf("write text report: %v", err)
	}

	rendered := out.String()
	rendered = stripANSI(rendered)
	if !strings.Contains(rendered, "sample") {
		t.Fatalf("missing header in report:\n%s", rendered)
	}
	if !strings.Contains(rendered, "quality.max-function-lines") {
		t.Fatalf("missing grouped finding subsection in report:\n%s", rendered)
	}
	if !strings.Contains(rendered, "1. at: main.go:12") {
		t.Fatalf("missing finding location in report:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Summary: 0 pass, 1 warn, 0 fail, 1 findings, 0 suppressed") {
		t.Fatalf("missing summary in report:\n%s", rendered)
	}
}

func TestRunPatchUsesPatchedContent(t *testing.T) {
	dir := t.TempDir()
	promptPath := filepath.Join(dir, "prompts", "system.prompt")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(promptPath, []byte("Keep prompts generic.\n"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	cfg := codeguard.ExampleConfig()
	cfg.Targets = []codeguard.TargetConfig{{
		Name:     "repo",
		Path:     dir,
		Language: "go",
	}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = true
	cfg.Checks.CI = false
	cfg.Output.Format = "json"

	diff := strings.Join([]string{
		"diff --git a/prompts/system.prompt b/prompts/system.prompt",
		"index 6d6dd26..9a4f7f4 100644",
		"--- a/prompts/system.prompt",
		"+++ b/prompts/system.prompt",
		"@@ -1 +1 @@",
		"-Keep prompts generic.",
		"+Use ${OPENAI_API_KEY} for downstream calls.",
		"",
	}, "\n")

	report, err := codeguard.RunPatch(context.Background(), cfg, diff)
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}
	if report.Summary.FailedSections == 0 || report.Summary.TotalFindings == 0 {
		t.Fatalf("expected failing report from patched content, got %#v", report.Summary)
	}
	if len(report.Sections) == 0 || len(report.Sections[0].Findings) == 0 {
		t.Fatalf("expected findings, got %#v", report.Sections)
	}
	if got := report.Sections[0].Findings[0].RuleID; got != "prompts.secret-interpolation" {
		t.Fatalf("unexpected rule id: %s", got)
	}

	data, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	if strings.Contains(string(data), "OPENAI_API_KEY") {
		t.Fatalf("working tree file was modified: %s", string(data))
	}
}

func TestRunPatchProvidesDiffCommandBaseAndHeadDirs(t *testing.T) {
	dir := t.TempDir()
	writeAPITestFile(t, filepath.Join(dir, "go.mod"), "module example.com/patchdiff\n\ngo 1.23.0\n")
	writeAPITestFile(t, filepath.Join(dir, "api.go"), "package patchdiff\n\nfunc Stable() {}\n")

	runAPITestGit(t, dir, "init", "-b", "main")

	script := filepath.Join(dir, "api-diff-check.sh")
	writeAPITestFile(t, script, "#!/bin/sh\nif grep -q 'func Stable' \"$CODEGUARD_DIFF_BASE_DIR/api.go\" && ! grep -q 'func Stable' \"$CODEGUARD_DIFF_HEAD_DIR/api.go\"; then\n  echo 'exported symbol Stable removed'\n  exit 1\nfi\n")
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	cfg := codeguard.ExampleConfig()
	cfg.Name = "patch-diff-command"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.DesignRules.LanguageDiffCommands = map[string][]codeguard.CommandCheckConfig{
		"go": {{
			Name:    "api-diff",
			Command: "./api-diff-check.sh",
		}},
	}

	diff := strings.Join([]string{
		"diff --git a/api.go b/api.go",
		"index 6d6dd26..9a4f7f4 100644",
		"--- a/api.go",
		"+++ b/api.go",
		"@@ -1,3 +1,3 @@",
		" package patchdiff",
		" ",
		"-func Stable() {}",
		"+func Replacement() {}",
		"",
	}, "\n")

	report, err := codeguard.RunPatch(context.Background(), cfg, diff)
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}
	if len(report.Sections) == 0 || len(report.Sections[0].Findings) == 0 {
		t.Fatalf("expected diff command findings, got %#v", report.Sections)
	}
	if got := report.Sections[0].Findings[0].RuleID; got != "design.diff-command-check" {
		t.Fatalf("unexpected diff command rule id: %s", got)
	}
	if !strings.Contains(report.Sections[0].Findings[0].Message, "Stable removed") {
		t.Fatalf("expected diff command output in finding, got %q", report.Sections[0].Findings[0].Message)
	}

	data, err := os.ReadFile(filepath.Join(dir, "api.go"))
	if err != nil {
		t.Fatalf("read api.go: %v", err)
	}
	if strings.Contains(string(data), "Replacement") {
		t.Fatalf("working tree file was modified: %s", string(data))
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

func writeAPITestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func runAPITestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(out))
	}
}
