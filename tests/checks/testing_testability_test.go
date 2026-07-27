package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestTestingBehaviorChangeWithoutTestAcrossLanguages(t *testing.T) {
	cases := []struct {
		name     string
		language string
		path     string
		before   string
		after    string
	}{
		{
			name:     "go",
			language: "go",
			path:     "pricing/price.go",
			before:   "package pricing\n\nfunc Price(v int) int {\n\treturn v\n}\n",
			after:    "package pricing\n\nfunc Price(v int) int {\n\tif v > 100 {\n\t\treturn v - 10\n\t}\n\treturn v\n}\n",
		},
		{
			name:     "python",
			language: "python",
			path:     "app/services/pricing.py",
			before:   "def price(value):\n    return value\n",
			after:    "def price(value):\n    if value > 100:\n        return value - 10\n    return value\n",
		},
		{
			name:     "typescript",
			language: "typescript",
			path:     "src/domain/pricing.ts",
			before:   "export function price(value: number) {\n  return value;\n}\n",
			after:    "export function price(value: number) {\n  if (value > 100) {\n    return value - 10;\n  }\n  return value;\n}\n",
		},
		{
			name:     "javascript",
			language: "javascript",
			path:     "src/domain/pricing.js",
			before:   "export function price(value) {\n  return value;\n}\n",
			after:    "export function price(value) {\n  if (value > 100) {\n    return value - 10;\n  }\n  return value;\n}\n",
		},
		{
			name:     "cpp",
			language: "c++",
			path:     "src/domain/pricing.cpp",
			before:   "int price(int value) {\n  return value;\n}\n",
			after:    "int price(int value) {\n  if (value > 100) {\n    return value - 10;\n  }\n  return value;\n}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := testingGitRepo(t)
			writeFile(t, filepath.Join(dir, tc.path), tc.before)
			commitAll(t, dir, "base")
			writeFile(t, filepath.Join(dir, tc.path), tc.after)

			report := runTestingChangeScan(t, testingChangeConfig(t, dir, tc.language))

			assertFindingRulePresent(t, report, "Change Safety", "testing.behavior-change-without-test")
			assertFindingLevel(t, report, "Change Safety", "testing.behavior-change-without-test", "fail")
		})
	}
}

func TestTestingBehaviorChangeWithChangedTestSuppressesFinding(t *testing.T) {
	dir := testingGitRepo(t)
	writeFile(t, filepath.Join(dir, "pricing", "price.go"), "package pricing\n\nfunc Price(v int) int {\n\treturn v\n}\n")
	writeFile(t, filepath.Join(dir, "pricing", "price_test.go"), "package pricing\n\nimport \"testing\"\n\nfunc TestPrice(t *testing.T) {\n\tif Price(10) != 10 { t.Fatal(\"price\") }\n}\n")
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "pricing", "price.go"), "package pricing\n\nfunc Price(v int) int {\n\tif v > 100 {\n\t\treturn v - 10\n\t}\n\treturn v\n}\n")
	writeFile(t, filepath.Join(dir, "pricing", "price_test.go"), "package pricing\n\nimport \"testing\"\n\nfunc TestPriceDiscount(t *testing.T) {\n\tif Price(120) != 110 { t.Fatal(\"discount\") }\n}\n")

	report := runTestingChangeScan(t, testingChangeConfig(t, dir, "go"))

	assertFindingRuleAbsent(t, report, "Change Safety", "testing.behavior-change-without-test")
}

func TestTestingFailurePathMissingRequiresFailureTestEvidence(t *testing.T) {
	t.Run("missing failure test", func(t *testing.T) {
		dir := testingGitRepo(t)
		writeFile(t, filepath.Join(dir, "src", "domain", "payment.ts"), "export function authorize(ok: boolean) {\n  return ok;\n}\n")
		commitAll(t, dir, "base")
		writeFile(t, filepath.Join(dir, "src", "domain", "payment.ts"), "export function authorize(ok: boolean) {\n  if (!ok) {\n    throw new Error('denied');\n  }\n  return true;\n}\n")

		cfg := testingChangeConfig(t, dir, "typescript")
		cfg.Checks.ChangeRules.DetectBehaviorChangeWithoutTest = boolValue(false)
		report := runTestingChangeScan(t, cfg)

		assertFindingRulePresent(t, report, "Change Safety", "testing.failure-path-missing")
	})

	t.Run("covered by failure test", func(t *testing.T) {
		dir := testingGitRepo(t)
		writeFile(t, filepath.Join(dir, "src", "domain", "payment.ts"), "export function authorize(ok: boolean) {\n  return ok;\n}\n")
		writeFile(t, filepath.Join(dir, "src", "domain", "payment.test.ts"), "import { authorize } from './payment';\n\ntest('authorize allows success', () => {\n  expect(authorize(true)).toBe(true);\n});\n")
		commitAll(t, dir, "base")
		writeFile(t, filepath.Join(dir, "src", "domain", "payment.ts"), "export function authorize(ok: boolean) {\n  if (!ok) {\n    throw new Error('denied');\n  }\n  return true;\n}\n")
		writeFile(t, filepath.Join(dir, "src", "domain", "payment.test.ts"), "import { authorize } from './payment';\n\ntest('authorize rejects denied payment', () => {\n  expect(() => authorize(false)).toThrow('denied');\n});\n")

		cfg := testingChangeConfig(t, dir, "typescript")
		cfg.Checks.ChangeRules.DetectBehaviorChangeWithoutTest = boolValue(false)
		report := runTestingChangeScan(t, cfg)

		assertFindingRuleAbsent(t, report, "Change Safety", "testing.failure-path-missing")
	})
}

