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
