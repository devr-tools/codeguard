package fix

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func inferPythonTestCommands(root string, changed []string, excludes []string, maxNearest int) []core.CommandCheckConfig {
	testFiles, err := runnersupport.WalkFiles(root, excludes, func(rel string) bool {
		return isPythonTestFile(rel)
	})
	if err != nil {
		return nil
	}

	selected := nearestOrFallbackRankedTests(changed, testFiles, maxNearest, pythonTestScore)
	checks := make([]core.CommandCheckConfig, 0, len(selected))
	for _, rel := range selected {
		name := "python3 -m unittest " + rel
		checks = append(checks, core.CommandCheckConfig{
			Name:    name,
			Command: "python3",
			Args:    []string{"-m", "unittest", rel},
		})
	}
	return checks
}

func inferScriptTestCommands(root string, changed []string, excludes []string, maxNearest int) []core.CommandCheckConfig {
	if checks := inferNodeTestCommands(root, changed, excludes, maxNearest); len(checks) > 0 {
		return checks
	}
	if check, ok := inferPackageManagerTestCommand(root); ok {
		return []core.CommandCheckConfig{check}
	}
	return nil
}

func inferNodeTestCommands(root string, changed []string, excludes []string, maxNearest int) []core.CommandCheckConfig {
	testFiles, err := runnersupport.WalkFiles(root, excludes, func(rel string) bool {
		return isNodeTestFile(rel)
	})
	if err != nil {
		return nil
	}

	selected := nearestOrFallbackRankedTests(changed, testFiles, maxNearest, scriptTestScore)
	if len(selected) == 0 {
		return nil
	}

	args := append([]string{"--test"}, selected...)
	return []core.CommandCheckConfig{{
		Name:    "node --test " + strings.Join(selected, " "),
		Command: "node",
		Args:    args,
	}}
}

func nearestOrFallbackRankedTests(changed []string, testFiles []string, maxNearest int, scorer func(string, string) int) []string {
	limit := maxNearest
	if limit <= 0 {
		limit = 3
	}

	selected := nearestRankedTestFiles(changed, testFiles, limit, scorer)
	if len(selected) > 0 {
		return selected
	}

	if len(testFiles) == 0 {
		return nil
	}
	sorted := append([]string(nil), testFiles...)
	slices.Sort(sorted)
	if limit > len(sorted) {
		limit = len(sorted)
	}
	return sorted[:limit]
}

func pythonTestScore(changedFile string, testFile string) int {
	if !strings.HasSuffix(changedFile, ".py") || isPythonTestFile(changedFile) {
		return 0
	}
	return genericTestScore(changedFile, testFile)
}

func scriptTestScore(changedFile string, testFile string) int {
	if !hasAnySuffix(changedFile, []string{".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts"}) || isNodeTestFile(changedFile) {
		return 0
	}
	return genericTestScore(changedFile, testFile)
}

func isPythonTestFile(rel string) bool {
	base := filepath.Base(rel)
	return strings.HasSuffix(base, "_test.py") || strings.HasPrefix(base, "test_")
}

func isNodeTestFile(rel string) bool {
	base := filepath.Base(rel)
	return hasAnySuffix(base, []string{
		".test.js", ".spec.js",
		".test.mjs", ".spec.mjs",
		".test.cjs", ".spec.cjs",
	})
}

func hasAnySuffix(value string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}
