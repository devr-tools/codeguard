package fix

import (
	"path/filepath"
	"strings"
)

func genericTestScore(changedFile string, testFile string) int {
	return scoredTestMatch(
		filepath.ToSlash(filepath.Dir(changedFile)),
		filepath.ToSlash(filepath.Dir(testFile)),
		testableBase(filepath.Base(changedFile)),
		testableBase(filepath.Base(testFile)),
	)
}

func scoredTestMatch(changedDir string, testDir string, changedBase string, testBase string) int {
	score := 10
	if changedDir == testDir {
		score += 100
	}
	if changedBase == testBase {
		score += 60
	}
	if strings.HasPrefix(testBase, changedBase) || strings.HasPrefix(changedBase, testBase) {
		score += 25
	}
	score -= pathDistance(changedDir, testDir) * 5
	return score
}

func testableBase(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	for _, suffix := range []string{"_test", ".test", ".spec"} {
		base = strings.TrimSuffix(base, suffix)
	}
	base = strings.TrimPrefix(base, "test_")
	return base
}
