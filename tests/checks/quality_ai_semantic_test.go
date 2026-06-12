package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualitySemanticChecksRequireAIGate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), `package sample

func BuildUser() error {
	return nil
}
`)
	diff := stringsJoin(
		"diff --git a/service.go b/service.go",
		"--- a/service.go",
		"+++ b/service.go",
		"@@ -1,5 +1,7 @@",
		" package sample",
		"+",
		" ",
		"+// BuildUser removes a user.",
		" func BuildUser() error {",
		" \treturn nil",
		" }",
	)
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticScript(counterPath, `{"verdicts":[{"rule_id":"quality.ai.semantic-doc-mismatch","path":"service.go","line":3,"message":"comment and implementation disagree"}]}`))

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	report, err := codeguard.RunPatch(context.Background(), qualityAISemanticConfig(dir, "quality-ai-semantic-gated"), diff)
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}

	assertRuleAbsentAnywhere(t, report, "quality.ai.semantic-doc-mismatch")
	assertFileMissing(t, counterPath)
}

func TestQualitySemanticChecksEmitVerdictsForAIAssistedPatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), `package sample

func BuildUser() error {
	return nil
}
`)
	writeFile(t, filepath.Join(dir, "service_test.go"), `package sample

import "testing"

func TestBuildUser(t *testing.T) {}
`)
	diff := stringsJoin(
		"diff --git a/service.go b/service.go",
		"--- a/service.go",
		"+++ b/service.go",
		"@@ -1,5 +1,7 @@",
		" package sample",
		"+",
		" ",
		"+// BuildUser removes a user.",
		" func BuildUser() error {",
		"-\treturn nil",
		"+\treturn errors.New(\"user created\")",
		" }",
	)
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticScript(counterPath, `{"verdicts":[{"rule_id":"quality.ai.semantic-doc-mismatch","path":"service.go","line":3,"message":"comment says removal but implementation builds a user"},{"rule_id":"quality.ai.semantic-error-message","path":"service.go","line":5,"message":"error says user created on a failure path"},{"rule_id":"quality.ai.semantic-test-coverage","path":"service.go","line":4,"message":"tests do not appear to exercise the changed failure behavior"}]}`))

	t.Setenv("CODEGUARD_AI_ASSISTED", "true")
	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	report, err := codeguard.RunPatch(context.Background(), qualityAISemanticConfig(dir, "quality-ai-semantic"), diff)
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.semantic-doc-mismatch")
	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.semantic-error-message")
	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.semantic-test-coverage")
	assertFileEquals(t, counterPath, "1")
}

func TestQualitySemanticChecksUseVerdictCache(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), `package sample

func BuildUser() error {
	return nil
}
`)
	diff := stringsJoin(
		"diff --git a/service.go b/service.go",
		"index 1111111..2222222 100644",
		"--- a/service.go",
		"+++ b/service.go",
		"@@ -1,4 +1,4 @@",
		" package sample",
		" ",
		" func BuildUser() error {",
		"-\treturn nil",
		"+\treturn nil",
		" }",
	)
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticScript(counterPath, `{"verdicts":[{"rule_id":"quality.ai.semantic-test-coverage","path":"service.go","line":3,"message":"tests do not appear to exercise the changed behavior"}]}`))

	t.Setenv("CODEGUARD_AI_ASSISTED", "true")
	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	cfg := qualityAISemanticConfig(dir, "quality-ai-semantic-cache")
	for i := 0; i < 2; i++ {
		report, err := codeguard.RunPatch(context.Background(), cfg, diff)
		if err != nil {
			t.Fatalf("run patch %d: %v", i, err)
		}
		assertFindingRulePresent(t, report, "Code Quality", "quality.ai.semantic-test-coverage")
	}

	assertFileEquals(t, counterPath, "1")
}

func qualityAISemanticConfig(dir string, name string) codeguard.Config {
	cfg := qualityAITestConfig(dir, name)
	enabled := true
	cfg.Cache.Enabled = &enabled
	cfg.Cache.Path = filepath.Join(dir, ".codeguard", "cache.json")
	return cfg
}

func semanticScript(counterPath string, response string) string {
	return "#!/bin/sh\n" +
		"count=0\n" +
		"if [ -f \"" + counterPath + "\" ]; then count=$(cat \"" + counterPath + "\"); fi\n" +
		"count=$((count + 1))\n" +
		"printf \"%s\" \"$count\" > \"" + counterPath + "\"\n" +
		"cat >/dev/null\n" +
		"printf '%s' '" + response + "'\n"
}

func assertRuleAbsentAnywhere(t *testing.T, report codeguard.Report, ruleID string) {
	t.Helper()
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID {
				t.Fatalf("unexpected finding %s in report %#v", ruleID, report)
			}
		}
	}
}

func assertFileEquals(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", path, string(data), want)
	}
}

func assertFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, err=%v", path, err)
	}
}

func stringsJoin(lines ...string) string {
	return strings.Join(lines, "\n")
}
