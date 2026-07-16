package quality

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

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

func readFrameworkFile(env support.Context, target core.TargetConfig, rel string, include func(string) bool, detect func(string) string) (string, bool) {
	if !include(rel) {
		return "", false
	}
	data, err := readAITargetFile(env, target, rel)
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
	return []core.Finding{warnFinding(env, "quality.ai.local-idiom-drift", file, 1, 1,
		fmt.Sprintf("test uses %s while the repository primarily uses %s", actual, dominant))}
}
