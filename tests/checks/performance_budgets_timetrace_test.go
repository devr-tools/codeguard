package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestPerformanceBudgetClangTimeTraceUnderBudgetPasses(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "trace.json"), `{"traceEvents":[{"name":"ExecuteCompiler","ph":"X","ts":0,"dur":9000}]}`)

	report, err := codeguard.Run(context.Background(), budgetConfig("trace-under", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "clang-build", Kind: "clang-time-trace", Path: "trace.json", MaxMilliseconds: 10},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "pass")
	assertFindingRuleAbsent(t, report, "Performance", "performance.budget")
}

func TestPerformanceBudgetClangTimeTraceOverBudgetWarns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "trace.json"), `{"traceEvents":[{"name":"ExecuteCompiler","ph":"X","ts":1000,"dur":45000},{"name":"Frontend","ph":"X","ts":5000,"dur":12000}]}`)

	report, err := codeguard.Run(context.Background(), budgetConfig("trace-over", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "clang-build", Kind: "clang-time-trace", Path: "trace.json", MaxMilliseconds: 20},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "warn")
	findBudgetFindingMessage(t, report, "max_milliseconds budget of 20")
}

func TestPerformanceBudgetClangTimeTraceEventBudget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "trace-a.json"), `{"traceEvents":[{"name":"Frontend","ph":"X","ts":0,"dur":7000}]}`)
	writeFile(t, filepath.Join(dir, "trace-b.json"), `{"traceEvents":[{"name":"Frontend","ph":"X","ts":0,"dur":8000}]}`)

	report, err := codeguard.Run(context.Background(), budgetConfig("trace-event", dir, []codeguard.PerformanceBudgetConfig{
		{Name: "frontend-total", Kind: "clang-time-trace", Path: "trace-*.json", Event: "Frontend", MaxMilliseconds: 10},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSectionStatus(t, report, "Performance", "warn")
	findBudgetFindingMessage(t, report, `events named "Frontend" total 15.0 ms`)
}

func TestPerformanceBudgetValidationRejectsBadClangTimeTraceEntries(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		label  string
		budget codeguard.PerformanceBudgetConfig
		want   string
	}{
		{"missing max_milliseconds", codeguard.PerformanceBudgetConfig{Name: "x", Kind: "clang-time-trace", Path: "trace.json"}, "max_milliseconds must be positive"},
		{"event on file-size", codeguard.PerformanceBudgetConfig{Name: "x", Kind: "file-size", Path: "a", Event: "Frontend", MaxBytes: 1}, "event only applies"},
	}
	for _, tc := range cases {
		cfg := budgetConfig("budget-validate-trace", dir, []codeguard.PerformanceBudgetConfig{tc.budget})
		err := codeguard.ValidateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: expected error containing %q, got %v", tc.label, tc.want, err)
		}
	}
}
