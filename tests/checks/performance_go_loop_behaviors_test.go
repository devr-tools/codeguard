package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestPerformanceCheckSkipsDeferInsideGoroutineLiteral(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pool.go"),
		"package pool\n\nimport \"sync\"\n\nfunc RunAll(jobs []func()) {\n\tvar wg sync.WaitGroup\n\tfor _, job := range jobs {\n\t\twg.Add(1)\n\t\tgo func(run func()) {\n\t\t\tdefer wg.Done()\n\t\t\trun()\n\t\t}(job)\n\t}\n\twg.Wait()\n}\n")

	off := false
	cfg := performanceConfig("performance-go-defer-goroutine", dir, "go")
	cfg.Checks.PerformanceRules.DetectUnboundedConcurrency = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.go.defer-in-loop")
}

func TestPerformanceCheckWarnsForDeferInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "files.go"),
		"package files\n\nimport \"os\"\n\nfunc ReadAllFiles(paths []string) {\n\tfor _, path := range paths {\n\t\tf, err := os.Open(path)\n\t\tif err != nil {\n\t\t\tcontinue\n\t\t}\n\t\tdefer f.Close()\n\t}\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-defer-loop", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.go.defer-in-loop")
}

func TestPerformanceCheckWarnsForSleepAndTimerInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "poll.go"),
		"package poll\n\nimport \"time\"\n\nfunc WaitReady(ready func() bool) {\n\tfor !ready() {\n\t\ttime.Sleep(100 * time.Millisecond)\n\t}\n}\n\nfunc Drain(inbox chan int) {\n\tfor {\n\t\tselect {\n\t\tcase <-inbox:\n\t\tcase <-time.After(time.Second):\n\t\t\treturn\n\t\t}\n\t}\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-sleep-timer", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.go.sleep-in-loop")
	assertFindingRulePresent(t, report, "Performance", "performance.go.timer-leak-in-loop")
}

func TestPerformanceCheckWarnsForUnboundedReadInHandler(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.go"),
		"package api\n\nimport (\n\t\"io\"\n\t\"net/http\"\n)\n\nfunc Upload(w http.ResponseWriter, r *http.Request) {\n\tbody, err := io.ReadAll(r.Body)\n\tif err != nil {\n\t\treturn\n\t}\n\t_, _ = w.Write(body)\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-unbounded-read", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.unbounded-read")
}

func TestPerformanceCheckSkipsLimitedReadInHandler(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "handler.go"),
		"package api\n\nimport (\n\t\"io\"\n\t\"net/http\"\n)\n\nfunc Upload(w http.ResponseWriter, r *http.Request) {\n\tbody, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))\n\tif err != nil {\n\t\treturn\n\t}\n\t_, _ = w.Write(body)\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-limited-read", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.unbounded-read")
}

func TestPerformanceCheckNewGoTogglesOff(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "poll.go"),
		"package poll\n\nimport (\n\t\"regexp\"\n\t\"time\"\n)\n\nfunc Scan(lines []string) {\n\tfor _, line := range lines {\n\t\t_ = regexp.MustCompile(`x`).MatchString(line)\n\t\ttime.Sleep(time.Millisecond)\n\t\tdefer func() {}()\n\t\t<-time.After(time.Millisecond)\n\t}\n}\n")

	off := false
	cfg := performanceConfig("performance-go-new-toggles-off", dir, "go")
	cfg.Checks.PerformanceRules.DetectRegexCompileInLoop = &off
	cfg.Checks.PerformanceRules.DetectSleepInLoop = &off
	cfg.Checks.PerformanceRules.DetectDeferInLoop = &off
	cfg.Checks.PerformanceRules.DetectTimerLeaks = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.regex-compile-in-loop")
	assertFindingRuleAbsent(t, report, "Performance", "performance.go.sleep-in-loop")
	assertFindingRuleAbsent(t, report, "Performance", "performance.go.defer-in-loop")
	assertFindingRuleAbsent(t, report, "Performance", "performance.go.timer-leak-in-loop")
}
