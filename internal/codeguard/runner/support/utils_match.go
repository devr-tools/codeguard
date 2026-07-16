package support

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var patternCache sync.Map // map[string]*regexp.Regexp

func IsInternalOrCmdFile(path string) bool {
	return isInternalFile(path) || IsCmdFile(path)
}

func isInternalFile(path string) bool {
	return strings.HasPrefix(filepath.ToSlash(path), "internal/")
}

func IsCmdFile(path string) bool {
	return strings.HasPrefix(filepath.ToSlash(path), "cmd/")
}

func IsPublicPackageFile(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.HasPrefix(normalized, "pkg/")
}

func IsSDKFacadeFile(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.HasPrefix(normalized, "pkg/codeguard/")
}

func ShouldExclude(rel string, excludes []string) bool {
	defaults := []string{".git/**", ".gocache/**", ".gomodcache/**", ".codeguard/**", "dist/**"}
	for _, pattern := range append(defaults, excludes...) {
		if MatchPattern(pattern, rel) {
			return true
		}
	}
	return false
}

func MatchPattern(pattern string, value string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	value = filepath.ToSlash(strings.TrimSpace(value))
	if pattern == "" {
		return false
	}
	re, ok := compilePattern(pattern)
	if !ok {
		return false
	}
	return re.MatchString(value)
}

func compilePattern(pattern string) (*regexp.Regexp, bool) {
	if cached, ok := patternCache.Load(pattern); ok {
		re, _ := cached.(*regexp.Regexp)
		return re, re != nil
	}
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`.`, `\.`,
		`+`, `\+`,
		`(`, `\(`,
		`)`, `\)`,
		`[`, `\[`,
		`]`, `\]`,
		`{`, `\{`,
		`}`, `\}`,
		`^`, `\^`,
		`$`, `\$`,
	)
	expr := replacer.Replace(pattern)
	expr = strings.ReplaceAll(expr, "**", "§§DOUBLESTAR§§")
	expr = strings.ReplaceAll(expr, "*", `[^/]*`)
	expr = strings.ReplaceAll(expr, "§§DOUBLESTAR§§", `.*`)
	expr = strings.ReplaceAll(expr, "?", `[^/]`)
	re, err := regexp.Compile("^" + expr + "$")
	if err != nil {
		patternCache.Store(pattern, (*regexp.Regexp)(nil))
		return nil, false
	}
	patternCache.Store(pattern, re)
	return re, true
}
