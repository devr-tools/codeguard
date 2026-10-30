package quality

import (
	"regexp"
	"sync"
	"sync/atomic"
)

// PERF: several precision detectors build a regex out of scan input (an
// identifier name, usually) and then match it once. Compiling per call made
// regexp.MustCompile responsible for ~32% of every byte a full scan allocated —
// one detector alone (duplicatedKnowledgeLineIsEnumOrTypeLiteral, which ran per
// source line) accounted for half of that. Patterns whose text is constant are
// package-level vars; the ones that interpolate scan input go through
// cachedPattern.
//
// The cache is process-global and `codeguard serve --mcp` is long-lived, so it
// is bounded: past maxCachedPatterns distinct expressions new entries are
// compiled but not retained. Distinct expressions track distinct identifier
// names, so real repositories stay well under the cap.
const maxCachedPatterns = 8192

var (
	dynamicPatternCache sync.Map // expression string -> *regexp.Regexp
	dynamicPatternCount atomic.Int64
)

// cachedPattern compiles expr at most once per distinct expression. It panics
// on an invalid expression exactly like the regexp.MustCompile calls it
// replaces, so a malformed interpolation still surfaces through SafeRun.
func cachedPattern(expr string) *regexp.Regexp {
	if cached, ok := dynamicPatternCache.Load(expr); ok {
		return cached.(*regexp.Regexp)
	}
	compiled := regexp.MustCompile(expr)
	if dynamicPatternCount.Load() >= maxCachedPatterns {
		return compiled
	}
	actual, loaded := dynamicPatternCache.LoadOrStore(expr, compiled)
	if !loaded {
		dynamicPatternCount.Add(1)
	}
	return actual.(*regexp.Regexp)
}
