package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func testQualityConfig(t *testing.T, dir string, language string) codeguard.Config {
	t.Helper()
	cfg := codeguard.ExampleConfig()
	cfg.Name = "test-quality"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = true
	cfg.Checks.CIRules.RequireWorkflowDir = boolValue(false)
	cfg.Checks.CIRules.RequiredWorkflowFiles = []string{}
	cfg.Checks.CIRules.WorkflowContentRules = []codeguard.WorkflowRuleConfig{}
	cfg.Checks.CIRules.RequiredReleaseFiles = []string{}
	cfg.Checks.CIRules.RequiredAutomationPaths = []string{}
	cfg.Checks.CIRules.AllowedTestPaths = []string{}
	cfg.Cache.Enabled = boolValue(false)
	return cfg
}

func runScan(t *testing.T, cfg codeguard.Config) codeguard.Report {
	t.Helper()
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

func findingsForRule(report codeguard.Report, ruleID string) []codeguard.Finding {
	matches := make([]codeguard.Finding, 0)
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID {
				matches = append(matches, finding)
			}
		}
	}
	return matches
}

func assertRuleCount(t *testing.T, report codeguard.Report, ruleID string, want int) {
	t.Helper()
	got := findingsForRule(report, ruleID)
	if len(got) != want {
		t.Fatalf("%s findings = %d, want %d: %+v", ruleID, len(got), want, got)
	}
}

func boolValue(v bool) *bool {
	return &v
}

func TestGoTestQualityRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "demo_test.go"), `package demo

import "testing"

func TestNoAssertion(t *testing.T) {
	value := compute()
	_ = value
}

func TestAlwaysTrue(t *testing.T) {
	require.True(t, true)
}

func TestConditionalAssert(t *testing.T) {
	value := compute()
	if value > 0 {
		assert.Equal(t, value, 5)
	}
}

func TestIdiomaticConditionalFatal(t *testing.T) {
	value := compute()
	if value != 5 {
		t.Fatalf("value = %d", value)
	}
}

func TestUnconditionalAssert(t *testing.T) {
	assert.Equal(t, compute(), 5)
}
`)

	report := runScan(t, testQualityConfig(t, dir, "go"))

	assertRuleCount(t, report, "ci.test-without-assertion", 1)
	assertRuleCount(t, report, "ci.always-true-test-assertion", 1)
	assertRuleCount(t, report, "ci.conditional-assertion", 1)

	noAssert := findingsForRule(report, "ci.test-without-assertion")[0]
	if noAssert.Line != 5 {
		t.Fatalf("test-without-assertion line = %d, want 5", noAssert.Line)
	}
}

func TestGoTestQualityCustomAssertionHelpers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "helper_test.go"), `package demo

import "testing"

func TestWithCustomHelper(t *testing.T) {
	assertValid(t, compute())
}
`)

	cfg := testQualityConfig(t, dir, "go")
	report := runScan(t, cfg)
	assertRuleCount(t, report, "ci.test-without-assertion", 1)

	cfg.Checks.CIRules.TestQuality.AssertionHelpers = []string{"assertValid"}
	report = runScan(t, cfg)
	assertRuleCount(t, report, "ci.test-without-assertion", 0)
	assertRuleCount(t, report, "ci.always-true-test-assertion", 0)
	assertRuleCount(t, report, "ci.conditional-assertion", 0)
}

func TestGoTestQualityDisabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "off_test.go"), `package demo

import "testing"

func TestNoAssertion(t *testing.T) {
	_ = compute()
}
`)

	cfg := testQualityConfig(t, dir, "go")
	cfg.Checks.CIRules.TestQuality.Enabled = boolValue(false)
	report := runScan(t, cfg)
	assertRuleCount(t, report, "ci.test-without-assertion", 0)
}

func TestTypeScriptTestQualityRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.test.ts"), `import { compute } from './app';

it('does nothing', () => {
  const value = compute();
});

it('asserts a constant', () => {
  expect(true).toBe(true);
});

it('asserts conditionally', () => {
  const value = compute();
  if (value > 0) {
    expect(value).toBe(5);
  }
});

it('asserts properly', () => {
  expect(compute()).toBe(5);
});

it('asserts in both branches', () => {
  if (compute() > 0) {
    expect(compute()).toBe(5);
  } else {
    expect(compute()).toBe(0);
  }
});
`)

	report := runScan(t, testQualityConfig(t, dir, "typescript"))

	assertRuleCount(t, report, "ci.test-without-assertion", 1)
	assertRuleCount(t, report, "ci.always-true-test-assertion", 1)
	assertRuleCount(t, report, "ci.conditional-assertion", 1)
}

func TestPythonTestQualityRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test_app.py"), `from app import compute


def test_no_assertion():
    value = compute()


def test_always_true():
    assert 1 == 1


def test_conditional_assert():
    value = compute()
    if value > 0:
        assert value == 5


def test_proper():
    assert compute() == 5


def test_conditional_with_else():
    if compute() > 0:
        assert compute() == 5
    else:
        assert compute() == 0
`)

	report := runScan(t, testQualityConfig(t, dir, "python"))

	assertRuleCount(t, report, "ci.test-without-assertion", 1)
	assertRuleCount(t, report, "ci.always-true-test-assertion", 1)
	assertRuleCount(t, report, "ci.conditional-assertion", 1)
}
