package semantic

// PerformanceRuleID is the rule id for the optional LLM-assisted performance
// lens. Its verdicts are emitted into the performance section (not quality);
// see checks/performance.semanticPerformanceFindings.
const PerformanceRuleID = "performance.ai.semantic-perf"

// performancePromptTemplate mirrors the contract-drift template shape: a
// concrete focus, framework-aware considerations, explicit non-goals, and a
// conservative emission threshold.
func performancePromptTemplate(frameworks []FrameworkRef) RulePromptTemplate {
	return RulePromptTemplate{
		RuleID:   PerformanceRuleID,
		Focus:    "Find performance problems introduced by the changed functions that a static pattern rule cannot judge: repeated expensive calls whose results should be cached or memoized, algorithmic complexity that is clearly out of line with the input sizes the local evidence makes plausible, and obviously redundant work performed across the change.",
		Consider: performanceConsiderations(frameworks),
		Avoid: []string{
			"Do not flag micro-optimizations (allocation tuning, instruction-level or constant-factor concerns) whose real-world impact is negligible or unproven.",
			"Do not flag style or idiom preferences as performance problems.",
			"Do not flag anything without clear evidence in the diff, source snapshots, or tests that the expensive work actually repeats or scales badly.",
			"Do not speculate about workloads, input sizes, or hot paths that the provided context does not support.",
		},
		Threshold: "Emit only when the local evidence shows concrete repeated or super-linear work whose cost would plausibly matter at realistic input sizes, and a caching, batching, hoisting, or algorithm change would clearly remove it.",
	}
}

func performanceConsiderations(frameworks []FrameworkRef) []string {
	items := []string{
		"Check whether the same expensive call (I/O, network, query, parse, compile, crypto, large copy) repeats with identical inputs inside a loop, recursion, or repeatedly-invoked path when its result could be computed once, cached, or memoized.",
		"Estimate the algorithmic complexity of changed loops and recursion relative to the input sizes that surrounding code, tests, and names make plausible, and flag clearly super-linear work on inputs that scale, such as nested scans over the same collection, membership tests against a slice inside a loop, or repeated sorting.",
		"Look for obviously redundant work across the change: recomputing a value that is already available, re-reading or re-parsing the same data, or per-item work that a batch API already visible in the snapshots covers.",
	}
	for _, framework := range frameworks {
		if hasHint(framework, "component-props-contract") || hasHint(framework, "stateful-component") {
			items = append(items, "For React or Next.js components, check whether changed render paths repeat expensive computation on every render that memoization or module-level caching should hoist.")
		}
		if hasHint(framework, "route-handler-contract") || hasHint(framework, "middleware-order-sensitive") || hasHint(framework, "middleware-next-chain") {
			items = append(items, "For route handlers and middleware, check whether per-request work such as config loads, client construction, or pattern compilation could move to initialization time.")
		}
	}
	return uniqueNonEmptyStrings(items)
}
