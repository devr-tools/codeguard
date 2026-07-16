package codeguard

import (
	"context"
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/security"
	"github.com/devr-tools/codeguard/internal/codeguard/history"
)

// HistoryScanOptions configures a git-history secret scan.
type HistoryScanOptions struct {
	RepoPath   string
	MaxCommits int
	AllRefs    bool
}

// HistoryFinding is a secret detected in a past commit.
type HistoryFinding = history.Finding

// HistoryReport is the result of ScanGitHistory.
type HistoryReport = history.Report

// ScanGitHistory walks the repository's git history for hardcoded secrets and
// credentials using the supplied config's secret settings (allowlist, custom
// patterns, entropy). Findings that only exist in history still represent leaked
// credentials that must be rotated.
func ScanGitHistory(ctx context.Context, cfg Config, opts HistoryScanOptions) (HistoryReport, error) {
	scanner, issues := security.BuildScanner(cfg.Checks.SecurityRules.Secrets)
	if len(issues) > 0 {
		return HistoryReport{}, fmt.Errorf("invalid secret scan config: %s", strings.Join(issues, "; "))
	}
	return history.Scan(ctx, history.Options{
		RepoPath:   opts.RepoPath,
		MaxCommits: opts.MaxCommits,
		AllRefs:    opts.AllRefs,
		Scanner:    scanner,
	})
}
