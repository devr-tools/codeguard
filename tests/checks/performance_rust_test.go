package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestPerformanceCheckWarnsForRustRegexAllocAndSleepInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "render.rs"),
		"use regex::Regex;\nuse std::time::Duration;\n\nfn render(rows: &[String]) -> String {\n    let mut out = String::new();\n    for row in rows {\n        let digits = Regex::new(r\"[0-9]+$\").unwrap();\n        if digits.is_match(row) {\n            out.push_str(row);\n        }\n        std::thread::sleep(Duration::from_millis(1));\n    }\n    out\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-rust-loop-smells", dir, "rust"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.regex-compile-in-loop")
	assertFindingRulePresent(t, report, "Performance", "performance.rust.alloc-in-loop")
	assertFindingRulePresent(t, report, "Performance", "performance.rust.sleep-in-loop")
}

func TestPerformanceCheckSkipsPreallocatedRustStringGrowth(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "render.rs"),
		"fn render(rows: &[String]) -> String {\n    let mut out = String::with_capacity(rows.len() * 8);\n    for row in rows {\n        out.push_str(row);\n    }\n    out\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-rust-prealloc", dir, "rust"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.rust.alloc-in-loop")
	assertFindingRuleAbsent(t, report, "Performance", "performance.regex-compile-in-loop")
	assertFindingRuleAbsent(t, report, "Performance", "performance.rust.sleep-in-loop")
}