func TestTestingHardwiredDependencyFindsChangedProductionLine(t *testing.T) {
	dir := testingGitRepo(t)
	writeFile(t, filepath.Join(dir, "app", "services", "profile.py"), "def load_profile(user_id):\n    return {\"id\": user_id}\n")
	commitAll(t, dir, "base")
	writeFile(t, filepath.Join(dir, "app", "services", "profile.py"), "import requests\n\n\ndef load_profile(user_id):\n    response = requests.get(f\"https://profiles.example/{user_id}\")\n    return response.json()\n")

	cfg := testingChangeConfig(t, dir, "python")
	cfg.Checks.ChangeRules.DetectBehaviorChangeWithoutTest = boolValue(false)
	report := runTestingChangeScan(t, cfg)

	assertFindingRulePresent(t, report, "Change Safety", "testing.hardwired-dependency")
}

func TestTestingNondeterministicDomainLogicFindsDomainClock(t *testing.T) {
	dir := testingGitRepo(t)
	writeFile(t, filepath.Join(dir, "src", "domain", "coupon.cpp"), "long issued_at() {\n  return 0;\n}\n")
	commitAll(t, dir, "base")
	writeFile(t, filepath.Join(dir, "src", "domain", "coupon.cpp"), "#include <chrono>\n\nlong issued_at() {\n  return std::chrono::system_clock::now().time_since_epoch().count();\n}\n")

	cfg := testingChangeConfig(t, dir, "c++")
	cfg.Checks.ChangeRules.DetectBehaviorChangeWithoutTest = boolValue(false)
	report := runTestingChangeScan(t, cfg)

	assertFindingRulePresent(t, report, "Change Safety", "testing.nondeterministic-domain-logic")
}

func TestTestingChangeRulesTogglesDisableDetectors(t *testing.T) {
	dir := testingGitRepo(t)
	writeFile(t, filepath.Join(dir, "pricing", "price.go"), "package pricing\n\nfunc Price(v int) int {\n\treturn v\n}\n")
	commitAll(t, dir, "base")
	writeFile(t, filepath.Join(dir, "pricing", "price.go"), "package pricing\n\nfunc Price(v int) int {\n\tif v > 100 {\n\t\treturn v - 10\n\t}\n\treturn v\n}\n")

	cfg := testingChangeConfig(t, dir, "go")
	cfg.Checks.ChangeRules.DetectBehaviorChangeWithoutTest = boolValue(false)
	report := runTestingChangeScan(t, cfg)

	assertFindingRuleAbsent(t, report, "Change Safety", "testing.behavior-change-without-test")
}

func TestTestingLegacyHotspotUncoveredDoesNotEmitWithoutHistory(t *testing.T) {
	dir := testingGitRepo(t)
	writeFile(t, filepath.Join(dir, "legacy", "calculator.py"), "def calculate(value):\n    return value\n")
	commitAll(t, dir, "base")
	writeFile(t, filepath.Join(dir, "legacy", "calculator.py"), "def calculate(value):\n    return value + 1\n")

	cfg := testingChangeConfig(t, dir, "python")
	cfg.Checks.ChangeRules.DetectBehaviorChangeWithoutTest = boolValue(false)
	cfg.Checks.ChangeRules.DetectFailurePathMissing = boolValue(false)
	cfg.Checks.ChangeRules.DetectHardwiredDependency = boolValue(false)
	cfg.Checks.ChangeRules.DetectNondeterministicDomain = boolValue(false)
	report := runTestingChangeScan(t, cfg)

	assertFindingRuleAbsent(t, report, "Change Safety", "testing.legacy-hotspot-uncovered")
}

func testingChangeConfig(t *testing.T, dir string, language string) codeguard.Config {
	t.Helper()
	cfg := codeguard.ExampleConfig()
	cfg.Name = "change-testability"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SupplyChain = false
	cfg.Checks.Performance = boolValue(false)
	cfg.Checks.Reliability = boolValue(false)
	cfg.Checks.Data = boolValue(false)
	cfg.Checks.Contracts = boolValue(false)
	cfg.Checks.Context = boolValue(false)
	cfg.Checks.Change = boolValue(true)
	cfg.Cache.Enabled = boolValue(false)
	return cfg
}

func runTestingChangeScan(t *testing.T, cfg codeguard.Config) codeguard.Report {
	t.Helper()
	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("run diff: %v", err)
	}
	return report
}

func testingGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "CodeGuard Test")
	return dir
}
