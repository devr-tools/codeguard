package support

import (
	"fmt"
	"path/filepath"
	"strings"
)

func rebaseUnifiedDiffBlock(block string, prefix string) (string, bool) {
	lines := strings.SplitAfter(block, "\n")
	keep := prefix == ""
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		rewritten, nextKeep, include, ok := rewriteUnifiedDiffLine(line, prefix, keep)
		if !ok {
			return "", false
		}
		keep = nextKeep
		if include {
			out = append(out, rewritten)
		}
	}

	if !keep || len(out) == 0 {
		return "", false
	}
	return strings.Join(out, ""), true
}

func rewriteUnifiedDiffLine(line string, prefix string, keep bool) (string, bool, bool, bool) {
	switch {
	case strings.HasPrefix(line, "diff --git "):
		return rewriteDiffGitLine(line, prefix)
	case strings.HasPrefix(line, "--- "):
		return rewriteDiffMarkerLine(line, prefix, keep, "--- ")
	case strings.HasPrefix(line, "+++ "):
		return rewriteDiffMarkerLine(line, prefix, keep, "+++ ")
	default:
		return line, keep, keep, true
	}
}

func rewriteDiffGitLine(line string, prefix string) (string, bool, bool, bool) {
	oldPath, newPath, ok := parseDiffGitPaths(line)
	if !ok {
		return "", false, false, false
	}
	oldPath, okOld := stripDiffPathPrefix(oldPath, prefix)
	newPath, okNew := stripDiffPathPrefix(newPath, prefix)
	keep := okOld || okNew
	if !keep {
		return "", false, false, true
	}
	return fmt.Sprintf("diff --git a/%s b/%s\n", oldPath, newPath), true, true, true
}

func rewriteDiffMarkerLine(line string, prefix string, keep bool, marker string) (string, bool, bool, bool) {
	path, ok := stripDiffMarkerPrefix(strings.TrimPrefix(strings.TrimRight(line, "\n"), marker), prefix)
	if !ok {
		if keep {
			return "", false, false, false
		}
		return "", false, false, true
	}
	return marker + path + "\n", true, true, true
}

func parseDiffGitPaths(line string) (string, string, bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 4 {
		return "", "", false
	}
	oldPath := strings.TrimPrefix(fields[2], "a/")
	newPath := strings.TrimPrefix(fields[3], "b/")
	return oldPath, newPath, true
}

func stripDiffMarkerPrefix(path string, prefix string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "/dev/null" {
		return path, true
	}
	if strings.HasPrefix(path, "a/") {
		trimmed, ok := stripDiffPathPrefix(strings.TrimPrefix(path, "a/"), prefix)
		if !ok {
			return "", false
		}
		return "a/" + trimmed, true
	}
	if strings.HasPrefix(path, "b/") {
		trimmed, ok := stripDiffPathPrefix(strings.TrimPrefix(path, "b/"), prefix)
		if !ok {
			return "", false
		}
		return "b/" + trimmed, true
	}
	return stripDiffPathPrefix(path, prefix)
}

func stripDiffPathPrefix(path string, prefix string) (string, bool) {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if prefix == "" {
		return path, true
	}
	if path == prefix {
		return filepath.Base(path), true
	}
	prefixWithSlash := prefix + "/"
	if !strings.HasPrefix(path, prefixWithSlash) {
		return "", false
	}
	return strings.TrimPrefix(path, prefixWithSlash), true
}
