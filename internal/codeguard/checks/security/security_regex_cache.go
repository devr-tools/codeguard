package security

import (
	"regexp"
	"sync"
)

// dynamicPatternCache memoizes regexes compiled from runtime-derived text such
// as import aliases, namespaces, and module names. The same handful of aliases
// and modules recur across many files in a project, so compiling each distinct
// pattern once and reusing it avoids recompiling identical regexes per file.
var dynamicPatternCache sync.Map // map[string]*regexp.Regexp

// compileDynamicPattern returns the compiled form of expr, reusing a previously
// compiled instance when one exists. The expressions passed here are always
// valid (fixed fragments plus regexp.QuoteMeta-escaped input), so it mirrors the
// regexp.MustCompile contract and panics on a genuinely malformed pattern.
func compileDynamicPattern(expr string) *regexp.Regexp {
	if cached, ok := dynamicPatternCache.Load(expr); ok {
		return cached.(*regexp.Regexp)
	}
	compiled := regexp.MustCompile(expr)
	actual, _ := dynamicPatternCache.LoadOrStore(expr, compiled)
	return actual.(*regexp.Regexp)
}
