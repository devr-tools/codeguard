package checks_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

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
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "match.go"),
		"package match\n\nimport \"regexp\"\n\nfunc CompileAll(patterns []string) []*regexp.Regexp {\n\tout := make([]*regexp.Regexp, 0, len(patterns))\n\tfor _, pattern := range patterns {\n\t\tre, err := regexp.Compile(pattern)\n\t\tif err != nil {\n\t\t\tcontinue\n\t\t}\n\t\tout = append(out, re)\n\t}\n\treturn out\n}\n")

	report, err := codeguard.Run(context.Background(), performanceConfig("performance-go-regex-variable", dir, "go"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertFindingRuleAbsent(t, report, "Performance", "performance.regex-compile-in-loop")
}
