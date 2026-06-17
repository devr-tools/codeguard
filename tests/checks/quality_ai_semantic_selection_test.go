package checks_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

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
	cfg.AI.Semantic.ContractDrift = &enabled
	cfg.AI.Semantic.MisleadingErrorMessages = &enabled
	cfg.AI.Semantic.TestBehaviorCoverage = &enabled
	cfg.AI.Semantic.TestAdequacy = &enabled

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

func TestQualitySemanticChecksCanSelectOnlyTestAdequacy(t *testing.T) {
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
		"@@ -1,4 +1,4 @@",
		" package sample",
		" ",
		" func BuildUser() error {",
		"-\treturn nil",
		"+\treturn errors.New(\"conflict\")",
		" }",
	)
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	requestPath := filepath.Join(dir, "semantic-request.json")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticCaptureScript(counterPath, requestPath, `{"verdicts":[{"rule_id":"quality.ai.semantic-test-adequacy","path":"service.go","line":3,"message":"tests appear inadequate for changed behavior: [risky-change-without-matching-test]"}]}`))

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	cfg := qualityAISemanticConfig(dir, "quality-ai-semantic-adequacy-selection")
	disabled := false
	cfg.AI.Semantic.FunctionContract = &disabled
	cfg.AI.Semantic.ContractDrift = &disabled
	cfg.AI.Semantic.MisleadingErrorMessages = &disabled
	cfg.AI.Semantic.TestBehaviorCoverage = &disabled

	report, err := codeguard.RunPatch(context.Background(), cfg, diff)
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.semantic-test-adequacy")

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
	if len(req.Checks) != 1 || req.Checks[0].RuleID != "quality.ai.semantic-test-adequacy" {
		t.Fatalf("semantic checks = %#v, want only test-adequacy", req.Checks)
	}
}

func TestQualitySemanticChecksCanSelectOnlyContractDrift(t *testing.T) {
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
		"@@ -1,4 +1,4 @@",
		" package sample",
		" ",
		" func BuildUser() error {",
		"-\treturn nil",
		"+\treturn errors.New(\"user deleted\")",
		" }",
	)
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	requestPath := filepath.Join(dir, "semantic-request.json")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticCaptureScript(counterPath, requestPath, `{"verdicts":[{"rule_id":"quality.ai.contract-drift","path":"service.go","line":3,"message":"behavior appears to drift from the existing contract"}]}`))

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	cfg := qualityAISemanticConfig(dir, "quality-ai-contract-drift-selection")
	disabled := false
	cfg.AI.Semantic.FunctionContract = &disabled
	cfg.AI.Semantic.MisleadingErrorMessages = &disabled
	cfg.AI.Semantic.TestBehaviorCoverage = &disabled
	cfg.AI.Semantic.TestAdequacy = &disabled

	report, err := codeguard.RunPatch(context.Background(), cfg, diff)
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}

	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.contract-drift")

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
	if len(req.Checks) != 1 || req.Checks[0].RuleID != "quality.ai.contract-drift" {
		t.Fatalf("semantic checks = %#v, want only contract-drift", req.Checks)
	}
}
