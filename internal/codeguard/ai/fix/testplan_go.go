package fix

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func inferGoTestCommands(root string, changed []string, excludes []string, maxNearest int) []core.CommandCheckConfig {
	testFiles, err := runnersupport.WalkFiles(root, excludes, func(rel string) bool {
		return strings.HasSuffix(rel, "_test.go")
	})
	if err != nil {
		return nil
	}

	selected := nearestOrFallbackGoTests(changed, testFiles, maxNearest)
	checks := make([]core.CommandCheckConfig, 0, len(selected))
	for _, dir := range selected {
		pattern, name := goTestPattern(filepath.ToSlash(dir))
		checks = append(checks, core.CommandCheckConfig{
			Name:    name,
			Command: "go",
			Args:    []string{"test", pattern},
		})
	}
	return checks
}

func nearestOrFallbackGoTests(changed []string, testFiles []string, maxNearest int) []string {
	limit := maxNearest
	if limit <= 0 {
		limit = 3
	}

	selected := nearestGoTestFiles(changed, testFiles, limit)
	if len(selected) == 0 {
		return fallbackGoPackageDirs(changed)
	}
	return uniquePackageDirs(selected)
}

func nearestGoTestFiles(changed []string, testFiles []string, limit int) []string {
	return nearestRankedTestFiles(changed, testFiles, limit, goTestScore)
}

func goTestScore(changedFile string, testFile string) int {
	if !strings.HasSuffix(changedFile, ".go") || strings.HasSuffix(changedFile, "_test.go") {
		return 0
	}
	return scoredTestMatch(
		filepath.ToSlash(filepath.Dir(changedFile)),
		filepath.ToSlash(filepath.Dir(testFile)),
		strings.TrimSuffix(filepath.Base(changedFile), ".go"),
		strings.TrimSuffix(filepath.Base(testFile), "_test.go"),
	)
}

func fallbackGoPackageDirs(changed []string) []string {
	dirs := make([]string, 0, len(changed))
	for _, rel := range changed {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(rel))
		if dir == "" {
			dir = "."
		}
		dirs = append(dirs, dir)
	}
	return uniquePackageDirs(dirs)
}
