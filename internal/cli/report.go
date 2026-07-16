package cli

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func runReport(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config file or directory path")
	profile := fs.String("profile", "", "optional policy profile override")
	slopHistory := fs.Bool("slop-history", false, "print the persisted slop-score trend per target")
	limit := fs.Int("limit", 0, "maximum history entries to print per target (0 = all)")
	if err := fs.Parse(args); err != nil {
		return exitError
	}
	if !*slopHistory {
		_, _ = fmt.Fprintln(stderr, "report requires a mode flag: -slop-history")
		return exitError
	}

	cfg, ok := loadConfigOrFail(*configPath, *profile, stderr)
	if !ok {
		return exitError
	}
	return writeSlopHistoryReport(stdout, cfg, *limit)
}

func writeSlopHistoryReport(stdout io.Writer, cfg service.Config, limit int) int {
	path := service.SlopHistoryPath(cfg)
	history := service.LoadSlopHistory(path)
	if len(history) == 0 {
		_, _ = fmt.Fprintf(stdout, "no slop-score history recorded at %s\n", path)
		return exitOK
	}
	keys := make([]string, 0, len(history))
	for key := range history {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entries := history[key]
		if limit > 0 && len(entries) > limit {
			entries = entries[len(entries)-limit:]
		}
		_, _ = fmt.Fprintf(stdout, "%s\n", key)
		previousScore := 0
		hasPrevious := false
		for _, entry := range entries {
			_, _ = fmt.Fprintf(stdout, "  %s  score %3d%s  signals %d  %s\n",
				entry.Timestamp, entry.Score, formatSlopDelta(entry.Score, previousScore, hasPrevious),
				entry.Signals, formatSlopComponents(entry))
			previousScore = entry.Score
			hasPrevious = true
		}
	}
	return 0
}

func formatSlopDelta(score int, previous int, hasPrevious bool) string {
	if !hasPrevious {
		return ""
	}
	return fmt.Sprintf(" (%+d)", score-previous)
}

func formatSlopComponents(entry service.SlopHistoryEntry) string {
	parts := make([]string, 0, len(entry.Components))
	for _, component := range entry.Components {
		parts = append(parts, fmt.Sprintf("%s=%d", component.RuleID, component.Count))
	}
	return strings.Join(parts, " ")
}
