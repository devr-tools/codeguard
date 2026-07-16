package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// writeLegibilityHistoryReport mirrors writeSlopHistoryReport for the
// repo_legibility trend. Legibility components are score/max slices rather
// than weighted finding counts, so the breakdown renders as label=score/max.
func writeLegibilityHistoryReport(stdout io.Writer, cfg service.Config, limit int) int {
	path := service.LegibilityHistoryPath(cfg)
	history := service.LoadLegibilityHistory(path)
	if len(history) == 0 {
		_, _ = fmt.Fprintf(stdout, "no legibility-score history recorded at %s\n", path)
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
			_, _ = fmt.Fprintf(stdout, "  %s  score %3d%s  %s\n",
				entry.Timestamp, entry.Score, formatSlopDelta(entry.Score, previousScore, hasPrevious),
				formatLegibilityHistoryComponents(entry))
			previousScore = entry.Score
			hasPrevious = true
		}
	}
	return 0
}

func formatLegibilityHistoryComponents(entry service.LegibilityHistoryEntry) string {
	parts := make([]string, 0, len(entry.Components))
	for _, component := range entry.Components {
		parts = append(parts, fmt.Sprintf("%s=%d/%d", component.Label, component.Score, component.Max))
	}
	return strings.Join(parts, " ")
}
