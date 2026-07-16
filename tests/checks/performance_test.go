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

func TestPerformanceCheckWarnsForGoQueryInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "repo.go"),
		"package repo\n\nimport \"database/sql\"\n\nfunc UpdateAll(db *sql.DB, ids []int) error {\n\tfor _, id := range ids {\n\t\tif _, err := db.Exec(\"UPDATE items SET done = 1 WHERE id = ?\", id); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\treturn nil\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-nplusone", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.n-plus-one-query")
}

func TestPerformanceCheckSkipsGoQueryOutsideLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "repo.go"),
		"package repo\n\nimport \"database/sql\"\n\nfunc UpdateOne(db *sql.DB, id int) error {\n\t_, err := db.Exec(\"UPDATE items SET done = 1 WHERE id = ?\", id)\n\treturn err\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-nplusone-neg", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.n-plus-one-query")
}

func TestPerformanceCheckWarnsForGoAllocInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "report.go"),
		"package report\n\nimport \"fmt\"\n\nfunc Describe(items []string) string {\n\tout := \"\"\n\tfor _, item := range items {\n\t\tout += fmt.Sprintf(\"- %s\\n\", item)\n\t}\n\treturn out\n}\n\nfunc Gather(items []string) []string {\n\tvar values []string\n\tfor _, item := range items {\n\t\tvalues = append(values, item)\n\t}\n\treturn values\n}\n")

	on := true
	cfg := performanceConfig("performance-go-alloc", dir, "go")
	cfg.Checks.PerformanceRules.DetectPreallocInLoop = &on

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.go.alloc-in-loop")
}

func TestPerformanceCheckWarnsForAppendWithoutPreallocWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "report.go"),
		"package report\n\nfunc Gather(items []string) []string {\n\tvar values []string\n\tfor _, item := range items {\n\t\tvalues = append(values, item)\n\t}\n\treturn values\n}\n")

	on := true
	cfg := performanceConfig("performance-go-prealloc-on", dir, "go")
	cfg.Checks.PerformanceRules.DetectPreallocInLoop = &on

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.go.alloc-in-loop")
}

func TestPerformanceCheckPreallocInLoopDefaultOff(t *testing.T) {
	appendDir := t.TempDir()
	writeFile(t, filepath.Join(appendDir, "report.go"),
		"package report\n\nfunc Gather(items []string) []string {\n\tvar values []string\n\tfor _, item := range items {\n\t\tvalues = append(values, item)\n\t}\n\treturn values\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-prealloc-default", appendDir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.go.alloc-in-loop")

	concatDir := t.TempDir()
	writeFile(t, filepath.Join(concatDir, "report.go"),
		"package report\n\nfunc Describe(items []string) string {\n\tout := \"\"\n\tfor _, item := range items {\n\t\tout += \"- \" + item\n\t}\n\treturn out\n}\n")

	report, err = codeguard.Run(context.Background(), performanceConfig("performance-go-concat-default", concatDir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.go.alloc-in-loop")
}

func TestPerformanceCheckSkipsPreallocatedAppendInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "report.go"),
		"package report\n\nfunc Gather(items []string) []string {\n\tvalues := make([]string, 0, len(items))\n\tfor _, item := range items {\n\t\tvalues = append(values, item)\n\t}\n\treturn values\n}\n")

	on := true
	cfg := performanceConfig("performance-go-alloc-neg", dir, "go")
	cfg.Checks.PerformanceRules.DetectPreallocInLoop = &on

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.go.alloc-in-loop")
}

func TestPerformanceCheckAllocInLoopToggleOff(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "report.go"),
		"package report\n\nimport \"fmt\"\n\nfunc Describe(items []string) string {\n\tout := \"\"\n\tfor _, item := range items {\n\t\tout += fmt.Sprintf(\"- %s\\n\", item)\n\t}\n\treturn out\n}\n")

	off := false
	cfg := performanceConfig("performance-go-alloc-off", dir, "go")
	cfg.Checks.PerformanceRules.DetectAllocInLoop = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.go.alloc-in-loop")
}

func TestPerformanceCheckWarnsForRegexCompileInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "match.go"),
		"package match\n\nimport \"regexp\"\n\nfunc CountDigits(lines []string) int {\n\ttotal := 0\n\tfor _, line := range lines {\n\t\tre := regexp.MustCompile(`[0-9]+`)\n\t\tif re.MatchString(line) {\n\t\t\ttotal++\n\t\t}\n\t}\n\treturn total\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-regex-loop", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.regex-compile-in-loop")
}

func TestPerformanceCheckSkipsHoistedRegexCompile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "match.go"),
		"package match\n\nimport \"regexp\"\n\nvar digits = regexp.MustCompile(`[0-9]+`)\n\nfunc CountDigits(lines []string) int {\n\ttotal := 0\n\tfor _, line := range lines {\n\t\tif digits.MatchString(line) {\n\t\t\ttotal++\n\t\t}\n\t}\n\treturn total\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-regex-hoisted", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.regex-compile-in-loop")
}

func TestPerformanceCheckSkipsVariablePatternRegexCompileInLoop(t *testing.T) {
	// Compiling N different config-supplied patterns in a loop over the
	// patterns is not the hoistable smell; only literal patterns flag.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "match.go"),
		"package match\n\nimport \"regexp\"\n\nfunc CompileAll(patterns []string) []*regexp.Regexp {\n\tout := make([]*regexp.Regexp, 0, len(patterns))\n\tfor _, pattern := range patterns {\n\t\tre, err := regexp.Compile(pattern)\n\t\tif err != nil {\n\t\t\tcontinue\n\t\t}\n\t\tout = append(out, re)\n\t}\n\treturn out\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-regex-variable", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.regex-compile-in-loop")
}

func TestPerformanceCheckSkipsDeferInsideGoroutineLiteral(t *testing.T) {
	// defer scopes to the enclosing function: defer wg.Done() inside a
	// goroutine launched from a loop runs per goroutine and never accumulates.
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

func TestQualityCheckNoLongerEmitsPerformanceRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "repo.go"),
		"package repo\n\nimport \"database/sql\"\n\nfunc UpdateAll(db *sql.DB, ids []int) error {\n\tfor _, id := range ids {\n\t\tif _, err := db.Exec(\"UPDATE items SET done = 1 WHERE id = ?\", id); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\treturn nil\n}\n")

	cfg := performanceConfig("quality-no-perf-rules", dir, "go")
	cfg.Checks.Performance = boolPtr(false)
	cfg.Checks.Quality = true

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Code Quality", "quality.n-plus-one-query")
	assertFindingRuleAbsent(t, report, "Code Quality", "performance.n-plus-one-query")
}
