package corpus_test

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
)

type statKind int

const (
	statTP statKind = iota
	statFN
	statFP
	statKnownFN
	statKnownFP
)

type ruleStats struct {
	tp, fn, fp, knownFN, knownFP int
}

// scoreboard accumulates per-rule tallies. Known gaps count as FN/FP in the
// metrics — recall and precision reflect documented detector deficiencies —
// while unexpected deviations additionally fail the test.
type scoreboard struct {
	rules []string
	stats map[string]*ruleStats
}

func newScoreboard(rules []string) *scoreboard {
	board := &scoreboard{rules: append([]string(nil), rules...), stats: make(map[string]*ruleStats, len(rules))}
	sort.Strings(board.rules)
	for _, ruleID := range board.rules {
		board.stats[ruleID] = &ruleStats{}
	}
	return board
}

func (b *scoreboard) add(ruleID string, kind statKind) {
	stats, ok := b.stats[ruleID]
	if !ok {
		stats = &ruleStats{}
		b.stats[ruleID] = stats
		b.rules = append(b.rules, ruleID)
		sort.Strings(b.rules)
	}
	switch kind {
	case statTP:
		stats.tp++
	case statFN:
		stats.fn++
	case statFP:
		stats.fp++
	case statKnownFN:
		stats.knownFN++
	case statKnownFP:
		stats.knownFP++
	}
}

func (b *scoreboard) render() string {
	var builder strings.Builder
	writer := tabwriter.NewWriter(&builder, 2, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "RULE\tTP\tFN\tFP\tPRECISION\tRECALL")
	var total ruleStats
	for _, ruleID := range b.rules {
		stats := b.stats[ruleID]
		fn := stats.fn + stats.knownFN
		fp := stats.fp + stats.knownFP
		total.tp += stats.tp
		total.fn += fn
		total.fp += fp
		fmt.Fprintf(writer, "%s\t%d\t%d\t%d\t%s\t%s\n", ruleID, stats.tp, fn, fp, ratio(stats.tp, fp), ratio(stats.tp, fn))
	}
	fmt.Fprintf(writer, "TOTAL\t%d\t%d\t%d\t%s\t%s\n", total.tp, total.fn, total.fp, ratio(total.tp, total.fp), ratio(total.tp, total.fn))
	_ = writer.Flush()
	return builder.String()
}

// ratio renders tp/(tp+other) as a fixed-point metric, or n/a when undefined.
func ratio(tp int, other int) string {
	if tp+other == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.3f", float64(tp)/float64(tp+other))
}
