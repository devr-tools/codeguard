package checks_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

const (
	retiredMixedAbstractionRuleID   = "quality.mixed-abstraction-levels"
	canonicalMixedAbstractionRuleID = "function.mixed-abstraction-level"
)

func writeMixedAbstractionFixture(t *testing.T, dir string, firstLine string) {
	t.Helper()
	lines := []string{
		"package sample",
		"",
		"func Checkout(order Order) error {",
		"\tvalidateOrder(order)",
		"\trows, err := db.Query(\"select 1\")",
		"\tif err != nil {",
		"\t\treturn err",
		"\t}",
		"\tdefer rows.Close()",
		"\treturn persistOrder(order)",
		"}",
		"",
		"type Order struct{}",
		"",
	}
	if firstLine != "" {
		lines[2] = firstLine + "\n" + lines[2]
	}
	writeFile(t, filepath.Join(dir, "checkout.go"), strings.Join(lines, "\n"))
}

// A waiver written against the retired, never-emitted id must keep working by
// resolving to the rule that actually fires.
func TestRetiredRuleAliasWaiverSuppressesCanonicalRule(t *testing.T) {
	dir := t.TempDir()
	writeMixedAbstractionFixture(t, dir, "")
	cfg := qualityPrecisionConfig(dir)
	cfg.Waivers = []codeguard.WaiverConfig{{Rule: retiredMixedAbstractionRuleID, Reason: "legacy id"}}

	report := runQualityPrecisionScan(t, cfg)

	assertFindingRuleAbsent(t, report, "Code Quality", canonicalMixedAbstractionRuleID)
}

func TestRetiredRuleAliasInlineSuppressionSuppressesCanonicalRule(t *testing.T) {
	dir := t.TempDir()
	writeMixedAbstractionFixture(t, dir, "// codeguard:ignore "+retiredMixedAbstractionRuleID+" legacy id")

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir))

	assertFindingRuleAbsent(t, report, "Code Quality", canonicalMixedAbstractionRuleID)
}

func TestRetiredRuleAliasExplainsCanonicalRule(t *testing.T) {
	rule, ok := codeguard.ExplainRule(retiredMixedAbstractionRuleID)
	if !ok || rule.ID != canonicalMixedAbstractionRuleID {
		t.Fatalf("ExplainRule(%q) = (%q, %v), want %q", retiredMixedAbstractionRuleID, rule.ID, ok, canonicalMixedAbstractionRuleID)
	}
	for _, listed := range codeguard.Rules() {
		if listed.ID == retiredMixedAbstractionRuleID {
			t.Fatalf("%q must not be listed as a separate catalog rule", retiredMixedAbstractionRuleID)
		}
	}
}
