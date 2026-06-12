package codeguard

import (
	"context"
	"io"

	internalfix "github.com/devr-tools/codeguard/internal/codeguard/ai/fix"
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

func VerifyFix(ctx context.Context, cfg Config, finding Finding, candidate FixCandidate, opts FixOptions) (VerifiedFix, error) {
	return internalfix.Verify(ctx, cfg, finding, candidate, opts)
}

func GenerateVerifiedFix(ctx context.Context, req FixGenerateRequest) (VerifiedFix, error) {
	return internalfix.GenerateVerified(ctx, req)
}

func WriteBaselineFile(path string, entries []BaselineEntry) error {
	return runner.WriteBaselineFile(path, entries)
}

// SlopHistoryPath derives the slop-score history file path for a config.
func SlopHistoryPath(cfg Config) string {
	return runner.SlopHistoryPath(cfg)
}

// LoadSlopHistory reads the persisted slop-score trend, keyed by artifact ID.
func LoadSlopHistory(path string) map[string][]SlopHistoryEntry {
	return runner.LoadSlopHistory(path)
}

func BaselineEntriesFromReport(rep Report) []BaselineEntry {
	return runner.BaselineEntriesFromReport(rep)
}

func Profiles() []PolicyProfile {
	return config.ProfileList()
}
