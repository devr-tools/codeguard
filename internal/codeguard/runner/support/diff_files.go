package support

import (
	"path/filepath"
	"slices"
	"strings"
)

func ChangedFilesFromUnifiedDiff(diffText string) []string {
	files := make([]string, 0)
	seen := map[string]struct{}{}
	for _, line := range strings.Split(strings.ReplaceAll(diffText, "\r\n", "\n"), "\n") {
		if !strings.HasPrefix(line, "+++ ") {
			continue
		}
		rel, ok := diffHeaderSidePath(strings.TrimPrefix(line, "+++ "), "b/")
		if !ok || rel == "" {
			continue
		}
		rel = filepath.ToSlash(rel)
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		files = append(files, rel)
	}
	slices.Sort(files)
	return files
}
