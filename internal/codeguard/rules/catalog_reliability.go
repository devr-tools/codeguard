package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var reliabilityCatalog = map[string]core.RuleMetadata{
	"reliability.missing-timeout":           reliabilityRule("reliability.missing-timeout", "fail", "Missing timeout", "Fails when outbound production calls are made without a bounded timeout or deadline.", "Pass a context with deadline/timeout, configure the client timeout, or route the call through a bounded project wrapper."),
	"reliability.unbounded-retry":           reliabilityRule("reliability.unbounded-retry", "fail", "Unbounded retry", "Fails when retry logic can continue without a clear attempt limit or cancellation boundary.", "Add a maximum attempt count and stop retrying when the caller context is cancelled."),
	"reliability.retry-without-backoff":     reliabilityRule("reliability.retry-without-backoff", "warn", "Retry without backoff", "Warns when retry logic repeats immediately without backoff or jitter.", "Use exponential backoff with jitter and cap the maximum delay."),
	"reliability.non-idempotent-retry":      reliabilityRule("reliability.non-idempotent-retry", "fail", "Non-idempotent retry", "Fails when retry logic wraps a non-idempotent operation without idempotency evidence.", "Add an idempotency key, deduplication guard, or avoid retrying the side effect."),
	"reliability.missing-cancellation":      reliabilityRule("reliability.missing-cancellation", "warn", "Missing cancellation propagation", "Warns when request or job code drops caller cancellation by using a background context for downstream work.", "Propagate the caller context into downstream calls and goroutines."),
	"reliability.unbounded-work":            reliabilityRule("reliability.unbounded-work", "warn", "Unbounded work", "Warns when goroutines, workers, queues, or buffers can grow without an explicit bound.", "Add a worker pool, semaphore, bounded queue, or backpressure strategy."),
	"reliability.missing-concurrency-limit": reliabilityRule("reliability.missing-concurrency-limit", "warn", "Missing concurrency limit", "Warns when concurrent work is launched without an obvious limit.", "Limit concurrency with a semaphore, worker pool, errgroup limit, or bounded queue."),
	"reliability.resource-leak":             reliabilityRule("reliability.resource-leak", "fail", "Resource leak", "Fails when opened resources such as response bodies, rows, files, tickers, or timers are not closed or stopped.", "Close or stop the resource on every path, and handle cleanup errors where they matter."),
	"reliability.partial-failure-hidden":    reliabilityRule("reliability.partial-failure-hidden", "fail", "Partial failure hidden", "Fails when batch or multi-step work can fail partially but still report success.", "Return structured partial-failure information or fail the operation explicitly."),
	"reliability.missing-graceful-shutdown": reliabilityRule("reliability.missing-graceful-shutdown", "warn", "Missing graceful shutdown", "Warns when servers or long-running workers start without shutdown/drain handling.", "Handle termination signals, stop accepting new work, drain in-flight work, and close resources with a timeout."),
	"reliability.swallowed-error":           reliabilityRule("reliability.swallowed-error", "fail", "Swallowed error", "Fails when an error is discarded or ignored in production code.", "Return, wrap, or explicitly handle the error; only ignore errors with a documented safe reason."),
	"reliability.lost-error-context":        reliabilityRule("reliability.lost-error-context", "warn", "Lost error context", "Warns when an error is replaced without operation context or wrapping.", "Wrap errors with operation context so callers can diagnose the failing dependency or step."),
	"reliability.recoverable-panic":         reliabilityRule("reliability.recoverable-panic", "fail", "Recoverable failure handled with panic", "Fails when a recoverable runtime condition is handled with panic in production code.", "Return an error or use an explicit failure result for recoverable conditions."),
}

func reliabilityRule(id string, level string, title string, description string, howToFix string) core.RuleMetadata {
	return core.RuleMetadata{
		ID:             id,
		Section:        "Reliability",
		DefaultLevel:   level,
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(
			core.RuleLanguageGo,
			core.RuleLanguageTypeScript,
			core.RuleLanguageJavaScript,
			core.RuleLanguagePython,
		),
		Title:       title,
		Description: description,
		HowToFix:    howToFix,
	}
}
