package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestPerformanceCheckWarnsForTypeScriptFetchInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "users.ts"),
		"export async function loadUsers(ids: string[]) {\n  const users = [];\n  for (const id of ids) {\n    const res = await fetch(`/api/users/${id}`);\n    users.push(await res.json());\n  }\n  return users;\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-nplusone", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.n-plus-one-query")
}

func TestPerformanceCheckWarnsForTypeScriptSyncIOInHandler(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "server.ts"),
		"import fs from \"fs\";\nimport express from \"express\";\n\nconst app = express();\n\napp.get(\"/report\", (req, res) => {\n  const data = fs.readFileSync(\"report.txt\", \"utf8\");\n  res.send(data);\n});\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-sync-io", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.typescript.sync-io-in-handler")
}

func TestPerformanceCheckWarnsForTypeScriptUnboundedConcurrency(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "warm.ts"),
		"export function warm(urls: string[]) {\n  const tasks = [];\n  for (const url of urls) {\n    tasks.push(fetch(url));\n  }\n  return Promise.all(tasks);\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-unbounded", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Performance", "performance.typescript.unbounded-concurrency")
}

func TestPerformanceCheckSkipsTypeScriptPerformanceSmellsOutsideRegions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "clean.ts"),
		"import fs from \"fs\";\n\nconst config = fs.readFileSync(\"config.json\", \"utf8\");\n\nexport async function loadUser(id: string) {\n  const res = await fetch(`/api/users/${id}`);\n  return res.json();\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-clean", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.n-plus-one-query")
	assertFindingRuleAbsent(t, report, "Performance", "performance.typescript.sync-io-in-handler")
	assertFindingRuleAbsent(t, report, "Performance", "performance.typescript.unbounded-concurrency")
}

func TestPerformanceCheckSkipsTypeScriptUnboundedConcurrencyWithPLimit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "warm.ts"),
		"import pLimit from \"p-limit\";\n\nconst limit = pLimit(4);\n\nexport function warm(urls: string[]) {\n  const tasks = [];\n  for (const url of urls) {\n    tasks.push(limit(() => fetch(url)));\n  }\n  return Promise.all(tasks);\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-plimit", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Performance", "performance.typescript.unbounded-concurrency")
}

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

func TestPerformanceCheckWarnsForTypeScriptAwaitInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "serial.ts"),
		"export async function loadUsers(ids: string[]) {\n  const users = [];\n  for (const id of ids) {\n    const user = await loadUser(id);\n    users.push(user);\n  }\n  return users;\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-await-loop", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.typescript.await-in-loop")
}

func TestPerformanceCheckSkipsForAwaitStreams(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "stream.ts"),
		"export async function drain(stream: AsyncIterable<string>) {\n  for await (const chunk of stream) {\n    consume(chunk);\n  }\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-for-await", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.typescript.await-in-loop")
}

func TestPerformanceCheckWarnsForTypeScriptRegexAndConcatInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "render.ts"),
		"export function render(rows: string[]) {\n  let out = \"\";\n  for (const row of rows) {\n    const re = new RegExp(\"[0-9]+\");\n    if (re.test(row)) {\n      out += row + \"\\n\";\n    }\n  }\n  return out;\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-regex-concat", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.regex-compile-in-loop")
	assertFindingRulePresent(t, report, "Performance", "performance.string-concat-in-loop")
}

func TestPerformanceCheckWarnsForTypeScriptTimerAndListenerLeaks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "poll.ts"),
		"export function start(items: Element[]) {\n  setInterval(refresh, 1000);\n  for (const el of items) {\n    el.addEventListener(\"click\", onClick);\n  }\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-leaks", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.typescript.timer-listener-leak")
}

func TestPerformanceCheckSkipsCleanedUpTimersAndListeners(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "poll.ts"),
		"export function start(items: Element[]) {\n  const handle = setInterval(refresh, 1000);\n  const controller = new AbortController();\n  for (const el of items) {\n    el.addEventListener(\"click\", onClick, { signal: controller.signal });\n  }\n  return () => {\n    clearInterval(handle);\n    controller.abort();\n  };\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-ts-leaks-cleaned", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.typescript.timer-listener-leak")
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
