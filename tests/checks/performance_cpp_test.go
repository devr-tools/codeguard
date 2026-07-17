package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestPerformanceCheckWarnsForCPPRegexAllocAndSleepInLoop(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "render.cpp"),
		"#include <regex>\n#include <string>\n#include <thread>\n#include <chrono>\n#include <vector>\n#include <sstream>\n\nstd::string render(const std::vector<std::string>& rows) {\n    std::ostringstream out;\n    for (const auto& row : rows) {\n        std::regex digits(\"[0-9]+$\");\n        if (std::regex_search(row, digits)) {\n            out << row << std::endl;\n        }\n        std::this_thread::sleep_for(std::chrono::milliseconds(1));\n    }\n    return out.str();\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-cpp-loop-smells", dir, "c++"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.regex-compile-in-loop")
	assertFindingRulePresent(t, report, "Performance", "performance.cpp.flush-in-loop")
	assertFindingRulePresent(t, report, "Performance", "performance.cpp.sleep-in-loop")
}

func TestPerformanceCheckSkipsReservedCPPStringGrowth(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "render.cpp"),
		"#include <string>\n#include <vector>\n\nstd::string render(const std::vector<std::string>& rows) {\n    std::string out;\n    out.reserve(rows.size() * 8);\n    for (const auto& row : rows) {\n        out.append(row);\n    }\n    return out;\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-cpp-reserved", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.cpp.alloc-in-loop")
	assertFindingRuleAbsent(t, report, "Performance", "performance.regex-compile-in-loop")
	assertFindingRuleAbsent(t, report, "Performance", "performance.cpp.flush-in-loop")
	assertFindingRuleAbsent(t, report, "Performance", "performance.cpp.sleep-in-loop")
}

func TestPerformanceCheckWarnsForCPPRangeForCopy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "pairs.cpp"),
		"#include <map>\n#include <string>\n\nvoid emit(const std::map<std::string, std::string>& values) {\n    for (auto [key, value] : values) {\n        (void)key;\n        (void)value;\n    }\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-cpp-range-copy", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRulePresent(t, report, "Performance", "performance.cpp.range-for-copy")
}

func TestPerformanceCheckSkipsCPPRangeForReference(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "pairs.cpp"),
		"#include <map>\n#include <string>\n\nvoid emit(const std::map<std::string, std::string>& values) {\n    for (const auto& [key, value] : values) {\n        (void)key;\n        (void)value;\n    }\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-cpp-range-copy-neg", dir, "cpp"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.cpp.range-for-copy")
}
