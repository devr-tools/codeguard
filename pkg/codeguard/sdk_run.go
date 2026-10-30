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

// ReportWriteOptions adjusts report rendering; see WriteReportWithOptions.
type ReportWriteOptions = report.WriteOptions

// WriteReportWithOptions renders rep in format, applying opts (for example
// omitting the text banner).
func WriteReportWithOptions(w io.Writer, rep Report, format string, opts ReportWriteOptions) error {
	return report.WriteWithOptions(w, rep, format, opts)
}

// Exit policies accepted by Config.Output.FailOn; see ReportGateError.
const (
	FailOnFail = core.FailOnFail
	FailOnWarn = core.FailOnWarn
	FailOnNone = core.FailOnNone
)

// ReportGateError returns a non-nil error when rep should fail a CI gate under
// the failOn exit policy ("fail", "warn", or "none"; empty means "fail").
// Fail-level operational diagnostics such as scan.section.panic and
// scan.target-walk fail the gate under every policy.
func ReportGateError(rep Report, failOn string) error {
	return core.ReportGateError(rep, failOn)
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

func Profiles() []PolicyProfile {
	return config.ProfileList()
}
