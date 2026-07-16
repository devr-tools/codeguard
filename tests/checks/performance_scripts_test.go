package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

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
