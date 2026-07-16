package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestPerformanceBudgetBundleStatsEsbuildTotal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "meta.json"), `{
  "outputs": {
    "dist/app.js": {"bytes": 90000},
    "dist/vendor.js": {"bytes": 60000}
  }
}`)

	report, err := codeguard.Run(context.Background(), budgetConfig("budget-esbuild", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "bundle-total", Kind: "bundle-stats", Path: "meta.json", MaxBytes: 100000},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "warn")
	findBudgetFindingMessage(t, report, "total 150000 bytes")
}

func TestPerformanceBudgetBundleStatsWebpackPerAsset(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "stats.json"), `{
  "assets": [
    {"name": "main.js", "size": 120000},
    {"name": "vendor.js", "size": 30000}
  ]
}`)

	cfg := budgetConfig("budget-webpack-asset", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "main-bundle", Kind: "bundle-stats", Path: "stats.json", Asset: "main.js", MaxBytes: 100000},
		{Name: "vendor-bundle", Kind: "bundle-stats", Path: "stats.json", Asset: "vendor.js", MaxBytes: 100000},
	})
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "warn")
	findBudgetFindingMessage(t, report, `asset "main.js" is 120000 bytes`)
	for _, section := range report.Sections {
		if section.Name != "Performance" {
			continue
		}
		for _, finding := range section.Findings {
			if strings.Contains(finding.Message, "vendor-bundle") {
				t.Fatalf("under-budget asset unexpectedly reported: %s", finding.Message)
			}
		}
	}
}
