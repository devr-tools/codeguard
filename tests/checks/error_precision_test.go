package checks_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestErrorLoggedAndIgnoredWarnsWhenErrorBecomesSuccess(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "errors.go"), strings.Join([]string{
		"package sample",
		"",
		"import \"log\"",
		"",
		"func LoadProfile(id string) error {",
		"\tif err := readProfile(id); err != nil {",
		"\t\tlog.Printf(\"load profile: %v\", err)",
		"\t\treturn nil",
		"\t}",
		"\treturn nil",
		"}",
		"",
		"func readProfile(string) error { return nil }",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir, "go"))

	assertFindingRulePresent(t, report, "Code Quality", "error.logged-and-ignored")
	assertFindingLevel(t, report, "Code Quality", "error.logged-and-ignored", "warn")
}

func TestErrorContextLostWarnsForBareErrorReturn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "context.go"), strings.Join([]string{
		"package sample",
		"",
		"func SaveProfile(id string) error {",
		"\tif err := writeProfile(id); err != nil {",
		"\t\treturn err",
		"\t}",
		"\treturn nil",
		"}",
		"",
		"func writeProfile(string) error { return nil }",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir, "go"))

	assertFindingRulePresent(t, report, "Code Quality", "error.context-lost")
	assertFindingLevel(t, report, "Code Quality", "error.context-lost", "warn")
}
