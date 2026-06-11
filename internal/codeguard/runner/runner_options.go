package runner

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func normalizeScanOptions(opts core.ScanOptions) core.ScanOptions {
	if opts.Mode == "" {
		opts.Mode = core.ScanModeFull
	}
	if opts.BaseRef == "" {
		opts.BaseRef = "main"
	}
	return opts
}

func buildSections(ctx context.Context, sc scanContext) []core.SectionResult {
	sections := make([]core.SectionResult, 0, 6)
	if sc.cfg.Checks.Quality {
		sections = append(sections, sc.runQuality(ctx))
	}
	if sc.cfg.Checks.Design {
		sections = append(sections, sc.runDesign(ctx))
	}
	if sc.cfg.Checks.Security {
		sections = append(sections, sc.runSecurity(ctx))
	}
	if sc.cfg.Checks.Prompts {
		sections = append(sections, sc.runPrompts(ctx))
	}
	if sc.cfg.Checks.CI {
		sections = append(sections, sc.runCI(ctx))
	}
	if len(sc.customRules) > 0 {
		sections = append(sections, sc.runCustomRules(ctx))
	}
	return sections
}
