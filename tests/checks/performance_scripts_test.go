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
