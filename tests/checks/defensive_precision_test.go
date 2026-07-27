package checks_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefensiveUncheckedTypeAssertionWarnsForSingleValueAssertion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "assertion.go"), strings.Join([]string{
		"package sample",
		"",
		"func Decode(value any) string {",
		"\treturn value.(string)",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir, "go"))

	assertFindingRulePresent(t, report, "Code Quality", "defensive.unchecked-type-assertion")
	assertFindingLevel(t, report, "Code Quality", "defensive.unchecked-type-assertion", "warn")
}

func TestDefensiveUncheckedTypeAssertionAllowsCommaOKAssertion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "assertion_safe.go"), strings.Join([]string{
		"package sample",
		"",
		"func Decode(value any) (string, bool) {",
		"\ttext, ok := value.(string)",
		"\treturn text, ok",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir, "go"))

	assertFindingRuleAbsent(t, report, "Code Quality", "defensive.unchecked-type-assertion")
}

func TestDefensiveUnsafeNumericConversionWarnsForNarrowingConversion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "numeric.go"), strings.Join([]string{
		"package sample",
		"",
		"func Narrow(count int64) int32 {",
		"\treturn int32(count)",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir, "go"))

	assertFindingRulePresent(t, report, "Code Quality", "defensive.unsafe-numeric-conversion")
	assertFindingLevel(t, report, "Code Quality", "defensive.unsafe-numeric-conversion", "warn")
}
