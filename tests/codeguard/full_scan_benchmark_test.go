package codeguard_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func BenchmarkFullScanMixedGoTypeScriptRepository(b *testing.B) {
	root := buildFullScanBenchmarkRepository(b, 64)
	for _, tc := range []struct {
		name               string
		cacheEnabled       bool
		warmup             bool
		scanVendoredSource bool
	}{
		{name: "cold", cacheEnabled: false},
		{name: "warm-cache", cacheEnabled: true, warmup: true},
		{name: "cold-vendored-source", cacheEnabled: false, scanVendoredSource: true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			cfg := fullScanBenchmarkConfig(root, tc.cacheEnabled, tc.scanVendoredSource)
			if tc.scanVendoredSource {
				report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull})
				if err != nil {
					b.Fatalf("verify vendored benchmark fixture: %v", err)
				}
				if !benchmarkReportHasVendorFinding(report) {
					b.Fatal("vendored benchmark case did not analyze vendored source")
				}
			}
			if tc.warmup {
				if _, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull}); err != nil {
					b.Fatalf("warm benchmark cache: %v", err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull})
				if err != nil {
					b.Fatalf("full scan: %v", err)
				}
				if len(report.Sections) < 10 {
					b.Fatalf("full scan returned only %d sections", len(report.Sections))
				}
			}
		})
	}
}

func buildFullScanBenchmarkRepository(b *testing.B, filesPerLanguage int) string {
	b.Helper()
	root := b.TempDir()
	for i := 0; i < filesPerLanguage; i++ {
		goSource := fmt.Sprintf(`package service

import "context"

type Handler%d struct{}

func (Handler%d) Execute(ctx context.Context, values []int) int {
	total := 0
	for _, value := range values {
		if value > 0 {
			total += value
		}
	}
	return total
}
`, i, i)
		writeBenchmarkFile(b, filepath.Join(root, "app", fmt.Sprintf("handler_%03d.go", i)), goSource)

		tsSource := fmt.Sprintf(`export interface Request%d { values: number[] }

export function execute%d(request: Request%d): number {
  let total = 0;
  for (const value of request.values) {
    if (value > 0) total += value;
  }
  return total;
}
`, i, i, i)
		writeBenchmarkFile(b, filepath.Join(root, "infrastructure", "src", fmt.Sprintf("stack-%03d.ts", i)), tsSource)
	}

	generated := strings.Repeat("export const generatedValue = 1;\n", 32)
	for i := 0; i < filesPerLanguage*4; i++ {
		writeBenchmarkFile(b, filepath.Join(root, "infrastructure", "node_modules", "library", fmt.Sprintf("generated-%03d.ts", i)), generated)
		writeBenchmarkFile(b, filepath.Join(root, "infrastructure", "cdk.out", fmt.Sprintf("asset.%03d", i), "index.js"), generated)
		writeBenchmarkFile(b, filepath.Join(root, "vendor", "example.com", "dependency", fmt.Sprintf("generated_%03d.go", i)), "package dependency\n")
	}
	writeBenchmarkFile(b, filepath.Join(root, "vendor", "example.com", "dependency", "oversized.go"), "package dependency\n"+strings.Repeat("// benchmark vendored source\n", 600))
	writeBenchmarkFile(b, filepath.Join(root, "go.mod"), "module example.com/fullscanbench\n\ngo 1.23\n")
	writeBenchmarkFile(b, filepath.Join(root, "package.json"), `{"name":"full-scan-benchmark","private":true}`)
	return root
}

func benchmarkReportHasVendorFinding(report codeguard.Report) bool {
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if strings.HasPrefix(filepath.ToSlash(finding.Path), "vendor/") {
				return true
			}
		}
	}
	return false
}

func fullScanBenchmarkConfig(root string, cacheEnabled bool, scanVendoredSource bool) codeguard.Config {
	enabled := true
	disabled := false
	cfg := codeguard.ExampleConfig()
	cfg.Name = "mixed-full-scan-benchmark"
	cfg.ScanVendoredSource = scanVendoredSource
	cfg.Targets = []codeguard.TargetConfig{
		{Name: "go-app", Path: root, Language: "go", Entrypoints: []string{"app"}},
		{Name: "typescript-infrastructure", Path: root, Language: "typescript", Entrypoints: []string{"infrastructure/src"}},
	}
	cfg.Checks.Quality = true
	cfg.Checks.Performance = &enabled
	cfg.Checks.Reliability = &enabled
	cfg.Checks.Data = &enabled
	cfg.Checks.Observability = &enabled
	cfg.Checks.Operations = &enabled
	cfg.Checks.Design = true
	cfg.Checks.Security = true
	cfg.Checks.Prompts = true
	cfg.Checks.CI = true
	cfg.Checks.SupplyChain = true
	cfg.Checks.Context = &enabled
	cfg.Checks.Change = &disabled
	cfg.Checks.Contracts = &disabled
	cfg.Checks.Delivery = &disabled
	cfg.Checks.QualityRules.DeadCode.Enabled = &disabled
	cfg.Checks.QualityRules.LocalPrecision = &disabled
	cfg.Checks.QualityRules.AIProvenance.Enabled = &disabled
	cfg.Checks.QualityRules.AIChangeRisk.Enabled = &disabled
	cfg.Checks.SecurityRules.GovulncheckMode = "off"
	cfg.Cache.Enabled = &cacheEnabled
	cfg.Cache.Path = filepath.Join(root, ".codeguard", "benchmark-cache.json")
	cfg.Output.Format = "json"
	return cfg
}

func writeBenchmarkFile(tb testing.TB, path string, content string) {
	tb.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tb.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		tb.Fatalf("write %s: %v", path, err)
	}
}
