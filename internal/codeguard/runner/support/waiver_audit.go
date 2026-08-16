package support

import (
	"sort"
	"sync"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type WaiverAuditCollector struct {
	mu      sync.Mutex
	waivers []core.WaiverConfig
	matches map[int]waiverAuditMatchTally
}

type waiverAuditMatchTally struct {
	count        int
	fingerprints map[string]struct{}
}

func NewWaiverAuditCollector(waivers []core.WaiverConfig) *WaiverAuditCollector {
	copied := append([]core.WaiverConfig(nil), waivers...)
	return &WaiverAuditCollector{
		waivers: copied,
		matches: make(map[int]waiverAuditMatchTally, len(copied)),
	}
}

func (collector *WaiverAuditCollector) RecordMatches(matches []WaiverMatch, finding core.Finding) {
	if collector == nil || len(matches) == 0 {
		return
	}
	fingerprint := firstNonEmpty(finding.ContextFingerprint, finding.Fingerprint)
	collector.mu.Lock()
	defer collector.mu.Unlock()
	for _, match := range matches {
		tally := collector.matches[match.Index]
		tally.count++
		if fingerprint != "" {
			if tally.fingerprints == nil {
				tally.fingerprints = map[string]struct{}{}
			}
			tally.fingerprints[fingerprint] = struct{}{}
		}
		collector.matches[match.Index] = tally
	}
}

func (collector *WaiverAuditCollector) Snapshot(catalog map[string]core.RuleMetadata, today time.Time) []core.WaiverAuditEntry {
	if collector == nil || len(collector.waivers) == 0 {
		return nil
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()

	entries := make([]core.WaiverAuditEntry, 0, len(collector.waivers))
	for idx, waiver := range collector.waivers {
		tally := collector.matches[idx]
		matched := tally.count
		status := waiverAuditStatus(waiver, matched, catalog, today)
		entries = append(entries, core.WaiverAuditEntry{
			Index:               idx,
			Rule:                waiver.Rule,
			Path:                waiver.Path,
			Reason:              waiver.Reason,
			ExpiresOn:           waiver.ExpiresOn,
			Status:              status,
			MatchedFindings:     matched,
			MatchedFingerprints: sortedWaiverAuditFingerprints(tally.fingerprints),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Index < entries[j].Index })
	return entries
}

func waiverAuditStatus(waiver core.WaiverConfig, matched int, catalog map[string]core.RuleMetadata, today time.Time) string {
	if waiver.Rule != "*" {
		if _, ok := catalog[waiver.Rule]; !ok {
			return core.WaiverAuditStatusUnknownRule
		}
	}
	if suppressionExpired(waiver.ExpiresOn, today) {
		return core.WaiverAuditStatusExpired
	}
	if matched > 0 {
		return core.WaiverAuditStatusActive
	}
	return core.WaiverAuditStatusUnused
}

func sortedWaiverAuditFingerprints(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
