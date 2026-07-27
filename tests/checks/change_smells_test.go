package checks_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func changeSmellQuietConfig(name string, dir string) codeguard.Config {
	cfg := changeSafetyTestConfig(name, dir)
	cfg.Checks.ChangeRules.MaxChangedFiles = 100
	cfg.Checks.ChangeRules.MaxChangedDirectories = 100
	cfg.Checks.ChangeRules.MaxChangedLines = 5000
	cfg.Checks.ChangeRules.MaxPublicInterfacesChanged = 100
	cfg.Checks.ChangeRules.MaxConcernFamilies = 100
	cfg.Checks.ChangeRules.MinTestToProductionRatioPercent = 0
	off := false
	cfg.Checks.ChangeRules.DetectBehaviorChangeWithoutTest = &off
	cfg.Checks.ChangeRules.DetectFailurePathMissing = &off
	cfg.Checks.ChangeRules.DetectHardwiredDependency = &off
	cfg.Checks.ChangeRules.DetectNondeterministicDomain = &off
	return cfg
}

type changeSmellLanguageCase struct {
	name     string
	language string
	path     string
	before   string
	after    string
}

func runChangeSmellCase(t *testing.T, name string, tc changeSmellLanguageCase) codeguard.Report {
	t.Helper()
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, tc.path), tc.before)
	commitAll(t, dir, "base")
	writeFile(t, filepath.Join(dir, tc.path), tc.after)
	cfg := changeSmellQuietConfig(name, dir)
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: tc.language}}
	return runChangeDiff(t, cfg)
}

func TestChangeOneUseAbstractionDetectsGoInterface(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "service", "payment.go"), "package service\n\nfunc Charge() error { return nil }\n")
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "service", "payment.go"), strings.Join([]string{
		"package service",
		"",
		"type PaymentGateway interface {",
		"\tCharge() error",
		"}",
		"",
		"func NewPayment(gateway PaymentGateway) error {",
		"\treturn gateway.Charge()",
		"}",
		"",
	}, "\n"))

	report := runChangeDiff(t, changeSmellQuietConfig("change-one-use-go", dir))
	assertFindingRulePresent(t, report, "Change Safety", "change.one-use-abstraction")
}

func TestChangeOneUseAbstractionAllowsMultipleConsumers(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "service", "payment.go"), "package service\n\nfunc Charge() error { return nil }\n")
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "service", "payment.go"), strings.Join([]string{
		"package service",
		"",
		"type PaymentGateway interface {",
		"\tCharge() error",
		"}",
		"",
		"func NewPayment(gateway PaymentGateway) error {",
		"\treturn gateway.Charge()",
		"}",
		"",
		"func RetryPayment(primary PaymentGateway, fallback PaymentGateway) error {",
		"\tif err := primary.Charge(); err != nil {",
		"\t\treturn fallback.Charge()",
		"\t}",
		"\treturn nil",
		"}",
		"",
	}, "\n"))

	report := runChangeDiff(t, changeSmellQuietConfig("change-one-use-negative", dir))
	assertFindingRuleAbsent(t, report, "Change Safety", "change.one-use-abstraction")
}

func TestChangeOneUseAbstractionDetectsTypeScriptInterface(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "src", "billing.ts"), "export function charge() { return true }\n")
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "src", "billing.ts"), strings.Join([]string{
		"export interface BillingGateway {",
		"  charge(): boolean",
		"}",
		"",
		"export function chargeWith(gateway: BillingGateway) {",
		"  return gateway.charge()",
		"}",
		"",
	}, "\n"))

	report := runChangeDiff(t, changeSmellQuietConfig("change-one-use-ts", dir))
	assertFindingRulePresent(t, report, "Change Safety", "change.one-use-abstraction")
}

func TestChangeOneUseAbstractionDetectsAdditionalLanguages(t *testing.T) {
	for _, tc := range []changeSmellLanguageCase{
		{
			name:     "python",
			language: "python",
			path:     "service/payment.py",
			before:   "def charge():\n    return True\n",
			after: strings.Join([]string{
				"from typing import Protocol",
				"",
				"class PaymentGateway(Protocol):",
				"    def charge(self) -> bool: ...",
				"",
				"def charge_with(gateway: PaymentGateway):",
				"    return gateway.charge()",
				"",
			}, "\n"),
		},
		{
			name:     "javascript",
			language: "javascript",
			path:     "src/billing.js",
			before:   "export function charge() { return true }\n",
			after: strings.Join([]string{
				"export class BillingGateway {",
				"  charge() { return true }",
				"}",
				"",
				"export function chargeWith(gateway) {",
				"  return gateway.charge()",
				"}",
				"",
			}, "\n"),
		},
		{
			name:     "cpp",
			language: "c++",
			path:     "service/payment.cpp",
			before:   "bool Charge() { return true; }\n",
			after: strings.Join([]string{
				"class PaymentGateway { public: virtual bool Charge() = 0; };",
				"",
				"bool ChargeWith(PaymentGateway& gateway) {",
				"  return gateway.Charge();",
				"}",
				"",
			}, "\n"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := runChangeSmellCase(t, "change-one-use-"+tc.name, tc)
			assertFindingRulePresent(t, report, "Change Safety", "change.one-use-abstraction")
		})
	}
}

