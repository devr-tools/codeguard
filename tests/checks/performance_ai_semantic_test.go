package checks_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// performanceAISemanticConfig enables both the quality and performance
// sections plus the verdict cache, mirroring the quality semantic harness.
func performanceAISemanticConfig(dir string, name string) codeguard.Config {
	cfg := qualityAISemanticConfig(dir, name)
	cfg.Checks.Performance = boolPtr(true)
	return cfg
}

func performanceSemanticDiff() string {
	return stringsJoin(
		"diff --git a/service.go b/service.go",
		"--- a/service.go",
		"+++ b/service.go",
		"@@ -1,4 +1,5 @@",
		" package sample",
		" ",
		"+// BuildUser loads settings per call.",
		" func BuildUser() error {",
		" \treturn nil",
		" }",
	)
}

const performanceSemanticVerdicts = `{"verdicts":[` +
	`{"rule_id":"performance.ai.semantic-perf","path":"service.go","line":4,"message":"the same settings file is re-read and re-parsed on every call; load it once and cache the result"},` +
	`{"rule_id":"quality.ai.contract-drift","path":"service.go","line":3,"message":"comment describes settings loading the implementation does not perform"}]}`

func TestPerformanceSemanticFindingLandsInPerformanceSection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), "package sample\n\nfunc BuildUser() error {\n\treturn nil\n}\n")
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticScript(counterPath, performanceSemanticVerdicts))

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	cfg := performanceAISemanticConfig(dir, "performance-ai-semantic")
	for i := 0; i < 2; i++ {
		report, err := codeguard.RunPatch(context.Background(), cfg, performanceSemanticDiff())
		if err != nil {
			t.Fatalf("run patch %d: %v", i, err)
		}
		assertFindingRulePresent(t, report, "Performance", "performance.ai.semantic-perf")
		assertFindingLevel(t, report, "Performance", "performance.ai.semantic-perf", "warn")
		assertFindingRulePresent(t, report, "Code Quality", "quality.ai.contract-drift")
		assertSectionRuleAbsent(t, report, "Performance", "quality.ai.contract-drift")
		assertSectionRuleAbsent(t, report, "Code Quality", "performance.ai.semantic-perf")
	}

	// One combined request serves both sections and both scans: the quality
	// and performance lenses share a single runtime invocation (in-process
	// single-flight on the first scan, verdict cache on the second).
	assertFileEquals(t, counterPath, "1")
}

func TestPerformanceSemanticSkippedWhenPerformanceSectionIsOff(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), "package sample\n\nfunc BuildUser() error {\n\treturn nil\n}\n")
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	requestPath := filepath.Join(dir, "semantic-request.json")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticCaptureScript(counterPath, requestPath, performanceSemanticVerdicts))

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	cfg := performanceAISemanticConfig(dir, "performance-ai-semantic-off")
	cfg.Checks.Performance = nil

	report, err := codeguard.RunPatch(context.Background(), cfg, performanceSemanticDiff())
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}

	for _, section := range report.Sections {
		if section.Name == "Performance" {
			t.Fatal("performance section ran despite checks.performance being off")
		}
	}
	assertFindingRulePresent(t, report, "Code Quality", "quality.ai.contract-drift")
	assertSectionRuleAbsent(t, report, "Code Quality", "performance.ai.semantic-perf")
	assertFileEquals(t, counterPath, "1")

	// With the performance section off, the semantic request must be
	// byte-identical to previous releases: no performance lens rides along.
	data, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	if strings.Contains(string(data), "performance.ai.semantic-perf") {
		t.Fatal("semantic request includes the performance lens while checks.performance is off")
	}
}

func TestPerformanceSemanticRunsWithQualitySectionDisabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), "package sample\n\nfunc BuildUser() error {\n\treturn nil\n}\n")
	counterPath := filepath.Join(dir, "semantic-calls.txt")
	requestPath := filepath.Join(dir, "semantic-request.json")
	scriptPath := filepath.Join(dir, "semantic.sh")
	writeExecutableFile(t, scriptPath, semanticCaptureScript(counterPath, requestPath, performanceSemanticVerdicts))

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")
	t.Setenv("CODEGUARD_SEMANTIC_COMMAND", scriptPath)

	cfg := performanceAISemanticConfig(dir, "performance-ai-semantic-only")
	cfg.Checks.Quality = false

	report, err := codeguard.RunPatch(context.Background(), cfg, performanceSemanticDiff())
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.ai.semantic-perf")
	assertSectionRuleAbsent(t, report, "Performance", "quality.ai.contract-drift")
	for _, section := range report.Sections {
		if section.Name == "Code Quality" {
			t.Fatal("quality section ran despite checks.quality being false")
		}
	}
	assertFileEquals(t, counterPath, "1")
	assertPerformanceLensRequested(t, requestPath)
}

type capturedSemanticRequest struct {
	Checks []struct {
		RuleID string `json:"rule_id"`
	} `json:"checks"`
	Prompt struct {
		RuleInstructions []struct {
			RuleID string   `json:"rule_id"`
			Focus  string   `json:"focus"`
			Avoid  []string `json:"avoid"`
		} `json:"rule_instructions"`
	} `json:"prompt"`
}

// assertPerformanceLensRequested checks that the captured semantic request
// carries the performance check spec plus its structured prompt instruction.
func assertPerformanceLensRequested(t *testing.T, requestPath string) {
	t.Helper()
	data, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	var req capturedSemanticRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	assertPerformanceCheckSpecPresent(t, req)
	for _, instruction := range req.Prompt.RuleInstructions {
		if instruction.RuleID != "performance.ai.semantic-perf" {
			continue
		}
		if !strings.Contains(instruction.Focus, "cached or memoized") {
			t.Fatalf("performance prompt focus = %q, want caching/memoization guidance", instruction.Focus)
		}
		if !strings.Contains(strings.Join(instruction.Avoid, " "), "micro-optimizations") {
			t.Fatalf("performance prompt avoid list = %#v, want micro-optimization exclusion", instruction.Avoid)
		}
		return
	}
	t.Fatal("semantic request prompt missing the performance rule instruction")
}

func assertPerformanceCheckSpecPresent(t *testing.T, req capturedSemanticRequest) {
	t.Helper()
	for _, check := range req.Checks {
		if check.RuleID == "performance.ai.semantic-perf" {
			return
		}
	}
	t.Fatalf("semantic request checks missing performance lens: %#v", req.Checks)
}

func TestPerformanceSemanticEmitsRuntimeFindingWhenCommandIsMissing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.go"), "package sample\n\nfunc BuildUser() error {\n\treturn nil\n}\n")

	t.Setenv("CODEGUARD_SEMANTIC_CHECKS", "1")

	cfg := performanceAISemanticConfig(dir, "performance-ai-semantic-runtime-missing")
	cfg.Checks.Quality = false

	report, err := codeguard.RunPatch(context.Background(), cfg, performanceSemanticDiff())
	if err != nil {
		t.Fatalf("run patch: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.ai.semantic-runtime")
	assertFindingLevel(t, report, "Performance", "performance.ai.semantic-runtime", "fail")
}

func assertSectionRuleAbsent(t *testing.T, report codeguard.Report, section string, ruleID string) {
	t.Helper()
	for _, result := range report.Sections {
		if result.Name != section {
			continue
		}
		for _, finding := range result.Findings {
			if finding.RuleID == ruleID {
				t.Fatalf("section %q unexpectedly contains rule %q", section, ruleID)
			}
		}
	}
}
