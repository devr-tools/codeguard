package checks_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualitySemanticChecksRunWithoutProvenanceWhenSemanticRuntimeIsConfigured(t *testing.T) {
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

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.semantic-doc-mismatch")
	assertFileEquals(t, counterPath, "1")
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

func TestQualitySemanticChecksRunInFullScanUsingGitBaseRef(t *testing.T) {
	dir := t.TempDir()
	runSemanticGit(t, dir, "init", "-b", "main")
	runSemanticGit(t, dir, "config", "user.email", "test@example.com")
	runSemanticGit(t, dir, "config", "user.name", "CodeGuard Test")
	writeFile(t, filepath.Join(dir, "service.go"), `package sample

func BuildUser() error {
	return nil
}
`)
	runSemanticGit(t, dir, "add", ".")
	runSemanticGit(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "service.go"), `package sample

// BuildUser removes a user.
func BuildUser() error {
	return nil
}
`)
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticScript(counterPath, `{"verdicts":[{"rule_id":"quality.ai.semantic-doc-mismatch","path":"service.go","line":3,"message":"comment and implementation disagree"}]}`))

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	report, err := codeguard.Run(context.Background(), qualityAISemanticConfig(dir, "quality-ai-semantic-full"))
	if err != nil {
		t.Fatalf("run full scan: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.semantic-doc-mismatch")
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

func TestQualitySemanticChecksHonorRuleSelection(t *testing.T) {
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
		"@@ -1,4 +1,5 @@",
		" package sample",
		" ",
		"+// BuildUser removes a user.",
		" func BuildUser() error {",
		" \treturn nil",
		" }",
	)
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	requestPath := filepath.Join(dir, "semantic-request.json")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticCaptureScript(counterPath, requestPath, `{"verdicts":[{"rule_id":"quality.ai.semantic-doc-mismatch","path":"service.go","line":3,"message":"comment and implementation disagree"}]}`))

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	cfg := qualityAISemanticConfig(dir, "quality-ai-semantic-selection")
	enabled := false
	cfg.AI.Semantic.MisleadingErrorMessages = &enabled
	cfg.AI.Semantic.TestBehaviorCoverage = &enabled

	report, err := codeguard.RunPatch(context.Background(), cfg, diff)
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.semantic-doc-mismatch")
	assertFileEquals(t, counterPath, "1")

	var req struct {
		Checks []struct {
			RuleID string `json:"rule_id"`
		} `json:"checks"`
	}
	data, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if len(req.Checks) != 1 || req.Checks[0].RuleID != "quality.ai.semantic-doc-mismatch" {
		t.Fatalf("semantic checks = %#v, want only doc-mismatch", req.Checks)
	}
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

func semanticCaptureScript(counterPath string, requestPath string, response string) string {
	return "#!/bin/sh\n" +
		"count=0\n" +
		"if [ -f \"" + counterPath + "\" ]; then count=$(cat \"" + counterPath + "\"); fi\n" +
		"count=$((count + 1))\n" +
		"printf \"%s\" \"$count\" > \"" + counterPath + "\"\n" +
		"cat >\"" + requestPath + "\"\n" +
		"printf '%s' '" + response + "'\n"
}

func runSemanticGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
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

func stringsJoin(lines ...string) string {
	return strings.Join(lines, "\n")
}
