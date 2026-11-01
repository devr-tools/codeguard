package support

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var pathPatternCache sync.Map // map[string]*regexp.Regexp

// MatchPattern reports whether value matches the slash-separated glob
// pattern, where `*` and `?` stay within one path segment and `**` spans
// segments. Both sides are trimmed and normalized to forward slashes; an empty
// or invalid pattern matches nothing. Compiled patterns are cached.
func MatchPattern(pattern string, value string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	value = filepath.ToSlash(strings.TrimSpace(value))
	if pattern == "" {
		return false
	}
	re, ok := compilePathPattern(pattern)
	if !ok {
		return false
	}
	return re.MatchString(value)
}

func compilePathPattern(pattern string) (*regexp.Regexp, bool) {
	if cached, ok := pathPatternCache.Load(pattern); ok {
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
		pathPatternCache.Store(pattern, (*regexp.Regexp)(nil))
		return nil, false
	}
	pathPatternCache.Store(pattern, re)
	return re, true
}
