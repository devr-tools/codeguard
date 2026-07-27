package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func qualityPrecisionConfig(dir string) codeguard.Config {
	return qualityPrecisionConfigForLanguage(dir, "go")
}

func qualityPrecisionConfigForLanguage(dir string, language string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-precision"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.Quality = true
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	off := false
	cfg.Checks.Context = &off
	cfg.Cache.Enabled = &off
	return cfg
}

func runQualityPrecisionScan(t *testing.T, cfg codeguard.Config) codeguard.Report {
	t.Helper()
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

func TestNamingGenericIdentifierWarnsForPlaceholderNames(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "names.go"), strings.Join([]string{
		"package sample",
		"",
		"func foo(input string) string {",
		"\ttmp := input",
		"\treturn tmp",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRulePresent(t, report, "Code Quality", "naming.generic-identifier")
	assertFindingLevel(t, report, "Code Quality", "naming.generic-identifier", "warn")
}

func TestNamingGenericIdentifierSkipsTestFixtures(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "names_test.go"), strings.Join([]string{
		"package sample",
		"",
		"func TestFoo(t any) {",
		"\ttmp := t",
		"\t_ = tmp",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", "naming.generic-identifier")
}

func TestNamingBehaviorMismatchWarnsAcrossLanguages(t *testing.T) {
	cases := []struct {
		name     string
		language string
		file     string
		source   []string
	}{
		{
			name:     "go",
			language: "go",
			file:     "orders.go",
			source: []string{
				"package sample",
				"type Audit interface { Save(string) error }",
				"func GetOrder(audit Audit, id string) (string, error) {",
				"\tif err := audit.Save(id); err != nil { return \"\", err }",
				"\treturn id, nil",
				"}",
			},
		},
		{
			name:     "python",
			language: "python",
			file:     "orders.py",
			source: []string{
				"def get_order(audit, order_id):",
				"    audit.save(order_id)",
				"    return order_id",
			},
		},
		{
			name:     "typescript",
			language: "typescript",
			file:     "orders.ts",
			source: []string{
				"export function getOrder(audit: Audit, orderId: string): string {",
				"  audit.save(orderId);",
				"  return orderId;",
				"}",
				"interface Audit { save(id: string): void }",
			},
		},
		{
			name:     "javascript",
			language: "javascript",
			file:     "orders.js",
			source: []string{
				"export function getOrder(audit, orderId) {",
				"  audit.save(orderId);",
				"  return orderId;",
				"}",
			},
		},
		{
			name:     "cpp",
			language: "cpp",
			file:     "orders.cpp",
			source: []string{
				"struct Audit { void Save(const char* id); };",
				"const char* GetOrder(Audit& audit, const char* id) {",
				"  audit.Save(id);",
				"  return id;",
				"}",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file), strings.Join(tc.source, "\n"))

			report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, tc.language))

			assertFindingRulePresent(t, report, "Code Quality", "naming.behavior-mismatch")
		})
	}
}

func TestNamingPredicateAndCardinalityPositiveNegative(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "names.ts"), strings.Join([]string{
		"export function evaluate(users: number, user: Array<string>, enabled: boolean): boolean {",
		"  const active = enabled === true;",
		"  const isReady = users > 0;",
		"  return active && isReady;",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRulePresent(t, report, "Code Quality", "naming.boolean-not-predicate")
	assertFindingRulePresent(t, report, "Code Quality", "naming.cardinality-mismatch")
}

func TestNamingUnitsAbbreviationsAndImplementationLeak(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "names.py"), strings.Join([]string{
		"def build_sql_invoice(cust_id: str, timeout: int, price: int):",
		"    retry_count = 2",
		"    timeout_ms = 30",
		"    return cust_id",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "python"))

	assertFindingRulePresent(t, report, "Code Quality", "naming.implementation-leak")
	assertFindingRulePresent(t, report, "Code Quality", "naming.missing-unit")
	assertFindingRulePresent(t, report, "Code Quality", "naming.unknown-abbreviation")
}

func TestNamingGlossaryRoleSuffixAndCrossLayerHeuristics(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "layers.go"), strings.Join([]string{
		"package sample",
		"type RestaurantResponse struct{}",
		"type VenueEntity struct{}",
		"type MerchantRecord struct{}",
		"type OrderManager struct{}",
		"type OrderHelper struct{}",
		"type OrderUtil struct{}",
		"type OrderProcessor struct{}",
	}, "\n"))
	cfg := qualityPrecisionConfig(dir)
	cfg.Checks.QualityRules.Naming.Glossary = map[string]codeguard.QualityNamingGlossaryEntry{
		"restaurant": {Avoid: []string{"venue", "merchant"}},
	}

	report := runQualityPrecisionScan(t, cfg)

	assertFindingRulePresent(t, report, "Code Quality", "naming.domain-vocabulary-drift")
	assertFindingRulePresent(t, report, "Code Quality", "naming.role-suffix-overuse")
	assertFindingRulePresent(t, report, "Code Quality", "naming.cross-layer-inconsistency")
}

func TestNamingPrecisionSkipsClearNames(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "clear.ts"), strings.Join([]string{
		"export function isOrderReady(items: string[], timeoutMs: number): boolean {",
		"  const hasItems = items.length > 0;",
		"  return hasItems && timeoutMs > 0;",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfigForLanguage(dir, "typescript"))

	assertFindingRuleAbsent(t, report, "Code Quality", "naming.boolean-not-predicate")
	assertFindingRuleAbsent(t, report, "Code Quality", "naming.cardinality-mismatch")
	assertFindingRuleAbsent(t, report, "Code Quality", "naming.missing-unit")
}
