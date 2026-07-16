package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func writeCPPRebuildCascadeFixture(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "include", "common.hpp"), "#pragma once\n")
	writeFile(t, filepath.Join(dir, "include", "mid.hpp"), "#pragma once\n#include \"common.hpp\"\n")
	writeFile(t, filepath.Join(dir, "src", "alpha.cpp"), "#include \"../include/common.hpp\"\n")
	writeFile(t, filepath.Join(dir, "src", "beta.cpp"), "#include \"../include/common.hpp\"\n")
	writeFile(t, filepath.Join(dir, "src", "gamma.cpp"), "#include \"../include/mid.hpp\"\n")
}

func TestPerformanceCheckWarnsForCPPRebuildHotHeaderAndAmplifier(t *testing.T) {
	dir := t.TempDir()
	writeCPPRebuildCascadeFixture(t, dir)
	cfg := performanceConfig("performance-cpp-rebuild", dir, "cpp")
	cfg.Checks.PerformanceRules.HotPackageImporterThreshold = 2
	cfg.Checks.PerformanceRules.RebuildAmplifierThreshold = 3

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.cpp.hot-header")
	assertFindingRulePresent(t, report, "Performance", "performance.cpp.rebuild-amplifier")
}

func TestPerformanceCheckWarnsForCPPUnboundedThreadLaunchInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "workers.cpp"), "#include <thread>\n#include <vector>\nvoid run_all(const std::vector<int>& jobs) {\n  std::vector<std::thread> threads;\n  for (int job : jobs) {\n    threads.emplace_back(run, job);\n  }\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-cpp-threads", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.cpp.unbounded-concurrency")
}

func TestPerformanceCheckSkipsCPPRebuildCascadeBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	writeCPPRebuildCascadeFixture(t, dir)

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-cpp-rebuild-neg", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.cpp.hot-header")
	assertFindingRuleAbsent(t, report, "Performance", "performance.cpp.rebuild-amplifier")
}
