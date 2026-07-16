package checks_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/runner/benchregression"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// fakeBenchOutput mimics real `go test -bench` output: headers, sub-benchmark
// names, GOMAXPROCS suffixes, float ns/op values, -benchmem columns, custom
// metrics, and trailer noise. The parser must ignore everything that is not a
// benchmark result line.
const fakeBenchOutput = `goos: darwin
goarch: arm64
pkg: example.com/sample
cpu: Apple M3
BenchmarkEncode-8          	 1000000	      1234 ns/op	     456 B/op	       7 allocs/op
BenchmarkDecode/small-8    	  500000	      2500.5 ns/op	    1024 B/op	      12 allocs/op
BenchmarkTiny-8            	1000000000	         0.5000 ns/op
BenchmarkWithMetric-8      	  200000	      9000 ns/op	        42.0 widgets/op	     128 B/op	       3 allocs/op
Benchmark
--- FAIL: BenchmarkBroken
PASS
ok  	example.com/sample	3.210s
`

func TestBenchmarkOutputParser(t *testing.T) {
	results := benchregression.ParseOutput(fakeBenchOutput)
	byName := map[string]benchregression.Result{}
	for _, result := range results {
		byName[result.Name] = result
	}
	if len(results) != 4 {
		t.Fatalf("parsed %d results, want 4: %+v", len(results), results)
	}
	encode := byName["BenchmarkEncode"]
	if encode.NsPerOp != 1234 || encode.BytesPerOp != 456 || encode.AllocsPerOp != 7 || encode.Iterations != 1000000 {
		t.Fatalf("BenchmarkEncode parsed wrong: %+v", encode)
	}
	if sub := byName["BenchmarkDecode/small"]; sub.NsPerOp != 2500.5 {
		t.Fatalf("sub-benchmark parsed wrong: %+v", sub)
	}
	if tiny := byName["BenchmarkTiny"]; tiny.NsPerOp != 0.5 {
		t.Fatalf("float ns/op parsed wrong: %+v", tiny)
	}
	if custom := byName["BenchmarkWithMetric"]; custom.NsPerOp != 9000 || custom.BytesPerOp != 128 {
		t.Fatalf("custom-metric line parsed wrong: %+v", custom)
	}
}

func TestBenchmarkComparator(t *testing.T) {
	baseline := map[string]benchregression.BaselineEntry{
		"BenchmarkStable":    {NsPerOp: 1000},
		"BenchmarkRegressed": {NsPerOp: 1000},
		"BenchmarkImproved":  {NsPerOp: 1000},
		"BenchmarkZero":      {NsPerOp: 0},
	}
	current := []benchregression.Result{
		{Name: "BenchmarkStable", NsPerOp: 1100},    // +10%: within the 20% threshold
		{Name: "BenchmarkRegressed", NsPerOp: 1500}, // +50%: regression
		{Name: "BenchmarkImproved", NsPerOp: 500},   // faster: never a finding
		{Name: "BenchmarkZero", NsPerOp: 100},       // zero baseline: skipped
		{Name: "BenchmarkNew", NsPerOp: 9999},       // absent from baseline: skipped
	}
	regressions := benchregression.Compare(baseline, current, 20)
	if len(regressions) != 1 {
		t.Fatalf("got %d regressions, want 1: %+v", len(regressions), regressions)
	}
	got := regressions[0]
	if got.Name != "BenchmarkRegressed" || got.Percent != 50 || got.BaselineNsPerOp != 1000 || got.CurrentNsPerOp != 1500 {
		t.Fatalf("regression fields wrong: %+v", got)
	}
}

func TestBenchmarkBaselineRoundTripAndMerge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.bench-baseline.json")
	results := []benchregression.Result{{Name: "BenchmarkA", NsPerOp: 100, BytesPerOp: 8, AllocsPerOp: 1}}
	if err := benchregression.WriteBaseline(path, results); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	baseline, ok := benchregression.LoadBaseline(path)
	if !ok || baseline["BenchmarkA"].NsPerOp != 100 {
		t.Fatalf("baseline roundtrip failed: ok=%v %+v", ok, baseline)
	}

	// Merging adds new names but never overwrites existing entries: a regressed
	// run must not silently become the new baseline.
	added, err := benchregression.MergeNewBenchmarks(path, baseline, []benchregression.Result{
		{Name: "BenchmarkA", NsPerOp: 999},
		{Name: "BenchmarkB", NsPerOp: 50},
	})
	if err != nil || !added {
		t.Fatalf("merge: added=%v err=%v", added, err)
	}
	merged, ok := benchregression.LoadBaseline(path)
	if !ok || merged["BenchmarkA"].NsPerOp != 100 || merged["BenchmarkB"].NsPerOp != 50 {
		t.Fatalf("merge result wrong: %+v", merged)
	}

	if _, ok := benchregression.LoadBaseline(filepath.Join(t.TempDir(), "missing.json")); ok {
		t.Fatal("missing baseline unexpectedly loaded")
	}
}

