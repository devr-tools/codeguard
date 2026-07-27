package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func qualityPrecisionConfig(dir string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = "quality-precision"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
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
