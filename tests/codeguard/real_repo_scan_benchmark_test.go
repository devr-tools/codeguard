package codeguard_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// BenchmarkRealRepositoryFullScan scans checked-out repository snapshots
// instead of a synthetic fixture, so a CPU profile shows where a real scan
// actually spends its time.
//
// Two ways to point it at a corpus; both are skipped when unset so
// `go test ./...` stays hermetic:
//
//	CODEGUARD_BENCH_CONFIGS  comma/colon-separated codeguard.yaml paths. The
//	                         config is used verbatim, so the profile covers
//	                         exactly the rule set a `codeguard scan` with that
//	                         config runs. Preferred — check-set differences
//	                         move full-scan cost by more than 2x.
//	CODEGUARD_BENCH_REPOS    comma/colon-separated repository roots, scanned
//	                         with the default (`codeguard init`) check set.
//
// Point either at FROZEN snapshots (`git archive HEAD | tar -x -C <dir>`),
// never a live working tree: a tree that mutates under the walk yields
// different findings run to run. Target paths inside a config must be
// absolute — a relative path resolves against the config file's directory.
func BenchmarkRealRepositoryFullScan(b *testing.B) {
	cases := realBenchmarkCases(b)
	if len(cases) == 0 {
		b.Skip("set CODEGUARD_BENCH_CONFIGS or CODEGUARD_BENCH_REPOS")
	}
	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			for _, variant := range []struct {
				name         string
				cacheEnabled bool
			}{
				{name: "cold", cacheEnabled: false},
				{name: "warm-cache", cacheEnabled: true},
			} {
				b.Run(variant.name, func(b *testing.B) {
					cfg := bc.cfg
					cfg.Cache.Enabled = &variant.cacheEnabled
					cfg.Cache.Path = filepath.Join(b.TempDir(), "benchmark-cache.json")
					if variant.cacheEnabled {
						if _, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull}); err != nil {
							b.Fatalf("warm cache for %s: %v", bc.name, err)
						}
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull}); err != nil {
							b.Fatalf("full scan %s: %v", bc.name, err)
						}
					}
				})
			}
		})
	}
}

type realBenchmarkCase struct {
	name string
	cfg  codeguard.Config
}

func realBenchmarkCases(tb testing.TB) []realBenchmarkCase {
	tb.Helper()
	var cases []realBenchmarkCase
	for _, path := range benchmarkEnvPaths("CODEGUARD_BENCH_CONFIGS") {
		cfg, err := codeguard.LoadConfigFile(path)
		if err != nil {
			tb.Fatalf("load benchmark config %q: %v", path, err)
		}
		cfg.Output.Format = "json"
		cases = append(cases, realBenchmarkCase{name: benchmarkCaseName(path), cfg: cfg})
	}
	for _, root := range benchmarkEnvPaths("CODEGUARD_BENCH_REPOS") {
		cases = append(cases, realBenchmarkCase{name: filepath.Base(root), cfg: defaultBenchmarkConfig(root)})
	}
	return cases
}

func benchmarkEnvPaths(name string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}
	fields := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ':' })
	paths := make([]string, 0, len(fields))
	for _, field := range fields {
		if path := strings.TrimSpace(field); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

// benchmarkCaseName labels a config case by its parent directory, so
// /tmp/cfg/szr/codeguard.yaml reports as "szr".
func benchmarkCaseName(path string) string {
	if dir := filepath.Base(filepath.Dir(path)); dir != "." && dir != string(filepath.Separator) {
		return dir
	}
	return filepath.Base(path)
}

// defaultBenchmarkConfig mirrors what `codeguard init` writes, with the target
// repointed at the snapshot. Nothing extra is disabled: the synthetic full-scan
// benchmark turns off dead code, local precision and the AI quality passes,
// which hides their cost.
func defaultBenchmarkConfig(root string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = "real-repository-benchmark"
	cfg.Targets = []codeguard.TargetConfig{{
		Name:     "repository",
		Path:     root,
		Language: detectBenchmarkLanguage(root),
	}}
	cfg.Checks.SecurityRules.GovulncheckMode = "off"
	cfg.Output.Format = "json"
	return cfg
}

func detectBenchmarkLanguage(root string) string {
	for _, candidate := range []struct {
		marker   string
		language string
	}{
		{marker: "go.mod", language: "go"},
		{marker: "tsconfig.json", language: "typescript"},
		{marker: "package.json", language: "typescript"},
		{marker: "pyproject.toml", language: "python"},
		{marker: "setup.py", language: "python"},
		{marker: "requirements.txt", language: "python"},
	} {
		if _, err := os.Stat(filepath.Join(root, candidate.marker)); err == nil {
			return candidate.language
		}
	}
	return "go"
}
