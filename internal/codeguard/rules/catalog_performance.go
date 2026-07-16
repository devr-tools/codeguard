package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// performanceCatalog covers the performance section. These rules previously
// lived in the quality section under quality.* ids (see
// retiredPerformanceRuleIDs for the mapping).
var performanceCatalog = map[string]core.RuleMetadata{
	"performance.n-plus-one-query": {
		ID:             "performance.n-plus-one-query",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(
			core.RuleLanguageGo,
			core.RuleLanguagePython,
			core.RuleLanguageTypeScript,
			core.RuleLanguageJavaScript,
		),
		Title:       "N+1 query in loop",
		Description: "Warns when a database query or remote fetch call runs inside a loop body, suggesting an N+1 access pattern.",
		HowToFix:    "Batch the lookups into one query, prefetch the data before the loop, or use a bulk API.",
	},
	"performance.go.alloc-in-loop": {
		ID:             "performance.go.alloc-in-loop",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelGoNative,
		Title:          "Allocation-heavy loop",
		Description:    "Warns when a loop grows a string by concatenation or accumulates fmt.Sprintf output (performance_rules.detect_alloc_in_loop, on by default). When performance_rules.detect_prealloc_in_loop is enabled (off by default), also warns when a loop appends to a slice without preallocated capacity despite a knowable bound.",
		HowToFix:       "Use strings.Builder for string accumulation and preallocate slice capacity with make(len 0, cap n) before the loop.",
	},
	"performance.sync-io-in-request-path": {
		ID:             "performance.sync-io-in-request-path",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelGoNative,
		Title:          "Synchronous I/O in request path",
		Description:    "Warns when a likely Go HTTP handler performs synchronous file I/O directly in the request path.",
		HowToFix:       "Move the I/O out of the request path, preload or cache the data, or switch to an architecture that avoids per-request filesystem access.",
	},
	"performance.unbounded-goroutines-in-loop": {
		ID:             "performance.unbounded-goroutines-in-loop",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelGoNative,
		Title:          "Goroutines launched from loops",
		Description:    "Warns when Go code launches goroutines from inside loops without any visible bounding mechanism.",
		HowToFix:       "Use a worker pool, semaphore, errgroup limit, or explicit queue so loop-driven concurrency stays bounded.",
	},
	"performance.typescript.sync-io-in-handler": {
		ID:             "performance.typescript.sync-io-in-handler",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "TypeScript sync I/O in handler",
		Description:    "Warns when a synchronous *Sync call runs inside an HTTP request handler, blocking the event loop.",
		HowToFix:       "Switch to the promise-based API (fs.promises, async exec) inside request handlers.",
	},
	"performance.javascript.sync-io-in-handler": {
		ID:             "performance.javascript.sync-io-in-handler",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "JavaScript sync I/O in handler",
		Description:    "Warns when a synchronous *Sync call runs inside an HTTP request handler, blocking the event loop.",
		HowToFix:       "Switch to the promise-based API (fs.promises, async exec) inside request handlers.",
	},
	"performance.typescript.unbounded-concurrency": {
		ID:             "performance.typescript.unbounded-concurrency",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "TypeScript unbounded concurrency",
		Description:    "Warns when promises are created inside a loop without batching or a concurrency limiter.",
		HowToFix:       "Process the work in chunks with Promise.all or wrap calls with a limiter such as p-limit.",
	},
	"performance.javascript.unbounded-concurrency": {
		ID:             "performance.javascript.unbounded-concurrency",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "JavaScript unbounded concurrency",
		Description:    "Warns when promises are created inside a loop without batching or a concurrency limiter.",
		HowToFix:       "Process the work in chunks with Promise.all or wrap calls with a limiter such as p-limit.",
	},
	"performance.python.sync-io-in-async": {
		ID:             "performance.python.sync-io-in-async",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "Python blocking call in async function",
		Description:    "Warns when requests, urllib, or time.sleep calls run inside an async def body, blocking the event loop.",
		HowToFix:       "Use an async HTTP client (httpx.AsyncClient, aiohttp) and asyncio.sleep inside async functions.",
	},
}

// RetiredPerformanceRuleIDs maps the retired quality.* ids of the performance
// rules to their performance.* replacements. The scan never consults this —
// there is deliberately no runtime aliasing — but `codeguard doctor` uses it
// to flag stale waivers and configs after the migration.
func RetiredPerformanceRuleIDs() map[string]string {
	return map[string]string{
		"quality.n-plus-one-query":                 "performance.n-plus-one-query",
		"quality.go.alloc-in-loop":                 "performance.go.alloc-in-loop",
		"quality.sync-io-in-request-path":          "performance.sync-io-in-request-path",
		"quality.unbounded-goroutines-in-loop":     "performance.unbounded-goroutines-in-loop",
		"quality.typescript.sync-io-in-handler":    "performance.typescript.sync-io-in-handler",
		"quality.javascript.sync-io-in-handler":    "performance.javascript.sync-io-in-handler",
		"quality.typescript.unbounded-concurrency": "performance.typescript.unbounded-concurrency",
		"quality.javascript.unbounded-concurrency": "performance.javascript.unbounded-concurrency",
		"quality.python.sync-io-in-async":          "performance.python.sync-io-in-async",
	}
}
