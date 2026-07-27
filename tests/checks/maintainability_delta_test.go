package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func initMaintainabilityDeltaRepo(t *testing.T, baseSource string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "api.go"), baseSource)
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "CodeGuard Test")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")
	runGit(t, dir, "checkout", "-b", "feature")
	return dir
}

func runMaintainabilityDeltaScan(t *testing.T, cfg codeguard.Config) codeguard.Report {
	t.Helper()
	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("diff scan: %v", err)
	}
	return report
}

func TestMaintainabilityPublicSurfaceGrowthWarnsInDiffScan(t *testing.T) {
	dir := initMaintainabilityDeltaRepo(t, strings.Join([]string{
		"package sample",
		"",
		"func Existing() string {",
		"\treturn \"ok\"",
		"}",
		"",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "api.go"), strings.Join([]string{
		"package sample",
		"",
		"func Existing() string {",
		"\treturn \"ok\"",
		"}",
		"",
		"func NewExported() string {",
		"\treturn \"new\"",
		"}",
		"",
	}, "\n"))

	report := runMaintainabilityDeltaScan(t, qualityPrecisionConfig(dir))

	assertFindingRulePresent(t, report, "Code Quality", "maintainability.public-surface-growth")
	assertFindingLevel(t, report, "Code Quality", "maintainability.public-surface-growth", "warn")
}

func TestMaintainabilityDependencyGrowthWarnsInDiffScan(t *testing.T) {
	dir := initMaintainabilityDeltaRepo(t, strings.Join([]string{
		"package sample",
		"",
		"import \"fmt\"",
		"",
		"func Existing() string {",
		"\treturn fmt.Sprint(\"ok\")",
		"}",
		"",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "api.go"), strings.Join([]string{
		"package sample",
		"",
		"import (",
		"\t\"fmt\"",
		"\t\"strings\"",
		")",
		"",
		"func Existing() string {",
		"\treturn strings.TrimSpace(fmt.Sprint(\"ok\"))",
		"}",
		"",
	}, "\n"))

	report := runMaintainabilityDeltaScan(t, qualityPrecisionConfig(dir))

	assertFindingRulePresent(t, report, "Code Quality", "maintainability.dependency-growth")
	assertFindingLevel(t, report, "Code Quality", "maintainability.dependency-growth", "warn")
}