func TestBenchmarkBaselinePathDerivedFromCachePath(t *testing.T) {
	if got := benchregression.BaselinePathForBase(".codeguard/cache.json"); got != ".codeguard/cache.bench-baseline.json" {
		t.Fatalf("derived baseline path = %q", got)
	}
	if got := benchregression.BaselinePathForBase(""); got != "" {
		t.Fatalf("empty base should derive empty path, got %q", got)
	}
}

func benchmarksConfig(name string, dir string, benchmarks codeguard.PerformanceBenchmarksConfig) codeguard.Config {
	cfg := performanceConfig(name, dir, "go")
	cfg.Checks.PerformanceRules.Benchmarks = benchmarks
	return cfg
}

func TestPerformanceBenchmarksFullScanRequiresExplicitPackages(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lib.go"), "package sample\n")

	enabled := true
	report, err := codeguard.Run(context.Background(), benchmarksConfig("bench-no-packages", dir, codeguard.PerformanceBenchmarksConfig{
		Enabled: &enabled,
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "warn")
	assertFindingRulePresent(t, report, "Performance", "performance.benchmark-regression")
	assertFindingMessageContains(t, report, "performance.benchmark-regression", "explicit performance_rules.benchmarks.packages")
}

func assertFindingMessageContains(t *testing.T, report codeguard.Report, ruleID string, want string) {
	t.Helper()
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID && strings.Contains(finding.Message, want) {
				return
			}
		}
	}
	t.Fatalf("no %s finding containing %q", ruleID, want)
}

func TestPerformanceBenchmarksPackageValidation(t *testing.T) {
	dir := t.TempDir()
	for _, pkg := range []string{"-bench=evil", "/abs/path", "../outside", "./ok; rm -rf", "pkg"} {
		cfg := benchmarksConfig("bench-validate", dir, codeguard.PerformanceBenchmarksConfig{Packages: []string{pkg}})
		if err := codeguard.ValidateConfig(cfg); err == nil {
			t.Fatalf("package pattern %q unexpectedly validated", pkg)
		}
	}
	cfg := benchmarksConfig("bench-validate-ok", dir, codeguard.PerformanceBenchmarksConfig{Packages: []string{".", "./...", "./internal/..."}})
	if err := codeguard.ValidateConfig(cfg); err != nil {
		t.Fatalf("valid package patterns rejected: %v", err)
	}
}

// TestPerformanceBenchmarkRegressionEndToEnd runs the real gate against a
// trivial one-benchmark module: the first scan seeds the baseline and reports
// nothing; after the baseline is doctored to a near-zero ns/op, the second
// scan reports a regression. This is the only test that actually executes
// `go test -bench`; everything else goes through the pure parser/comparator.
func TestPerformanceBenchmarkRegressionEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("runs real go benchmarks")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go binary unavailable: %v", err)
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/benchsample\n\ngo 1.21\n")
	writeFile(t, filepath.Join(dir, "bench_test.go"), `package benchsample

import "testing"

func BenchmarkNoop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = i
	}
}
`)
	baselinePath := filepath.Join(dir, ".codeguard", "bench-baseline.json")
	enabled := true
	cfg := benchmarksConfig("bench-end-to-end", dir, codeguard.PerformanceBenchmarksConfig{
		Enabled:      &enabled,
		Packages:     []string{"."},
		BaselinePath: baselinePath,
	})

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.benchmark-regression")
	if _, statErr := os.Stat(baselinePath); statErr != nil {
		t.Fatalf("first run did not write the baseline: %v", statErr)
	}

	// Doctor the baseline to an impossible speed so the second run regresses.
	writeFile(t, baselinePath, `{"version": 1, "benchmarks": {"BenchmarkNoop": {"ns_per_op": 0.0000001}}}`)
	report, err = codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "warn")
	assertFindingMessageContains(t, report, "performance.benchmark-regression", "BenchmarkNoop regressed")
}
