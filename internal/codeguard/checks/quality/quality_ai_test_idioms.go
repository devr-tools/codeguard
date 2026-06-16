package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func dominantGoTestFramework(root string, files []string) string {
	return dominantFramework(root, files, func(rel string, data string) (string, bool) {
		return goTestFramework(data), strings.HasSuffix(rel, "_test.go")
	})
}

func goTestFramework(source string) string {
	switch {
	case strings.Contains(source, "github.com/onsi/ginkgo"):
		return "ginkgo"
	case strings.Contains(source, "github.com/stretchr/testify/suite"):
		return "testify-suite"
	case strings.Contains(source, `"testing"`):
		return "testing"
	default:
		return ""
	}
}

func goIdiomDriftFinding(env support.Context, file string, source string, dominant string) []core.Finding {
	return idiomDriftFinding(env, file, dominant, goTestFramework(source))
}

func countMarkers(source string, markers []string) int {
	total := 0
	for _, marker := range markers {
		total += strings.Count(source, marker)
	}
	return total
}

func dominantFramework(root string, files []string, detector func(string, string) (string, bool)) string {
	counts := map[string]int{}
	for _, rel := range files {
		framework, include := readFrameworkFile(root, rel, func(string) bool { return true }, func(data string) string {
			framework, _ := detector(rel, data)
			return framework
		})
		if !include || framework == "" {
			continue
		}
		counts[framework]++
	}
	return dominantFrameworkFromCounts(counts)
}

func readFrameworkFile(root string, rel string, include func(string) bool, detect func(string) string) (string, bool) {
	if !include(rel) {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return "", false
	}
	return detect(string(data)), true
}

func dominantFrameworkFromCounts(counts map[string]int) string {
	bestName := ""
	bestCount := 0
	for name, count := range counts {
		if count > bestCount {
			bestName = name
			bestCount = count
		}
	}
	return bestName
}

func idiomDriftFinding(env support.Context, file string, dominant string, actual string) []core.Finding {
	if dominant == "" || actual == "" || actual == dominant {
		return nil
	}
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "quality.ai.local-idiom-drift",
		Level:   "warn",
		Path:    file,
		Line:    1,
		Column:  1,
		Message: fmt.Sprintf("test uses %s while the repository primarily uses %s", actual, dominant),
	})}
}
