package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestPerformanceCheckWarnsForPythonRequestsInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "loader.py"),
		"import requests\n\n\ndef load(ids):\n    out = []\n    for item in ids:\n        out.append(requests.get(\"https://example.com/api/%s\" % item))\n    return out\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-py-nplusone", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.n-plus-one-query")
}

func TestPerformanceCheckWarnsForPythonBlockingCallInAsync(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "pause.py"),
		"import time\n\n\nasync def pause():\n    time.sleep(1)\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-py-sync-async", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.python.sync-io-in-async")
}

func TestPerformanceCheckWarnsForPythonRegexConcatTasksReads(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "batch.py"),
		"import asyncio\nimport re\n\n\nasync def process(files, urls, lines):\n    out = \"\"\n    for line in lines:\n        pattern = re.compile(r\"\\d+\")\n        if pattern.match(line):\n            out += line\n    for url in urls:\n        asyncio.create_task(fetch(url))\n    for f in files:\n        data = f.read()\n        use(data)\n    return out\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-py-batch", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.regex-compile-in-loop")
	assertFindingRulePresent(t, report, "Performance", "performance.string-concat-in-loop")
	assertFindingRulePresent(t, report, "Performance", "performance.python.unbounded-concurrency")
	assertFindingRulePresent(t, report, "Performance", "performance.unbounded-read")
}

func TestPerformanceCheckSkipsBoundedPythonPatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "clean_batch.py"),
		"import asyncio\n\nsem = asyncio.Semaphore(8)\n\n\nasync def process(files, urls, counts):\n    total = 0\n    for n in counts:\n        total += n\n    for url in urls:\n        asyncio.create_task(bounded_fetch(url))\n    for f in files:\n        data = f.read(65536)\n        use(data)\n    return total\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-py-bounded", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.string-concat-in-loop")
	assertFindingRuleAbsent(t, report, "Performance", "performance.python.unbounded-concurrency")
	assertFindingRuleAbsent(t, report, "Performance", "performance.unbounded-read")
}

func TestPerformanceCheckSkipsPythonPerformanceSmellsOutsideRegions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app", "clean.py"),
		"import time\n\nimport requests\n\n\ndef load_once(url):\n    return requests.get(url)\n\n\ndef sleepy():\n    time.sleep(1)\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-py-clean", dir, "python"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.n-plus-one-query")
	assertFindingRuleAbsent(t, report, "Performance", "performance.python.sync-io-in-async")
}
