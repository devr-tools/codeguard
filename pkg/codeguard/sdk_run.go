package codeguard

import (
	"context"
	"io"

	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/report"
	"github.com/devr-tools/codeguard/internal/codeguard/runner"
)

func NewRunner(cfg Config) *Runner {
	return runner.New(cfg)
}

func WriteReport(w io.Writer, rep Report, format string) error {
	return report.Write(w, rep, format)
}

func Run(ctx context.Context, cfg Config) (Report, error) {
	return runner.Run(ctx, cfg)
}

func RunWithOptions(ctx context.Context, cfg Config, opts ScanOptions) (Report, error) {
	return runner.RunWithOptions(ctx, cfg, opts)
}

func RunPatch(ctx context.Context, cfg Config, diffText string) (Report, error) {
	return runner.RunWithOptions(ctx, cfg, core.ScanOptions{
		Mode:     core.ScanModeDiff,
		BaseRef:  "stdin",
		DiffText: diffText,
	})
}

func WriteBaselineFile(path string, entries []BaselineEntry) error {
	return runner.WriteBaselineFile(path, entries)
}

func BaselineEntriesFromReport(rep Report) []BaselineEntry {
	return runner.BaselineEntriesFromReport(rep)
}

func Profiles() []PolicyProfile {
	return config.ProfileList()
}
