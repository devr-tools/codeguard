package support

import (
	"path/filepath"
	"strings"
)

func PathMatchesPattern(pattern string, rel string) bool {
	pattern = strings.ToLower(filepath.ToSlash(strings.TrimSpace(pattern)))
	rel = strings.ToLower(filepath.ToSlash(rel))
	if pattern == "" {
		return false
	}
	if ok, err := filepath.Match(filepath.FromSlash(pattern), filepath.FromSlash(rel)); err == nil && ok {
		return true
	}
	if strings.HasPrefix(pattern, "**/") && strings.Contains(rel, strings.TrimPrefix(pattern, "**/")) {
		return true
	}
	if strings.HasSuffix(pattern, "/**") && strings.HasPrefix(rel, strings.TrimSuffix(pattern, "/**")) {
		return true
	}
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		pos := 0
		for _, part := range parts {
			if part == "" {
				continue
			}
			next := strings.Index(rel[pos:], part)
			if next < 0 {
				return false
			}
			pos += next + len(part)
		}
		return true
	}
	return rel == pattern
}
