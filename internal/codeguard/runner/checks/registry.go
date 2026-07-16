package checks

import (
	"context"

	agentContextCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/agentcontext"
	ciCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/ci"
	contractsCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/contracts"
	designCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/design"
	performanceCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/performance"
	promptsCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/prompts"
	qualityCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/quality"
	securityCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/security"
	supplyChainCheck "github.com/devr-tools/codeguard/internal/codeguard/checks/supplychain"
	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	customrunner "github.com/devr-tools/codeguard/internal/codeguard/runner/custom"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// sectionDef describes one check section in a data-driven form, replacing the
// hand-maintained if-ladder that previously lived in Build. Each entry carries
// its stable id/display name, the predicate that decides whether the section
// runs for a given scan, and the closure that actually executes it.
//
// run receives both the runner-level Context (sc) and the per-check Context
// (checkEnv) so that sections built on either entry point fit the same shape:
// most sections call <pkg>.Run(ctx, checkEnv); the custom-rules section calls
// customrunner.RunSection(ctx, sc).
type sectionDef struct {
	id      string
	name    string
	enabled func(sc runnersupport.Context) bool
	run     func(ctx context.Context, sc runnersupport.Context, checkEnv checkSupport.Context) core.SectionResult
}

// sectionRegistry lists every section in the exact order they appear in the
// scan result. Build iterates this slice and, for each enabled section, calls
// through the safeRun panic-recovery wrapper.
var sectionRegistry = []sectionDef{
	{
		id:      "quality",
		name:    "Quality",
		enabled: func(sc runnersupport.Context) bool { return sc.Cfg.Checks.Quality },
		run: func(ctx context.Context, _ runnersupport.Context, checkEnv checkSupport.Context) core.SectionResult {
			return qualityCheck.Run(ctx, checkEnv)
		},
	},
	{
		id:      "performance",
		name:    "Performance",
		enabled: func(sc runnersupport.Context) bool { return sc.Cfg.Checks.Performance },
		run: func(ctx context.Context, _ runnersupport.Context, checkEnv checkSupport.Context) core.SectionResult {
			return performanceCheck.Run(ctx, checkEnv)
		},
	},
	{
		id:      "design",
		name:    "Design",
		enabled: func(sc runnersupport.Context) bool { return sc.Cfg.Checks.Design },
		run: func(ctx context.Context, _ runnersupport.Context, checkEnv checkSupport.Context) core.SectionResult {
			return designCheck.Run(ctx, checkEnv)
		},
	},
	{
		id:      "security",
		name:    "Security",
		enabled: func(sc runnersupport.Context) bool { return sc.Cfg.Checks.Security },
		run: func(ctx context.Context, _ runnersupport.Context, checkEnv checkSupport.Context) core.SectionResult {
			return securityCheck.Run(ctx, checkEnv)
		},
	},
	{
		id:      "prompts",
		name:    "Prompts",
		enabled: func(sc runnersupport.Context) bool { return sc.Cfg.Checks.Prompts },
		run: func(ctx context.Context, _ runnersupport.Context, checkEnv checkSupport.Context) core.SectionResult {
			return promptsCheck.Run(ctx, checkEnv)
		},
	},
	{
		id:      "ci",
		name:    "CI",
		enabled: func(sc runnersupport.Context) bool { return sc.Cfg.Checks.CI },
		run: func(ctx context.Context, _ runnersupport.Context, checkEnv checkSupport.Context) core.SectionResult {
			return ciCheck.Run(ctx, checkEnv)
		},
	},
	{
		id:      "supply-chain",
		name:    "Supply Chain",
		enabled: func(sc runnersupport.Context) bool { return sc.Cfg.Checks.SupplyChain },
		run: func(ctx context.Context, _ runnersupport.Context, checkEnv checkSupport.Context) core.SectionResult {
			return supplyChainCheck.Run(ctx, checkEnv)
		},
	},
	{
		id:      "context",
		name:    "Agent Context",
		enabled: contextEnabled,
		run: func(ctx context.Context, _ runnersupport.Context, checkEnv checkSupport.Context) core.SectionResult {
			return agentContextCheck.Run(ctx, checkEnv)
		},
	},
	{
		id:      "contracts",
		name:    "Contracts",
		enabled: contractsEnabled,
		run: func(ctx context.Context, _ runnersupport.Context, checkEnv checkSupport.Context) core.SectionResult {
			return contractsCheck.Run(ctx, checkEnv)
		},
	},
	{
		id:      "custom",
		name:    "Custom Rules",
		enabled: func(sc runnersupport.Context) bool { return len(sc.CustomRules) > 0 },
		run: func(ctx context.Context, sc runnersupport.Context, _ checkSupport.Context) core.SectionResult {
			return customrunner.RunSection(ctx, sc)
		},
	},
}
