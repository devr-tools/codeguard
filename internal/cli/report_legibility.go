package cli

import (
	"fmt"
	"io"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// writeLegibilityHistoryReport mirrors writeSlopHistoryReport for the
// repo_legibility trend. Legibility components are score/max slices rather
// than weighted finding counts, so the breakdown renders as label=score/max.
func writeLegibilityHistoryReport(stdout io.Writer, cfg service.Config, limit int) int {
	path := service.LegibilityHistoryPath(cfg)
	history := service.LoadLegibilityHistory(path)
	return writeScoreHistoryReport(stdout, scoreHistoryReportSpec[service.LegibilityHistoryEntry]{
		path:       path,
		history:    history,
		limit:      limit,
		emptyLabel: "legibility-score",
		render: func(stdout io.Writer, entry service.LegibilityHistoryEntry, previousScore int, hasPrevious bool) int {
			_, _ = fmt.Fprintf(stdout, "  %s  score %3d%s  %s\n",
				entry.Timestamp, entry.Score, formatSlopDelta(entry.Score, previousScore, hasPrevious),
				formatLegibilityHistoryComponents(entry))
			return entry.Score
		},
	})
}

func formatLegibilityHistoryComponents(entry service.LegibilityHistoryEntry) string {
	parts := make([]string, 0, len(entry.Components))
	for _, component := range entry.Components {
		parts = append(parts, fmt.Sprintf("%s=%d/%d", component.Label, component.Score, component.Max))
	}
	return strings.Join(parts, " ")
}