func TestChangeDuplicateHelperDetectsGoDuplicate(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "service", "email.go"), strings.Join([]string{
		"package service",
		"",
		"import \"strings\"",
		"",
		"func canonicalizeEmail(value string) string {",
		"\ttrimmed := strings.TrimSpace(value)",
		"\tlower := strings.ToLower(trimmed)",
		"\treturn lower",
		"}",
		"",
	}, "\n"))
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "service", "email_new.go"), strings.Join([]string{
		"package service",
		"",
		"import \"strings\"",
		"",
		"func normalizeEmail(value string) string {",
		"\ttrimmed := strings.TrimSpace(value)",
		"\tlower := strings.ToLower(trimmed)",
		"\treturn lower",
		"}",
		"",
	}, "\n"))
	runGit(t, dir, "add", "-N", "service/email_new.go")

	report := runChangeDiff(t, changeSmellQuietConfig("change-duplicate-go", dir))
	assertFindingRulePresent(t, report, "Change Safety", "change.duplicate-helper")
}

func TestChangeDuplicateHelperAllowsDifferentLogic(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "service", "email.go"), strings.Join([]string{
		"package service",
		"",
		"import \"strings\"",
		"",
		"func canonicalizeEmail(value string) string {",
		"\ttrimmed := strings.TrimSpace(value)",
		"\tlower := strings.ToLower(trimmed)",
		"\treturn lower",
		"}",
		"",
	}, "\n"))
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "service", "email_new.go"), strings.Join([]string{
		"package service",
		"",
		"import \"strings\"",
		"",
		"func normalizeDisplayName(value string) string {",
		"\ttrimmed := strings.TrimSpace(value)",
		"\treturn strings.ReplaceAll(trimmed, \"_\", \" \")",
		"}",
		"",
	}, "\n"))
	runGit(t, dir, "add", "-N", "service/email_new.go")

	report := runChangeDiff(t, changeSmellQuietConfig("change-duplicate-negative", dir))
	assertFindingRuleAbsent(t, report, "Change Safety", "change.duplicate-helper")
}

func TestChangeDuplicateHelperDetectsTypeScriptDuplicate(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "src", "email.ts"), strings.Join([]string{
		"export function canonicalizeEmail(value: string) {",
		"  const trimmed = value.trim()",
		"  const lower = trimmed.toLowerCase()",
		"  return lower",
		"}",
		"",
	}, "\n"))
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "src", "email_new.ts"), strings.Join([]string{
		"export function normalizeEmail(value: string) {",
		"  const trimmed = value.trim()",
		"  const lower = trimmed.toLowerCase()",
		"  return lower",
		"}",
		"",
	}, "\n"))
	runGit(t, dir, "add", "-N", "src/email_new.ts")

	report := runChangeDiff(t, changeSmellQuietConfig("change-duplicate-ts", dir))
	assertFindingRulePresent(t, report, "Change Safety", "change.duplicate-helper")
}

func TestChangeComplexityIncreasedDetectsPythonBranchGrowth(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "service", "pricing.py"), strings.Join([]string{
		"def price(order):",
		"    return order.total",
		"",
	}, "\n"))
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "service", "pricing.py"), strings.Join([]string{
		"def price(order):",
		"    if order.vip:",
		"        return order.total * 0.9",
		"    if order.country == 'AU':",
		"        return order.total + order.tax",
		"    return order.total",
		"",
	}, "\n"))

	report := runChangeDiff(t, changeSmellQuietConfig("change-complexity-python", dir))
	assertFindingRulePresent(t, report, "Change Safety", "change.complexity-increased")
}

