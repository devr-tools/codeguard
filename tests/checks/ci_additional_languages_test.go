package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestCICheckHandlesAdditionalLanguageTestPaths(t *testing.T) {
	t.Parallel()

	for _, tc := range ciAdditionalLanguageCases() {
		t.Run(tc.name+"-fail", func(t *testing.T) {
			report := runCITestPathCase(t, tc, false)
			assertSectionStatus(t, report, "CI/CD", "fail")
		})

		t.Run(tc.name+"-pass", func(t *testing.T) {
			report := runCITestPathCase(t, tc, true)
			assertSectionStatus(t, report, "CI/CD", "pass")
		})
	}
}

type ciAdditionalLanguageCase struct {
	name        string
	language    string
	failPath    string
	failAllowed []string
	passPath    string
	passAllowed []string
}

func ciAdditionalLanguageCases() []ciAdditionalLanguageCase {
	return []ciAdditionalLanguageCase{
		{name: "rust", language: "rust", failPath: "tests/sample.rs", failAllowed: []string{"qa/**"}, passPath: "tests/sample.rs", passAllowed: []string{"tests/**"}},
		{name: "java", language: "java", failPath: "src/test/java/SampleTest.java", failAllowed: []string{"tests/**"}, passPath: "tests/java/SampleTest.java", passAllowed: []string{"tests/**"}},
		{name: "csharp", language: "csharp", failPath: "src/WidgetTests.cs", failAllowed: []string{"tests/**"}, passPath: "tests/WidgetTests.cs", passAllowed: []string{"tests/**"}},
		{name: "ruby", language: "ruby", failPath: "spec/sample_spec.rb", failAllowed: []string{"tests/**"}, passPath: "tests/sample_test.rb", passAllowed: []string{"tests/**"}},
	}
}

func runCITestPathCase(t *testing.T, tc ciAdditionalLanguageCase, pass bool) codeguard.Report {
	t.Helper()

	testPath := tc.failPath
	allowed := tc.failAllowed
	suffix := "fail"
	if pass {
		testPath = tc.passPath
		allowed = tc.passAllowed
		suffix = "pass"
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, filepath.FromSlash(testPath)), "test artifact\n")

	cfg := codeguard.ExampleConfig()
	cfg.Name = "ci-" + tc.name + "-test-location-" + suffix
	cfg.Targets = []codeguard.TargetConfig{{Name: tc.name, Path: dir, Language: tc.language}}
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
	cfg.Checks.CIRules.AllowedTestPaths = allowed

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}
