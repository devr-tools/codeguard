package checks_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestQualityLocalDesignRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "local.go"), strings.Join([]string{
		"package sample",
		"",
		"var CurrentUser string",
		"",
		"const PremiumPolicy = \"invoice_policy_code\"",
		"const VIPPolicy = \"invoice_policy_code\"",
		"",
		"// validate input",
		"func validateInput(input string) {}",
		"",
		"func process(data string, active bool, customerID string, orderStatus string, currency string) string {",
		"\ttmp := data",
		"\trows.Query()",
		"\tsaveOrder()",
		"\treturn tmp",
		"}",
		"",
		"func buildInvoice(customerID string) string {",
		"\trepo.Save(customerID)",
		"\treturn customerID",
		"}",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	for _, ruleID := range []string{
		"quality.duplicated-knowledge",
		"quality.ambiguous-name",
		"quality.boolean-argument",
		"quality.primitive-obsession",
		"quality.hidden-side-effect",
		"quality.mutable-global-state",
		"quality.redundant-comment",
	} {
		assertFindingRulePresent(t, report, "Code Quality", ruleID)
		assertFindingLevel(t, report, "Code Quality", ruleID, "warn")
	}
}

func TestQualityLocalDesignRulesForScriptLanguages(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "local.ts"), strings.Join([]string{
		"let currentUser = '';",
		"",
		"export function buildInvoice(data: string, active: boolean, customerId: string, orderStatus: string, currency: string): string {",
		"  repo.save(customerId);",
		"  return customerId;",
		"}",
	}, "\n"))

	cfg := qualityPrecisionConfig(dir)
	cfg.Targets[0].Language = "typescript"
	report := runQualityPrecisionScan(t, cfg)

	assertFindingRulePresent(t, report, "Code Quality", "quality.mutable-global-state")
	assertFindingRulePresent(t, report, "Code Quality", "quality.boolean-argument")
	assertFindingRulePresent(t, report, "Code Quality", "quality.hidden-side-effect")
}
