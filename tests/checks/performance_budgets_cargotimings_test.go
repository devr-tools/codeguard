package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestPerformanceBudgetCargoTimingsUnderBudgetPasses(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cargo-timing.html"), `<script>
UNIT_DATA = [{"name":"serde","start":0.0,"duration":0.009}];
</script>`)

	report, err := codeguard.Run(context.Background(), budgetConfig("cargo-timings-under", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "rust-build", Kind: "cargo-timings", Path: "cargo-timing.html", MaxMilliseconds: 10},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "pass")
	assertFindingRuleAbsent(t, report, "Performance", "performance.budget")
}

func TestPerformanceBudgetCargoTimingsPerCrateWarns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cargo-timing.html"), `<script>
UNIT_DATA = [
  {"name":"serde","start":0.000,"duration":0.030},
  {"name":"serde","start":0.040,"duration":0.020},
  {"name":"syn","start":0.010,"duration":0.015}
];
</script>`)

	report, err := codeguard.Run(context.Background(), budgetConfig("cargo-timings-crate", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "serde-build", Kind: "cargo-timings", Path: "cargo-timing.html", Crate: "serde", MaxMilliseconds: 40},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "warn")
	findBudgetFindingMessage(t, report, `crate "serde" totals 50.0 ms`)
}

func TestPerformanceBudgetCargoTimingsGlobSumsReports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cargo-a.html"), `<script>UNIT_DATA = [{"name":"serde","start":0.0,"duration":0.020}];</script>`)
	writeFile(t, filepath.Join(dir, "cargo-b.html"), `<script>UNIT_DATA = [{"name":"serde","start":0.0,"duration":0.015}];</script>`)

	report, err := codeguard.Run(context.Background(), budgetConfig("cargo-timings-glob", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "rust-build", Kind: "cargo-timings", Path: "cargo-*.html", MaxMilliseconds: 30},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "warn")
	findBudgetFindingMessage(t, report, `cargo-*.html" totals 35.0 ms`)
}

func TestPerformanceBudgetCargoTimingsMalformedReportWarns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cargo-timing.html"), `<html><body>no unit data</body></html>`)

	report, err := codeguard.Run(context.Background(), budgetConfig("cargo-timings-bad", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "rust-build", Kind: "cargo-timings", Path: "cargo-timing.html", MaxMilliseconds: 10, Level: "fail"},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "warn")
	findBudgetFindingMessage(t, report, "UNIT_DATA payload not found")
}

func TestPerformanceBudgetValidationRejectsBadCargoTimingsEntries(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		label  string
		budget codeguard.PerformanceBudgetConfig
		want   string
	}{
		{"missing max_milliseconds", codeguard.PerformanceBudgetConfig{Name: "x", Kind: "cargo-timings", Path: "cargo-timing.html"}, "max_milliseconds must be positive"},
		{"crate on file-size", codeguard.PerformanceBudgetConfig{Name: "x", Kind: "file-size", Path: "a", Crate: "serde", MaxBytes: 1}, "crate only applies"},
	}
	for _, tc := range cases {
		cfg := budgetConfig("budget-validate-cargo-timings", dir, []codeguard.PerformanceBudgetConfig{tc.budget})
		err := codeguard.ValidateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: expected error containing %q, got %v", tc.label, tc.want, err)
		}
	}
}
