package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func runReport(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config file or directory path")
	profile := fs.String("profile", "", "optional policy profile override")
	slopHistory := fs.Bool("slop-history", false, "print the persisted slop-score trend per target")
	perfHistory := fs.Bool("perf-history", false, "print the persisted performance-score trend per target")
	legibilityHistory := fs.Bool("legibility-history", false, "print the persisted repo-legibility score trend per target")
	limit := fs.Int("limit", 0, "maximum history entries to print per target (0 = all)")
	if err := fs.Parse(args); err != nil {
		return exitError
	}
	modes := 0
	for _, enabled := range []bool{*slopHistory, *perfHistory, *legibilityHistory} {
		if enabled {
			modes++
		}
	}
	if modes == 0 {
		_, _ = fmt.Fprintln(stderr, "report requires a mode flag: -slop-history, -perf-history, or -legibility-history")
		return exitError
	}
	if modes > 1 {
		_, _ = fmt.Fprintln(stderr, "report accepts only one mode flag: -slop-history, -perf-history, or -legibility-history")
		return exitError
	}

	cfg, ok := loadConfigOrFail(*configPath, *profile, stderr)
	if !ok {
		return exitError
	}
	if *perfHistory {
		return writePerfHistoryReport(stdout, cfg, *limit)
	}
	if *legibilityHistory {
		return writeLegibilityHistoryReport(stdout, cfg, *limit)
	}
	return writeSlopHistoryReport(stdout, cfg, *limit)
}

// writePerfHistoryReport mirrors writeSlopHistoryReport for the
// performance-score trend.
func writePerfHistoryReport(stdout io.Writer, cfg service.Config, limit int) int {
	path := service.PerfScoreHistoryPath(cfg)
	history := service.LoadPerfScoreHistory(path)
	return writeScoreHistoryReport(stdout, scoreHistoryReportSpec[service.PerformanceHistoryEntry]{
		path:       path,
		history:    history,
		limit:      limit,
		emptyLabel: "performance-score",
		render: func(stdout io.Writer, entry service.PerformanceHistoryEntry, previousScore int, hasPrevious bool) int {
			_, _ = fmt.Fprintf(stdout, "  %s  score %3d%s  signals %d  %s\n",
				entry.Timestamp, entry.Score, formatSlopDelta(entry.Score, previousScore, hasPrevious),
				entry.Signals, formatScoreComponents(entry.Components))
			return entry.Score
		},
	})
}

func writeSlopHistoryReport(stdout io.Writer, cfg service.Config, limit int) int {
	path := service.SlopHistoryPath(cfg)
	history := service.LoadSlopHistory(path)
	return writeScoreHistoryReport(stdout, scoreHistoryReportSpec[service.SlopHistoryEntry]{
		path:       path,
		history:    history,
		limit:      limit,
		emptyLabel: "slop-score",
		render: func(stdout io.Writer, entry service.SlopHistoryEntry, previousScore int, hasPrevious bool) int {
			_, _ = fmt.Fprintf(stdout, "  %s  score %3d%s  signals %d  %s\n",
				entry.Timestamp, entry.Score, formatSlopDelta(entry.Score, previousScore, hasPrevious),
				entry.Signals, formatSlopComponents(entry))
			return entry.Score
		},
	})
}

func formatSlopDelta(score int, previous int, hasPrevious bool) string {
	if !hasPrevious {
		return ""
	}
	return fmt.Sprintf(" (%+d)", score-previous)
}

func formatSlopComponents(entry service.SlopHistoryEntry) string {
	return formatScoreComponents(entry.Components)
}

func formatScoreComponents(components []service.SlopScoreComponent) string {
	parts := make([]string, 0, len(components))
	for _, component := range components {
		parts = append(parts, fmt.Sprintf("%s=%d", component.RuleID, component.Count))
	}
	return strings.Join(parts, " ")
}
