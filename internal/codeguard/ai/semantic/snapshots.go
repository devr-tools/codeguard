package semantic

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func collectSnapshots(root string, changedFiles []string) ([]FileSnapshot, []FileSnapshot) {
	sourcePaths := make([]string, 0)
	testCandidates := map[string]struct{}{}
	for _, rel := range changedFiles {
		if isTestPath(rel) {
			testCandidates[rel] = struct{}{}
			continue
		}
		sourcePaths = append(sourcePaths, rel)
		for _, candidate := range relatedTestCandidates(rel) {
			testCandidates[candidate] = struct{}{}
		}
	}
	sort.Strings(sourcePaths)
	sourceFiles := snapshotsForPaths(root, sourcePaths, 24_000)
	testPaths := make([]string, 0, len(testCandidates))
	for rel := range testCandidates {
		testPaths = append(testPaths, rel)
	}
	sort.Strings(testPaths)
	testFiles := snapshotsForPaths(root, testPaths, 16_000)
	return sourceFiles, testFiles
}

func snapshotsForPaths(root string, paths []string, maxBytes int) []FileSnapshot {
	snapshots := make([]FileSnapshot, 0, len(paths))
	for _, rel := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		content := string(data)
		if len(content) > maxBytes {
			content = content[:maxBytes]
		}
		snapshots = append(snapshots, FileSnapshot{
			Path:    filepath.ToSlash(rel),
			Content: content,
		})
	}
	return snapshots
}

func relatedTestCandidates(rel string) []string {
	dir := filepath.ToSlash(filepath.Dir(rel))
	base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	ext := filepath.Ext(rel)
	candidates := []string{
		filepath.ToSlash(filepath.Join(dir, base+"_test"+ext)),
		filepath.ToSlash(filepath.Join(dir, "test_"+base+ext)),
		filepath.ToSlash(filepath.Join(dir, base+".test"+ext)),
		filepath.ToSlash(filepath.Join(dir, base+".spec"+ext)),
	}
	trimmed := strings.TrimSuffix(base, ".tsx")
	if trimmed != base {
		candidates = append(candidates, filepath.ToSlash(filepath.Join(dir, trimmed+".test.tsx")))
	}
	return uniqueStrings(candidates)
}

func isTestPath(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	base := filepath.Base(lower)
	switch {
	case strings.HasSuffix(base, "_test.go"):
		return true
	case strings.HasSuffix(base, "_test.py"):
		return true
	case strings.HasSuffix(base, ".test.ts"), strings.HasSuffix(base, ".test.tsx"),
		strings.HasSuffix(base, ".test.js"), strings.HasSuffix(base, ".test.jsx"),
		strings.HasSuffix(base, ".spec.ts"), strings.HasSuffix(base, ".spec.tsx"),
		strings.HasSuffix(base, ".spec.js"), strings.HasSuffix(base, ".spec.jsx"):
		return true
	default:
		return false
	}
}

func uniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = filepath.ToSlash(filepath.Clean(value))
		if value == "." || strings.TrimSpace(value) == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
