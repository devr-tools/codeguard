package reliability

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// lineScope is the block context of one line in a line-based scanner.
type lineScope struct {
	inLoop          bool
	inUnboundedLoop bool
	// inHandler is true inside an exception handler (Python except, C++
	// catch); scanners without handler tracking leave it false.
	inHandler bool
}

// addFindingFunc records one finding for the line-based scanners.
type addFindingFunc func(ruleID string, level string, lineNo int, message string, confidence string, metaKey string, metaValue string)

// retryPatterns are one language's line-level retry signals. language names
// the language in finding messages ("Python", "JavaScript", "C++").
type retryPatterns struct {
	language      string
	retry         func(string) bool
	backoff       func(string) bool
	nonIdempotent func(string) bool
	idempotent    func(string) bool
}

// check reports retry loops without backoff, retry loops without an attempt
// limit, and retried non-idempotent side effects, in that order. Patterns
// only run when their rule is enabled and the line is inside a loop.
func (p retryPatterns) check(rules core.ReliabilityRulesConfig, lineNo int, line string, scope lineScope, add addFindingFunc) {
	if enabled(rules.DetectRetryWithoutBackoff) && scope.inLoop && p.retry(line) && !p.backoff(line) {
		add("reliability.retry-without-backoff", "warn", lineNo, "retry-like "+p.language+" loop has no visible backoff or jitter", "medium", "retry", "no-backoff")
	}
	if enabled(rules.DetectUnboundedRetry) && scope.inUnboundedLoop && p.retry(line) {
		add("reliability.unbounded-retry", "fail", lineNo, "retry-like "+p.language+" loop can run forever without an attempt limit", "medium", "retry", "while-true")
	}
	if enabled(rules.DetectNonIdempotentRetry) && scope.inLoop && p.nonIdempotent(line) && !p.idempotent(line) {
		add("reliability.non-idempotent-retry", "fail", lineNo, "retry-like "+p.language+" loop wraps a non-idempotent side effect without idempotency evidence", "medium", "retry", "side-effect")
	}
}

// popBlockRegions closes the brace-delimited regions (recorded by their
// opening depth) that end before depth.
func popBlockRegions(regions []int, depth int) []int {
	for len(regions) > 0 && depth <= regions[len(regions)-1] {
		regions = regions[:len(regions)-1]
	}
	return regions
}
