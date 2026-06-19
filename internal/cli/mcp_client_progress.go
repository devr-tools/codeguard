package cli

import (
	"context"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

type progressFunc func(progress float64, total float64, message string)

type progressCtxKey struct{}

func withProgress(ctx context.Context, fn progressFunc) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, progressCtxKey{}, fn)
}

func progressFrom(ctx context.Context) progressFunc {
	fn, _ := ctx.Value(progressCtxKey{}).(progressFunc)
	return fn
}

// countEnabledSections returns a best-effort count of the scan sections that
// will run for a config, used as the `total` for per-section progress.
func countEnabledSections(cfg service.Config, mode service.ScanMode) float64 {
	enabled := []bool{
		cfg.Checks.Quality,
		cfg.Checks.Design,
		cfg.Checks.Security,
		cfg.Checks.Prompts,
		cfg.Checks.CI,
		cfg.Checks.SupplyChain,
		cfg.Checks.Contracts != nil && *cfg.Checks.Contracts,
		cfg.Checks.Contracts == nil && mode == service.ScanModeDiff,
		hasRulePackRules(cfg),
	}
	count := 0
	for _, ok := range enabled {
		if ok {
			count++
		}
	}
	if count == 0 {
		count = 1
	}
	return float64(count)
}

func hasRulePackRules(cfg service.Config) bool {
	for _, pack := range cfg.RulePacks {
		if len(pack.Rules) > 0 {
			return true
		}
	}
	return false
}
