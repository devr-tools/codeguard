package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func runScanHistory(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("scan-history", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config file or directory path (for secret allowlist/custom patterns)")
	repoPath := fs.String("path", ".", "repository path to scan")
	maxCommits := fs.Int("max-commits", 0, "limit the number of commits scanned (0 = all)")
	allRefs := fs.Bool("all", false, "scan all refs rather than just HEAD history")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Config is optional: it only supplies secret allowlist/custom-pattern/entropy
	// settings. Fall back to defaults when it cannot be loaded.
	cfg, err := loadConfigWithProfile(*configPath, "")
	if err != nil {
		cfg = service.ExampleConfig()
	}

	report, err := service.ScanGitHistory(context.Background(), cfg, service.HistoryScanOptions{
		RepoPath:   *repoPath,
		MaxCommits: *maxCommits,
		AllRefs:    *allRefs,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "scan-history failed: %v\n", err)
		return 1
	}

	if err := writeHistoryReport(stdout, report, *format); err != nil {
		_, _ = fmt.Fprintf(stderr, "scan-history output: %v\n", err)
		return 1
	}

	for _, finding := range report.Findings {
		if finding.Level == "fail" {
			return 1
		}
	}
	return 0
}

func writeHistoryReport(stdout io.Writer, report service.HistoryReport, format string) error {
	switch strings.TrimSpace(strings.ToLower(format)) {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	case "", "text":
		if len(report.Findings) == 0 {
			_, _ = fmt.Fprintf(stdout, "No secrets found in %d commit(s) of history.\n", report.CommitsScanned)
			return nil
		}
		_, _ = fmt.Fprintf(stdout, "%d secret(s) found in %d commit(s) of history:\n\n", len(report.Findings), report.CommitsScanned)
		for _, finding := range report.Findings {
			_, _ = fmt.Fprintf(stdout, "  %s  %s:%d  [%s] %s\n    %s\n", shortCommit(finding.Commit), finding.Path, finding.Line, strings.ToUpper(finding.Level), finding.RuleID, finding.Message)
		}
		_, _ = fmt.Fprintln(stdout, "\nRotate any real credentials: removing them from HEAD does not unleak history.")
		return nil
	default:
		return fmt.Errorf("output format must be text or json")
	}
}

func shortCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}
