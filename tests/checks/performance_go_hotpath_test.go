package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestPerformanceCheckWarnsForGoSliceMembershipInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "filter.go"),
		"package filter\n\nimport \"slices\"\n\nfunc KeepAllowed(ids []int, allowed []int) []int {\n\tout := make([]int, 0, len(ids))\n\tfor _, id := range ids {\n\t\tif slices.Contains(allowed, id) {\n\t\t\tout = append(out, id)\n\t\t}\n\t}\n\treturn out\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-slice-membership", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.go.slice-membership-in-loop")
}

func TestPerformanceCheckWarnsForGoNestedLoopMembershipScan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "filter.go"),
		"package filter\n\nfunc KeepAllowed(ids []int, allowed []int) []int {\n\tout := make([]int, 0, len(ids))\n\tfor _, id := range ids {\n\t\tfor _, allowedID := range allowed {\n\t\t\tif id == allowedID {\n\t\t\t\tout = append(out, id)\n\t\t\t\tbreak\n\t\t\t}\n\t\t}\n\t}\n\treturn out\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-nested-loop-scan", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.go.nested-loop-scan")
}

func TestPerformanceCheckSkipsNestedLoopWithoutMembershipCompare(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "matrix.go"),
		"package matrix\n\nfunc SumPairs(xs []int, ys []int) int {\n\tout := 0\n\tfor _, x := range xs {\n\t\tfor _, y := range ys {\n\t\t\tout += x + y\n\t\t}\n\t}\n\treturn out\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-nested-loop-neg", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.go.nested-loop-scan")
}

func TestPerformanceCheckSkipsGoSliceMembershipOutsideLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "filter.go"),
		"package filter\n\nimport \"slices\"\n\nfunc IsAllowed(id int, allowed []int) bool {\n\treturn slices.Contains(allowed, id)\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-slice-membership-neg", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.go.slice-membership-in-loop")
}

func TestPerformanceCheckWarnsForGoLockHeldAcrossBlockingCall(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "state.go"),
		"package state\n\nimport (\n\t\"os\"\n\t\"sync\"\n)\n\nfunc Load(path string) ([]byte, error) {\n\tvar mu sync.Mutex\n\tmu.Lock()\n\tdefer mu.Unlock()\n\treturn os.ReadFile(path)\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-lock-held", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.go.lock-held-across-blocking-call")
}

func TestPerformanceCheckSkipsGoBlockingCallAfterUnlock(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "state.go"),
		"package state\n\nimport (\n\t\"os\"\n\t\"sync\"\n)\n\nfunc Load(path string) ([]byte, error) {\n\tvar mu sync.Mutex\n\tmu.Lock()\n\tpathCopy := path\n\tmu.Unlock()\n\treturn os.ReadFile(pathCopy)\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-lock-held-neg", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.go.lock-held-across-blocking-call")
}

func TestPerformanceCheckHotPathPatternsToggleOff(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "filter.go"),
		"package filter\n\nimport \"slices\"\n\nfunc KeepAllowed(ids []int, allowed []int) []int {\n\tfor _, id := range ids {\n\t\tif slices.Contains(allowed, id) {\n\t\t\tfor _, allowedID := range allowed {\n\t\t\t\tif id == allowedID {\n\t\t\t\t\treturn []int{id}\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n\treturn nil\n}\n")

	off := false
	cfg := performanceConfig("performance-go-hot-path-off", dir, "go")
	cfg.Checks.PerformanceRules.DetectHotPathPatterns = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.go.slice-membership-in-loop")
	assertFindingRuleAbsent(t, report, "Performance", "performance.go.nested-loop-scan")
}

func TestPerformanceCheckHotPathPatternsToggleOffForLockHeld(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "state.go"),
		"package state\n\nimport (\n\t\"os\"\n\t\"sync\"\n)\n\nfunc Load(path string) ([]byte, error) {\n\tvar mu sync.Mutex\n\tmu.Lock()\n\tdefer mu.Unlock()\n\treturn os.ReadFile(path)\n}\n")

	off := false
	cfg := performanceConfig("performance-go-hot-path-lock-off", dir, "go")
	cfg.Checks.PerformanceRules.DetectHotPathPatterns = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.go.lock-held-across-blocking-call")
}
