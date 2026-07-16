package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestQualityCheckWarnsForAdditionalLanguageMaintainability(t *testing.T) {
	t.Parallel()

	for _, tc := range additionalLanguageMaintainabilityCases() {
		t.Run(tc.name, func(t *testing.T) {
			report := runAdditionalLanguageQualityCase(t, tc)
			assertSectionStatus(t, report, "Code Quality", "warn")
			assertFindingRulePresent(t, report, "Code Quality", "quality.max-function-lines")
			assertFindingRulePresent(t, report, "Code Quality", "quality.max-parameters")
			assertFindingRulePresent(t, report, "Code Quality", "quality.cyclomatic-complexity")
		})
	}
}

type additionalLanguageMaintainabilityCase struct {
	name     string
	language string
	path     string
	source   string
}

func additionalLanguageMaintainabilityCases() []additionalLanguageMaintainabilityCase {
	return []additionalLanguageMaintainabilityCase{
		{name: "python", language: "python", path: "pkg/example.py", source: "def sample(\n    a,\n    /,\n    b,\n    *,\n    c,\n):\n    if a and b:\n        return c\n    return b\n"},
		{name: "rust", language: "rust", path: "src/lib.rs", source: "pub fn sample(a: i32, b: i32, c: i32) -> i32 {\n    if a > 0 { return b; }\n    if b > 0 { return c; }\n    if c > 0 { return a; }\n    a + b + c\n}\n"},
		{name: "java", language: "java", path: "src/main/java/Sample.java", source: "class Sample {\n    public int sample(int a, int b, int c) {\n        if (a > 0) { return b; }\n        if (b > 0) { return c; }\n        if (c > 0) { return a; }\n        return a + b + c;\n    }\n}\n"},
		{name: "cpp", language: "c++", path: "src/sample.cpp", source: "int sample(int a, int b, int c) {\n    if (a > 0) { return b; }\n    if (b > 0) { return c; }\n    if (c > 0) { return a; }\n    return a + b + c;\n}\n"},
		{name: "csharp", language: "csharp", path: "src/Sample.cs", source: "public class Sample {\n    public int Run(int a, int b, int c) {\n        if (a > 0) { return b; }\n        if (b > 0) { return c; }\n        if (c > 0) { return a; }\n        return a + b + c;\n    }\n}\n"},
		{name: "ruby", language: "ruby", path: "app/sample.rb", source: "def sample(a, b, c)\n  if a\n    return b\n  end\n  if b\n    return c\n  end\n  if c\n    return a\n  end\n  a + b + c\nend\n"},
	}
}

func runAdditionalLanguageQualityCase(t *testing.T, tc additionalLanguageMaintainabilityCase) codeguard.Report {
	t.Helper()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, filepath.FromSlash(tc.path)), tc.source)

	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-" + tc.name + "-native"
	cfg.Targets = []codeguard.TargetConfig{{Name: tc.name, Path: dir, Language: tc.language}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.QualityRules.MaxFunctionLines = 4
	cfg.Checks.QualityRules.MaxParameters = 2
	cfg.Checks.QualityRules.MaxCyclomaticComplexity = 2

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}
