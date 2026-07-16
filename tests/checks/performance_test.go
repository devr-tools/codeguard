package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// performanceConfig enables only the performance section (which is opt-in by
// default) so section assertions are isolated from the other check families.
func performanceConfig(name string, dir string, language string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.Performance = boolPtr(true)
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	return cfg
}

func TestPerformanceCheckOffByDefault(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "worker.go"), `package sample

func dispatch(items []int) {
	for _, item := range items {
		go func(value int) {
			_ = value
		}(item)
	}
}
`)

	cfg := performanceConfig("performance-default-off", dir, "go")
	cfg.Checks.Performance = nil

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, section := range report.Sections {
		if section.Name == "Performance" {
			t.Fatal("performance section ran despite checks.performance being false")
		}
	}
}

func TestPerformanceCheckWarnsForSyncIOInRequestPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.go"), `package sample

import (
	"net/http"
	"os"
)

func handle(w http.ResponseWriter, r *http.Request) {
	_, _ = os.ReadFile("payload.json")
	_, _ = w.Write([]byte("ok"))
}
`)

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-sync-io-request-path", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Performance", "warn")
	assertFindingRulePresent(t, report, "Performance", "performance.sync-io-in-request-path")
}

func TestPerformanceCheckWarnsForGoroutinesInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "worker.go"), `package sample

func dispatch(items []int) {
	for _, item := range items {
		go func(value int) {
			_ = value
		}(item)
	}
}
`)

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-unbounded-goroutines", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Performance", "warn")
	assertFindingRulePresent(t, report, "Performance", "performance.unbounded-goroutines-in-loop")
}

func TestPerformanceCheckSkipsBoundedWorkerPools(t *testing.T) {
	// Counted loops and semaphore-acquiring loops construct bounded worker
	// pools; the unbounded rule targets data-driven goroutine fan-out.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pool.go"),
		"package pool\n\nimport \"sync\"\n\nfunc CountedRange(workers int) {\n\tvar wg sync.WaitGroup\n\tfor range workers {\n\t\twg.Add(1)\n\t\tgo func() { defer wg.Done() }()\n\t}\n\twg.Wait()\n}\n\nfunc CountedFor(workers int) {\n\tvar wg sync.WaitGroup\n\tfor i := 0; i < workers; i++ {\n\t\twg.Add(1)\n\t\tgo func() { defer wg.Done() }()\n\t}\n\twg.Wait()\n}\n\nfunc SemaphoreBounded(jobs []func()) {\n\tsem := make(chan struct{}, 4)\n\tvar wg sync.WaitGroup\n\tfor _, job := range jobs {\n\t\twg.Add(1)\n\t\tsem <- struct{}{}\n\t\tgo func(run func()) {\n\t\t\tdefer wg.Done()\n\t\t\tdefer func() { <-sem }()\n\t\t\trun()\n\t\t}(job)\n\t}\n\twg.Wait()\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-bounded-pools", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.unbounded-goroutines-in-loop")
}

func TestPerformanceCheckStillFlagsDataDrivenCountedBound(t *testing.T) {
	// A len()-bounded counted loop is data-driven fan-out, not a worker pool.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "fanout.go"),
		"package fanout\n\nfunc Dispatch(items []int) {\n\tfor i := 0; i < len(items); i++ {\n\t\tgo process(items[i])\n\t}\n}\n\nfunc process(int) {}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-len-bound", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.unbounded-goroutines-in-loop")
}

func TestPerformanceCheckSkipsSleepInLoopInTests(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wait_test.go"),
		"package wait\n\nimport \"time\"\n\nfunc waitReady(ready func() bool) {\n\tfor !ready() {\n\t\ttime.Sleep(10 * time.Millisecond)\n\t}\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-sleep-test-exempt", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.go.sleep-in-loop")
}

func TestPerformanceCheckGoTogglesGateGoRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "worker.go"), `package sample

import (
	"net/http"
	"os"
)

func handle(w http.ResponseWriter, r *http.Request) {
	_, _ = os.ReadFile("payload.json")
	for range []int{1, 2} {
		go func() {}()
	}
}
`)

	off := false
	cfg := performanceConfig("performance-go-toggles-off", dir, "go")
	cfg.Checks.PerformanceRules.DetectUnboundedConcurrency = &off
	cfg.Checks.PerformanceRules.DetectSyncIOInHandlers = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.unbounded-goroutines-in-loop")
	assertFindingRuleAbsent(t, report, "Performance", "performance.sync-io-in-request-path")
}
