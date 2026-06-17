package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualitySemanticChecksRunWithoutProvenanceWhenSemanticRuntimeIsConfigured(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), "package sample\n\nfunc BuildUser() error {\n\treturn nil\n}\n")
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
	writeFile(t, filepath.Join(dir, "service.go"), "package sample\n\nfunc BuildUser() error {\n\treturn nil\n}\n")
	writeFile(t, filepath.Join(dir, "service_test.go"), "package sample\n\nimport \"testing\"\n\nfunc TestBuildUser(t *testing.T) {}\n")
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

func TestQualitySemanticChecksEmitTestAdequacyVerdict(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), "package sample\n\nfunc BuildUser() error {\n\treturn nil\n}\n")
	writeFile(t, filepath.Join(dir, "service_test.go"), "package sample\n\nimport \"testing\"\n\nfunc TestBuildUser(t *testing.T) {}\n")
	diff := stringsJoin(
		"diff --git a/service.go b/service.go",
		"--- a/service.go",
		"+++ b/service.go",
		"@@ -1,4 +1,4 @@",
		" package sample",
		" ",
		" func BuildUser() error {",
		"-\treturn nil",
		"+\treturn errors.New(\"conflict\")",
		" }",
	)
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticScript(counterPath, `{"verdicts":[{"rule_id":"quality.ai.semantic-test-adequacy","path":"service.go","line":3,"message":"tests appear inadequate for changed behavior: [happy-path-only] [missing-negative-path] nearby tests only cover the success path"}]}`))

	t.Setenv("CODEGUARD_AI_ASSISTED", "true")
	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	report, err := codeguard.RunPatch(context.Background(), qualityAISemanticConfig(dir, "quality-ai-semantic-adequacy"), diff)
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.semantic-test-adequacy")
	assertFileEquals(t, counterPath, "1")
}

func TestQualitySemanticChecksEmitContractDriftVerdict(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), "package sample\n\nfunc BuildUser() error {\n\treturn nil\n}\n")
	diff := stringsJoin(
		"diff --git a/service.go b/service.go",
		"--- a/service.go",
		"+++ b/service.go",
		"@@ -1,4 +1,5 @@",
		" package sample",
		" ",
		"+// BuildUser creates a new user.",
		" func BuildUser() error {",
		"-\treturn nil",
		"+\treturn errors.New(\"user deleted\")",
		" }",
	)
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticScript(counterPath, `{"verdicts":[{"rule_id":"quality.ai.contract-drift","path":"service.go","line":3,"message":"function behavior appears to drift from the existing create-user contract without matching caller or test updates"}]}`))

	t.Setenv("CODEGUARD_AI_ASSISTED", "true")
	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	report, err := codeguard.RunPatch(context.Background(), qualityAISemanticConfig(dir, "quality-ai-contract-drift"), diff)
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.contract-drift")
	assertFileEquals(t, counterPath, "1")
}

func TestQualitySemanticChecksRunInFullScanUsingGitBaseRef(t *testing.T) {
	dir := t.TempDir()
	runSemanticGit(t, dir, "init", "-b", "main")
	runSemanticGit(t, dir, "config", "user.email", "test@example.com")
	runSemanticGit(t, dir, "config", "user.name", "CodeGuard Test")
	writeFile(t, filepath.Join(dir, "service.go"), "package sample\n\nfunc BuildUser() error {\n\treturn nil\n}\n")
	runSemanticGit(t, dir, "add", ".")
	runSemanticGit(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "service.go"), "package sample\n\n// BuildUser removes a user.\nfunc BuildUser() error {\n\treturn nil\n}\n")
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
	writeFile(t, filepath.Join(dir, "service.go"), "package sample\n\nfunc BuildUser() error {\n\treturn nil\n}\n")
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

func TestQualitySemanticChecksEmitFindingWhenSemanticCommandFails(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), "package sample\n\nfunc BuildUser() error {\n\treturn nil\n}\n")
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
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, "#!/bin/sh\ncat >/dev/null\necho 'semantic backend exploded' >&2\nexit 2\n")

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	report, err := codeguard.RunPatch(context.Background(), qualityAISemanticConfig(dir, "quality-ai-semantic-runtime-failure"), diff)
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.semantic-runtime")
	assertFindingLevel(t, report, "Code Quality", "quality.ai.semantic-runtime", "fail")
	assertSemanticRuntimeMessageContains(t, report, "semantic backend exploded")
}

func TestQualitySemanticChecksEmitFindingWhenSemanticCommandIsMissing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), "package sample\n\nfunc BuildUser() error {\n\treturn nil\n}\n")
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

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")

	report, err := codeguard.RunPatch(context.Background(), qualityAISemanticConfig(dir, "quality-ai-semantic-runtime-missing"), diff)
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.semantic-runtime")
	assertFindingLevel(t, report, "Code Quality", "quality.ai.semantic-runtime", "fail")
	assertSemanticRuntimeMessageContains(t, report, "no semantic command is configured")
}

func assertSemanticRuntimeMessageContains(t *testing.T, report codeguard.Report, want string) {
	t.Helper()
	for _, section := range report.Sections {
		if section.Name != "Code Quality" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == "quality.ai.semantic-runtime" {
				if !strings.Contains(finding.Message, want) {
					t.Fatalf("semantic runtime message = %q, want substring %q", finding.Message, want)
				}
				return
			}
		}
	}
	t.Fatal("quality.ai.semantic-runtime finding not found")
}
