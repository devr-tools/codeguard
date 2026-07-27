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

func TestTestingFailurePathMissingAcrossLanguages(t *testing.T) {
	for _, tc := range testabilityFailurePathCases() {
		t.Run(tc.name, func(t *testing.T) {
			report := runTestabilityCase(t, tc, func(cfg *codeguard.Config) {
				cfg.Checks.ChangeRules.DetectBehaviorChangeWithoutTest = boolValue(false)
			})

			assertFindingRulePresent(t, report, "Change Safety", "testing.failure-path-missing")
		})
	}
}

func TestTestingHardwiredDependencyAcrossLanguages(t *testing.T) {
	for _, tc := range testabilityHardwiredCases() {
		t.Run(tc.name, func(t *testing.T) {
			report := runTestabilityCase(t, tc, func(cfg *codeguard.Config) {
				cfg.Checks.ChangeRules.DetectBehaviorChangeWithoutTest = boolValue(false)
				cfg.Checks.ChangeRules.DetectFailurePathMissing = boolValue(false)
			})

			assertFindingRulePresent(t, report, "Change Safety", "testing.hardwired-dependency")
		})
	}
}

func TestTestingNondeterministicDomainLogicAcrossLanguages(t *testing.T) {
	for _, tc := range testabilityNondeterministicCases() {
		t.Run(tc.name, func(t *testing.T) {
			report := runTestabilityCase(t, tc, func(cfg *codeguard.Config) {
				cfg.Checks.ChangeRules.DetectBehaviorChangeWithoutTest = boolValue(false)
				cfg.Checks.ChangeRules.DetectFailurePathMissing = boolValue(false)
				cfg.Checks.ChangeRules.DetectHardwiredDependency = boolValue(false)
			})

			assertFindingRulePresent(t, report, "Change Safety", "testing.nondeterministic-domain-logic")
		})
	}
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

func TestTestingLegacyHotspotUncoveredUsesHistoryEvidence(t *testing.T) {
	dir := testingGitRepo(t)
	path := filepath.Join("legacy", "calculator.py")
	writeFile(t, filepath.Join(dir, path), "def calculate(value):\n    return value\n")
	commitAll(t, dir, "base")
	writeFile(t, filepath.Join(dir, path), "def calculate(value):\n    adjusted = value + 1\n    return adjusted\n")
	commitAll(t, dir, "fix calculator adjustment")
	writeFile(t, filepath.Join(dir, path), "def calculate(value):\n    adjusted = value + 2\n    if adjusted > 10:\n        return adjusted - 1\n    return adjusted\n")
	commitAll(t, dir, "bugfix calculator threshold")
	writeFile(t, filepath.Join(dir, path), "def calculate(value):\n    adjusted = value + 3\n    if adjusted > 10:\n        return adjusted - 1\n    if adjusted < 0:\n        return 0\n    return adjusted\n")
	commitAll(t, dir, "refactor calculator branch")
	writeFile(t, filepath.Join(dir, path), "def calculate(value):\n    adjusted = value + 4\n    if adjusted > 10:\n        return adjusted - 2\n    if adjusted < 0:\n        return 0\n    return adjusted\n")

	cfg := testingChangeConfig(t, dir, "python")
	cfg.Checks.ChangeRules.DetectBehaviorChangeWithoutTest = boolValue(false)
	cfg.Checks.ChangeRules.DetectFailurePathMissing = boolValue(false)
	cfg.Checks.ChangeRules.DetectHardwiredDependency = boolValue(false)
	cfg.Checks.ChangeRules.DetectNondeterministicDomain = boolValue(false)
	report := runTestingChangeScan(t, cfg)

	assertFindingRulePresent(t, report, "Change Safety", "testing.legacy-hotspot-uncovered")
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

type testabilityCase struct {
	name     string
	language string
	path     string
	before   string
	after    string
}

func runTestabilityCase(t *testing.T, tc testabilityCase, tune func(*codeguard.Config)) codeguard.Report {
	t.Helper()
	dir := testingGitRepo(t)
	writeFile(t, filepath.Join(dir, tc.path), tc.before)
	commitAll(t, dir, "base")
	writeFile(t, filepath.Join(dir, tc.path), tc.after)

	cfg := testingChangeConfig(t, dir, tc.language)
	if tune != nil {
		tune(&cfg)
	}
	return runTestingChangeScan(t, cfg)
}

func testabilityFailurePathCases() []testabilityCase {
	return []testabilityCase{
		{name: "go", language: "go", path: "domain/payment.go", before: "package domain\n\nfunc Authorize(ok bool) bool { return ok }\n", after: "package domain\n\nimport \"errors\"\n\nfunc Authorize(ok bool) error {\n\tif !ok { return errors.New(\"denied\") }\n\treturn nil\n}\n"},
		{name: "python", language: "python", path: "app/domain/payment.py", before: "def authorize(ok):\n    return ok\n", after: "def authorize(ok):\n    if not ok:\n        raise Exception('denied')\n    return True\n"},
		{name: "typescript", language: "typescript", path: "src/domain/payment.ts", before: "export function authorize(ok: boolean) { return ok }\n", after: "export function authorize(ok: boolean) { if (!ok) { throw new Error('denied') } return true }\n"},
		{name: "javascript", language: "javascript", path: "src/domain/payment.js", before: "export function authorize(ok) { return ok }\n", after: "export function authorize(ok) { if (!ok) { throw new Error('denied') } return true }\n"},
		{name: "cpp", language: "c++", path: "src/domain/payment.cpp", before: "bool Authorize(bool ok) { return ok; }\n", after: "#include <stdexcept>\n\nbool Authorize(bool ok) { if (!ok) { throw std::runtime_error(\"denied\"); } return true; }\n"},
	}
}

func testabilityHardwiredCases() []testabilityCase {
	return []testabilityCase{
		{name: "go", language: "go", path: "domain/profile.go", before: "package domain\n\nfunc Load() string { return \"ok\" }\n", after: "package domain\n\nimport \"net/http\"\n\nfunc Load() string { http.Get(\"https://example.test\"); return \"ok\" }\n"},
		{name: "python", language: "python", path: "app/domain/profile.py", before: "def load():\n    return 'ok'\n", after: "import requests\n\ndef load():\n    requests.get('https://example.test')\n    return 'ok'\n"},
		{name: "typescript", language: "typescript", path: "src/domain/profile.ts", before: "export function load() { return 'ok' }\n", after: "export function load() { fetch('https://example.test'); return 'ok' }\n"},
		{name: "javascript", language: "javascript", path: "src/domain/profile.js", before: "export function load() { return 'ok' }\n", after: "export function load() { fetch('https://example.test'); return 'ok' }\n"},
		{name: "cpp", language: "c++", path: "src/domain/profile.cpp", before: "std::string Load() { return \"ok\"; }\n", after: "#include <fstream>\n\nstd::string Load() { std::ifstream file(\"profile.txt\"); return \"ok\"; }\n"},
	}
}

func testabilityNondeterministicCases() []testabilityCase {
	return []testabilityCase{
		{name: "go", language: "go", path: "domain/coupon.go", before: "package domain\n\nfunc IssuedAt() int64 { return 0 }\n", after: "package domain\n\nimport \"time\"\n\nfunc IssuedAt() int64 { return time.Now().Unix() }\n"},
		{name: "python", language: "python", path: "app/domain/coupon.py", before: "def issued_at():\n    return 0\n", after: "import datetime\n\ndef issued_at():\n    return datetime.datetime.now().timestamp()\n"},
		{name: "typescript", language: "typescript", path: "src/domain/coupon.ts", before: "export function issuedAt() { return 0 }\n", after: "export function issuedAt() { return Date.now() }\n"},
		{name: "javascript", language: "javascript", path: "src/domain/coupon.js", before: "export function issuedAt() { return 0 }\n", after: "export function issuedAt() { return Math.random() }\n"},
		{name: "cpp", language: "c++", path: "src/domain/coupon.cpp", before: "long issued_at() { return 0; }\n", after: "#include <chrono>\n\nlong issued_at() { return std::chrono::system_clock::now().time_since_epoch().count(); }\n"},
	}
}

func testingGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "CodeGuard Test")
	return dir
}
