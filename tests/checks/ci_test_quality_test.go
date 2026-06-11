package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestCICheckFailsForGoTestsWithoutAssertions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tests", "sample_test.go"), "package sample_test\n\nimport \"testing\"\n\nfunc TestSample(t *testing.T) {\n\tvalue := 1 + 1\n\t_ = value\n}\n")

	report := runTestQualityCICheck(t, dir, "go")

	assertSectionStatus(t, report, "CI/CD", "fail")
	assertFindingRulePresent(t, report, "CI/CD", "ci.test-without-assertion")
}

func TestCICheckFailsForAlwaysTruePythonAssertions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tests", "test_sample.py"), "def test_sample():\n    assert True\n")

	report := runTestQualityCICheck(t, dir, "python")

	assertSectionStatus(t, report, "CI/CD", "fail")
	assertFindingRulePresent(t, report, "CI/CD", "ci.always-true-test-assertion")
}

func TestCICheckIgnoresCommentedAssertionTokensAndPassesRealTypeScriptAssertions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tests", "sample.test.ts"), "test('sample', () => {\n  // expect(value).toBe(2)\n  expect(sum(1, 1)).toBe(2)\n})\n")

	report := runTestQualityCICheck(t, dir, "typescript")

	assertSectionStatus(t, report, "CI/CD", "pass")
}

func runTestQualityCICheck(t *testing.T, dir string, language string) codeguard.Report {
	t.Helper()

	cfg := codeguard.ExampleConfig()
	cfg.Name = "ci-test-quality-" + language
	cfg.Targets = []codeguard.TargetConfig{{Name: language, Path: dir, Language: language}}
	cfg.Checks.CI = true
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	disabled := false
	cfg.Checks.CIRules.RequireWorkflowDir = &disabled
	cfg.Checks.CIRules.RequiredWorkflowFiles = []string{}
	cfg.Checks.CIRules.RequiredReleaseFiles = []string{}
	cfg.Checks.CIRules.RequiredAutomationPaths = []string{}
	cfg.Checks.CIRules.WorkflowContentRules = []codeguard.WorkflowRuleConfig{}
	cfg.Checks.CIRules.AllowedTestPaths = []string{"tests/**"}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}
