package codeguard

import (
	"context"
	"io"

	"github.com/devr-tools/codeguard/codeguard/report"
	"github.com/devr-tools/codeguard/codeguard/runner"
)

func NewRunner(cfg Config) *Runner {
	return runner.New(cfg)
}

func WriteReport(w io.Writer, result Report, format string) error {
	return report.Write(w, result, format)
}

func Run(ctx context.Context, cfg Config) (Report, error) {
	return runner.New(cfg).Run(ctx)
}

func RunWithOptions(ctx context.Context, cfg Config, opts ScanOptions) (Report, error) {
	return runner.New(cfg).RunWithOptions(ctx, opts)
}
