package checks_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestNaturalLanguageCustomRuleFindingsWhenRuntimeEnabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handlers", "login.go"), "package handlers\n\nimport \"log\"\n\nfunc handleLogin(body string) {\n\tlog.Printf(\"body=%s\", body)\n}\n")
	runtimePath := writeNLRuleRuntime(t, dir)
	t.Setenv("CODEGUARD_AI_RUNTIME_COMMAND", runtimePath)

	cfg := codeguard.ExampleConfig()
	cfg.Name = "custom-nl-enabled"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Design = false
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.RulePacks = []codeguard.RulePackConfig{{
		Name: "repo-policy",
		Rules: []codeguard.CustomRuleConfig{{
			ID:              "custom.no-request-body-logs",
			Title:           "Never log request bodies",
			Severity:        "fail",
			Message:         "request bodies must not be logged in handlers",
			HowToFix:        "Remove request body logging and log a request identifier instead.",
			NaturalLanguage: "never log request bodies in handlers",
			Paths:           []string{"handlers/**"},
		}},
	}}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Custom Rules", "fail")
	if got := len(report.Sections[0].Findings); got != 1 {
		t.Fatalf("expected one finding, got %d", got)
	}
	finding := report.Sections[0].Findings[0]
	if finding.RuleID != "custom.no-request-body-logs" {
		t.Fatalf("unexpected rule id %q", finding.RuleID)
	}
	if finding.Line != 6 {
		t.Fatalf("expected line 6, got %d", finding.Line)
	}
	if !strings.Contains(finding.Why, "request body") {
		t.Fatalf("expected rationale in why field, got %q", finding.Why)
	}
}

func TestNaturalLanguageCustomRuleCacheInvalidatesWhenRuntimeEnables(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handlers", "login.go"), "package handlers\n\nimport \"log\"\n\nfunc handleLogin(body string) {\n\tlog.Printf(\"body=%s\", body)\n}\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "custom-nl-cache"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Design = false
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.RulePacks = []codeguard.RulePackConfig{{
		Name: "repo-policy",
		Rules: []codeguard.CustomRuleConfig{{
			ID:              "custom.no-request-body-logs",
			Title:           "Never log request bodies",
			Severity:        "fail",
			Message:         "request bodies must not be logged in handlers",
			NaturalLanguage: "never log request bodies in handlers",
			Paths:           []string{"handlers/**"},
		}},
	}}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run with runtime disabled: %v", err)
	}
	assertSectionStatus(t, report, "Custom Rules", "pass")

	runtimePath := writeNLRuleRuntime(t, dir)
	t.Setenv("CODEGUARD_AI_RUNTIME_COMMAND", runtimePath)

	report, err = codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run with runtime enabled: %v", err)
	}
	assertSectionStatus(t, report, "Custom Rules", "fail")
	if got := len(report.Sections[0].Findings); got != 1 {
		t.Fatalf("expected one finding after enabling runtime, got %d", got)
	}
}

func TestCacheFileCreatedAndInvalidatedOnContentChange(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "prompts", "system.prompt"), "Use ${OPENAI_API_KEY} for downstream calls.\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "cache-test"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Prompts = true
	cfg.Checks.Design = false
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.CI = false
	cfg.Cache.Path = filepath.Join(dir, ".codeguard", "cache.json")

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "AI Prompts", "fail")
	if _, err := os.Stat(cfg.Cache.Path); err != nil {
		t.Fatalf("expected cache file: %v", err)
	}

	writeFile(t, filepath.Join(dir, "prompts", "system.prompt"), "Safe prompt line.\n")
	report, err = codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run after edit: %v", err)
	}
	assertSectionStatus(t, report, "AI Prompts", "pass")
}

func TestProfileOverridesGovulncheckMode(t *testing.T) {
	cfg, err := codeguard.ExampleConfigForProfile("strict")
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if cfg.Profile != "strict" {
		t.Fatalf("profile = %q, want strict", cfg.Profile)
	}
	if cfg.Checks.SecurityRules.GovulncheckMode != "required" {
		t.Fatalf("govulncheck mode = %q, want required", cfg.Checks.SecurityRules.GovulncheckMode)
	}
}

func writeNLRuleRuntime(t *testing.T, dir string) string {
	t.Helper()
	runtimePath := filepath.Join(dir, "fake-nl-runtime.sh")
	script := strings.Join([]string{
		"#!/bin/sh",
		"cat >/dev/null",
		"printf '%s\\n' '{\"matches\":[{\"line\":6,\"column\":2,\"message\":\"request body is logged in a handler\",\"rationale\":\"the handler logs the request body through log.Printf\"}]}'",
	}, "\n")
	writeExecutableFile(t, runtimePath, script)
	return runtimePath
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
