package cli

import (
	"fmt"
	"io"
	"sort"
)

type scoreHistoryReportSpec[T any] struct {
	path       string
	history    map[string][]T
	limit      int
	emptyLabel string
	render     func(io.Writer, T, int, bool) int
}

func writeScoreHistoryReport[T any](stdout io.Writer, spec scoreHistoryReportSpec[T]) int {
	if len(spec.history) == 0 {
		_, _ = fmt.Fprintf(stdout, "no %s history recorded at %s\n", spec.emptyLabel, spec.path)
		return exitOK
	}
	keys := make([]string, 0, len(spec.history))
	for key := range spec.history {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entries := spec.history[key]
		if spec.limit > 0 && len(entries) > spec.limit {
			entries = entries[len(entries)-spec.limit:]
		}
		_, _ = fmt.Fprintf(stdout, "%s\n", key)
		previousScore := 0
		hasPrevious := false
		for _, entry := range entries {
			previousScore = renderScoreHistoryEntry(stdout, entry, previousScore, hasPrevious, spec.render)
			hasPrevious = true
		}
	}
	return exitOK
}

func renderScoreHistoryEntry[T any](stdout io.Writer, entry T, previousScore int, hasPrevious bool, render func(io.Writer, T, int, bool) int) int {
	return render(stdout, entry, previousScore, hasPrevious)
}