func TestChangeComplexityIncreasedAcrossAdditionalLanguages(t *testing.T) {
	for _, tc := range []changeSmellLanguageCase{
		{name: "go", language: "go", path: "service/pricing.go", before: "package service\n\nfunc Price(total int) int {\n\treturn total\n}\n", after: "package service\n\nfunc Price(total int, vip bool) int {\n\tif vip { return total - 10 }\n\tif total > 100 { return total - 5 }\n\treturn total\n}\n"},
		{name: "typescript", language: "typescript", path: "src/pricing.ts", before: "export function price(total: number) { return total }\n", after: "export function price(total: number, vip: boolean) {\n  if (vip) return total - 10\n  if (total > 100) return total - 5\n  return total\n}\n"},
		{name: "javascript", language: "javascript", path: "src/pricing.js", before: "export function price(total) { return total }\n", after: "export function price(total, vip) {\n  if (vip) return total - 10\n  if (total > 100) return total - 5\n  return total\n}\n"},
		{name: "cpp", language: "c++", path: "service/pricing.cpp", before: "int Price(int total) { return total; }\n", after: "int Price(int total, bool vip) {\n  if (vip) return total - 10;\n  if (total > 100) return total - 5;\n  return total;\n}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := runChangeSmellCase(t, "change-complexity-"+tc.name, tc)
			assertFindingRulePresent(t, report, "Change Safety", "change.complexity-increased")
		})
	}
}

func TestChangeComplexityIncreasedAllowsLinearEdit(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "service", "pricing.go"), "package service\n\nfunc Price(total int) int {\n\treturn total\n}\n")
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "service", "pricing.go"), "package service\n\nfunc Price(total int) int {\n\tdiscounted := total - 1\n\treturn discounted\n}\n")

	report := runChangeDiff(t, changeSmellQuietConfig("change-complexity-negative", dir))
	assertFindingRuleAbsent(t, report, "Change Safety", "change.complexity-increased")
}

func TestChangeCleanupRegressionDetectsClaimedCleanupComplexityGrowth(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "service", "cleanup.go"), "package service\n\nfunc Route(kind string) string {\n\treturn \"default\"\n}\n")
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "service", "cleanup.go"), strings.Join([]string{
		"package service",
		"",
		"func Route(kind string, admin bool) string {",
		"\tif admin {",
		"\t\treturn \"admin\"",
		"\t}",
		"\tif kind == \"vip\" {",
		"\t\treturn \"vip\"",
		"\t}",
		"\treturn \"default\"",
		"}",
		"",
	}, "\n"))

	report := runChangeDiff(t, changeSmellQuietConfig("cleanup-refactor-regression", dir))
	assertFindingRulePresent(t, report, "Change Safety", "change.cleanup-regression")
}

func TestChangeCleanupRegressionAcrossAdditionalLanguages(t *testing.T) {
	for _, tc := range []changeSmellLanguageCase{
		{name: "python", language: "python", path: "service/cleanup.py", before: "def route(kind):\n    return 'default'\n", after: "def route(kind, admin):\n    if admin:\n        return 'admin'\n    if kind == 'vip':\n        return 'vip'\n    return 'default'\n"},
		{name: "typescript", language: "typescript", path: "src/cleanup.ts", before: "export function route(kind: string) { return 'default' }\n", after: "export function route(kind: string, admin: boolean) {\n  if (admin) return 'admin'\n  if (kind === 'vip') return 'vip'\n  return 'default'\n}\n"},
		{name: "javascript", language: "javascript", path: "src/cleanup.js", before: "export function route(kind) { return 'default' }\n", after: "export function route(kind, admin) {\n  if (admin) return 'admin'\n  if (kind === 'vip') return 'vip'\n  return 'default'\n}\n"},
		{name: "cpp", language: "c++", path: "service/cleanup.cpp", before: "std::string Route(std::string kind) { return \"default\"; }\n", after: "std::string Route(std::string kind, bool admin) {\n  if (admin) return \"admin\";\n  if (kind == \"vip\") return \"vip\";\n  return \"default\";\n}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := runChangeSmellCase(t, "cleanup-regression-"+tc.name, tc)
			assertFindingRulePresent(t, report, "Change Safety", "change.cleanup-regression")
		})
	}
}

func TestChangeCleanupRegressionRequiresCleanupClaim(t *testing.T) {
	dir := initChangeRepo(t)
	writeFile(t, filepath.Join(dir, "service", "route.go"), "package service\n\nfunc Route(kind string) string {\n\treturn \"default\"\n}\n")
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "service", "route.go"), strings.Join([]string{
		"package service",
		"",
		"func Route(kind string, admin bool) string {",
		"\tif admin {",
		"\t\treturn \"admin\"",
		"\t}",
		"\tif kind == \"vip\" {",
		"\t\treturn \"vip\"",
		"\t}",
		"\treturn \"default\"",
		"}",
		"",
	}, "\n"))

	report := runChangeDiff(t, changeSmellQuietConfig("feature-route-change", dir))
	assertFindingRuleAbsent(t, report, "Change Safety", "change.cleanup-regression")
}
