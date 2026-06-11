package codeguard_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard"
)

func TestRunnerProducesSections(t *testing.T) {
	report, err := codeguard.Run(context.Background(), codeguard.ExampleConfig())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(report.Sections) == 0 {
		t.Fatal("expected report sections")
	}
	if report.Summary.PassedSections == 0 {
		t.Fatal("expected at least one passing section")
	}
}

func TestRunnerDisablesIndividualChecks(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.Checks.Security = false
	cfg.Checks.CI = false

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	sections := make(map[string]string, len(report.Sections))
	for _, section := range report.Sections {
		sections[section.Name] = string(section.Status)
	}

	if sections["Security"] != "skip" {
		t.Fatalf("expected Security to be skipped, got %q", sections["Security"])
	}
	if sections["CI/CD"] != "skip" {
		t.Fatalf("expected CI/CD to be skipped, got %q", sections["CI/CD"])
	}
}

func TestRunnerWarnsForFutureLanguageSupport(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.Targets = append(cfg.Targets, codeguard.TargetConfig{
		Name:     "frontend",
		Path:     ".",
		Language: "typescript",
	})

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, section := range report.Sections {
		if section.Name != "Language Support" {
			continue
		}
		if string(section.Status) != "warn" {
			t.Fatalf("expected warn status, got %q", section.Status)
		}
		if !strings.Contains(section.Note, "future language support") {
			t.Fatalf("unexpected note %q", section.Note)
		}
		return
	}

	t.Fatal("Language Support section not found")
}

func TestYAMLConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codeguard.yaml")

	cfg := codeguard.ExampleConfig()
	if err := codeguard.WriteConfigFile(path, cfg); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	loaded, err := codeguard.LoadConfigFile(path)
	if err != nil {
		t.Fatalf("load yaml: %v", err)
	}
	if loaded.Name != cfg.Name {
		t.Fatalf("loaded name = %q, want %q", loaded.Name, cfg.Name)
	}
}

func TestDiffScanScopesFileBasedChecks(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, filepath.Join(dir, "go.mod"), "module example.com/diffscan\n\ngo 1.23.0\n")
	writeRepoFile(t, filepath.Join(dir, "good.go"), "package main\n\nfunc good() {}\n")
	writeRepoFile(t, filepath.Join(dir, "bad.go"), "package main\nfunc bad(){println(\"hi\")}\n")

	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	runGit(t, dir, "checkout", "-b", "feature")

	writeRepoFile(t, filepath.Join(dir, "good.go"), "package main\n\nfunc good() {\n\tprintln(\"updated\")\n}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Security = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false

	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("diff scan: %v", err)
	}

	for _, section := range report.Sections {
		if section.Name != "Code Quality" {
			continue
		}
		if string(section.Status) == "fail" {
			t.Fatalf("expected diff scan not to fail on untouched bad.go: %+v", section.Findings)
		}
		return
	}
	t.Fatal("Code Quality section not found")
}

func writeRepoFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(out))
	}
}
