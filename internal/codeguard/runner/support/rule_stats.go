package support

import (
	"sort"
	"sync"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// RuleStatsCollector tallies, per rule ID, how many findings were kept and how
// many were silenced by each suppression mechanism. Sections run in parallel
// (and file scanning may be parallelized within a section), so every method is
// safe for concurrent use; a nil collector ignores all calls.
type RuleStatsCollector struct {
	mu      sync.Mutex
	tallies map[string]*ruleTally
}

type ruleTally struct {
	emitted  int
	baseline int
	waiver   int
	inline   int
}

func NewRuleStatsCollector() *RuleStatsCollector {
	return &RuleStatsCollector{tallies: make(map[string]*ruleTally)}
}

// RecordEmitted counts one finding that survived suppression for ruleID.
func (collector *RuleStatsCollector) RecordEmitted(ruleID string) {
	if collector == nil || ruleID == "" {
		return
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	collector.lockedTally(ruleID).emitted++
}

// RecordSuppressed counts one suppressed finding for ruleID, attributed by the
// reason string returned from IsSuppressed.
func (collector *RuleStatsCollector) RecordSuppressed(ruleID string, reason string) {
	if collector == nil || ruleID == "" {
		return
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	switch reason {
	case SuppressionReasonBaseline:
		collector.lockedTally(ruleID).baseline++
	case SuppressionReasonWaiver:
		collector.lockedTally(ruleID).waiver++
	case SuppressionReasonInline:
		collector.lockedTally(ruleID).inline++
	}
}

// lockedTally returns the tally for ruleID, creating it on first use. The
// caller must hold collector.mu.
func (collector *RuleStatsCollector) lockedTally(ruleID string) *ruleTally {
	tally, ok := collector.tallies[ruleID]
	if !ok {
		tally = &ruleTally{}
		collector.tallies[ruleID] = tally
	}
	return tally
}

// Snapshot returns per-rule stats sorted by rule ID. Only rules with recorded
// activity appear (rules never touched are absent by construction).
func (collector *RuleStatsCollector) Snapshot() []core.RuleStatsEntry {
	if collector == nil {
		return nil
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	if len(collector.tallies) == 0 {
		return nil
	}
	entries := make([]core.RuleStatsEntry, 0, len(collector.tallies))
	for ruleID, tally := range collector.tallies {
		entries = append(entries, newRuleStatsEntry(ruleID, *tally))
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].RuleID < entries[j].RuleID })
	return entries
}

func newRuleStatsEntry(ruleID string, tally ruleTally) core.RuleStatsEntry {
	entry := core.RuleStatsEntry{
		RuleID:             ruleID,
		Emitted:            tally.emitted,
		BaselineSuppressed: tally.baseline,
		WaiverSuppressed:   tally.waiver,
		InlineSuppressed:   tally.inline,
	}
	if total := entry.Emitted + entry.Suppressed(); total > 0 {
		entry.SuppressionRatio = float64(entry.Suppressed()) / float64(total)
	}
	return entry
}
