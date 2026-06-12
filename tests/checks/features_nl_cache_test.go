package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestNaturalLanguageRuleVerdictCacheSkipsRuntimeReinvocation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handlers", "login.go"), "package handlers\n\nimport \"log\"\n\nfunc handleLogin(body string) {\n\tlog.Printf(\"body=%s\", body)\n}\n")

	countPath := filepath.Join(dir, "runtime-count.txt")
	runtimePath := writeCountingNLRuleRuntime(t, dir, countPath)
	t.Setenv("CODEGUARD_AI_RUNTIME_COMMAND", runtimePath)

	cfg := codeguard.ExampleConfig()
	cfg.Name = "custom-nl-verdict-cache"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Design = false
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cacheEnabled := true
	cfg.Cache = codeguard.CacheConfig{
		Enabled: &cacheEnabled,
		Path:    filepath.Join(dir, ".codeguard", "cache.json"),
	}
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
		t.Fatalf("first run: %v", err)
	}
	assertSectionStatus(t, report, "Custom Rules", "fail")
	if got := countRuntimeInvocations(t, countPath); got != 1 {
		t.Fatalf("expected 1 runtime invocation after first run, got %d", got)
	}

	// Second run with an unchanged file and unchanged rule must not
	// re-invoke the runtime.
	report, err = codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	assertSectionStatus(t, report, "Custom Rules", "fail")
	if got := countRuntimeInvocations(t, countPath); got != 1 {
		t.Fatalf("expected runtime to stay at 1 invocation across rerun, got %d", got)
	}

	// A config change that does not touch the NL rule invalidates the
	// file-level scan cache, but the per-verdict cache must still serve the
	// stored verdict without re-invoking the runtime.
	cfg.RulePacks[0].Rules = append(cfg.RulePacks[0].Rules, codeguard.CustomRuleConfig{
		ID:           "custom.unrelated-regex-rule",
		Title:        "Unrelated rule",
		Severity:     "warn",
		Message:      "unrelated",
		ContentRegex: "string-that-never-appears-anywhere",
		Paths:        []string{"handlers/**"},
	})
	report, err = codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("third run: %v", err)
	}
	assertSectionStatus(t, report, "Custom Rules", "fail")
	if got := countRuntimeInvocations(t, countPath); got != 1 {
		t.Fatalf("expected per-verdict cache hit after config change, got %d invocations", got)
	}

	data, err := os.ReadFile(cfg.Cache.Path)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	if !strings.Contains(string(data), "\"nl_rule_verdicts\"") {
		t.Fatalf("expected nl_rule_verdicts in cache file, got %s", string(data))
	}
}

func writeCountingNLRuleRuntime(t *testing.T, dir string, countPath string) string {
	t.Helper()
	runtimePath := filepath.Join(dir, "counting-nl-runtime.sh")
	script := strings.Join([]string{
		"#!/bin/sh",
		"cat >/dev/null",
		"echo x >> \"" + countPath + "\"",
		"printf '%s\\n' '{\"matches\":[{\"line\":6,\"column\":2,\"message\":\"request body is logged in a handler\",\"rationale\":\"the handler logs the request body through log.Printf\"}]}'",
	}, "\n")
	writeExecutableFile(t, runtimePath, script)
	return runtimePath
}

func countRuntimeInvocations(t *testing.T, countPath string) int {
	t.Helper()
	data, err := os.ReadFile(countPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read count file: %v", err)
	}
	return len(strings.Split(strings.TrimSpace(string(data)), "\n"))
}
