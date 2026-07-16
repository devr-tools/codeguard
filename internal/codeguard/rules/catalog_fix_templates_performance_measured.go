package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// performanceMeasuredFixTemplates covers the measurement-based performance
// gates (budgets, benchmark regression).
var performanceMeasuredFixTemplates = map[string]core.FixTemplate{
	"performance.budget":               {Kind: guided, Text: "Bring the artifact back under its byte budget, or raise the budget deliberately.\n\nTypical levers:\n- Bundles: split by route, lazy-load rarely used code, replace heavy dependencies, enable minification/tree-shaking.\n- Binaries: build with -ldflags=\"-s -w\", drop unused embeds.\n- Assets: compress (gzip/brotli/webp) or downscale.\n\nIf the growth is intentional, raise max_bytes for the entry in performance_rules.budgets and note why in the change."},
	"performance.benchmark-regression": {Kind: guided, Text: "Investigate the regressed benchmark before accepting the slowdown.\n\n1. Reproduce: go test -run='^$' -bench=<Name> -benchmem ./pkg\n2. Profile: add -cpuprofile=cpu.out -memprofile=mem.out and inspect with go tool pprof.\n3. Fix the hot path (avoid allocations in loops, hoist repeated work, use faster data structures).\n\nIf the new cost is intentional, delete the baseline file (performance_rules.benchmarks.baseline_path) so the next run records the new numbers as the baseline."},
	"performance.build-regression":     {Kind: guided, Text: "Investigate the regressed build command before accepting the slowdown.\n\n1. Reproduce the configured command locally and capture a profile or trace from the toolchain when available.\n2. Check what invalidated caches or broadened the work (dependency graph changes, whole-repo rebuilds, disabled incremental mode).\n3. Reduce the work or restore caching.\n\nIf the new cost is intentional, delete the baseline file (performance_rules.build_regression.baseline_path) so the next run records the new time as the baseline."},
}
