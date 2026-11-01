package support

import (
	"fmt"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/version"
)

const DefaultWaiverAuditHistoryLimit = 100

var waiverAuditHistory = listHistory[core.WaiverAuditHistoryEntry]{version: 1, defaultLimit: DefaultWaiverAuditHistoryLimit}

func WaiverAuditHistoryPathForBase(base string) string {
	return derivedCachePath(base, ".waiver-audit-history")
}

func LoadWaiverAuditHistory(path string) []core.WaiverAuditHistoryEntry {
	return waiverAuditHistory.Load(path)
}

// AppendWaiverAuditHistory records entry, trims the history to limit entries,
// and persists the file, returning any write error.
func AppendWaiverAuditHistory(path string, entry core.WaiverAuditHistoryEntry, limit int) error {
	return waiverAuditHistory.Append(path, entry, limit)
}

// RecordWaiverAuditHistory annotates waivers with upgrade drift against the
// previous persisted audit and records the current audit. A failed write is
// returned as a warn-level operational diagnostic rather than dropped.
func RecordWaiverAuditHistory(sc Context, waivers []core.WaiverAuditEntry) ([]core.WaiverAuditEntry, []core.Diagnostic) {
	if len(waivers) == 0 || strings.TrimSpace(sc.Opts.DiffText) != "" {
		return waivers, nil
	}
	if sc.Cfg.Cache.Enabled != nil && !*sc.Cfg.Cache.Enabled {
		return waivers, nil
	}
	path := WaiverAuditHistoryPathForBase(sc.Cfg.Cache.Path)
	if path == "" {
		return waivers, nil
	}

	history := LoadWaiverAuditHistory(path)
	current := core.WaiverAuditHistoryEntry{
		Timestamp:         sc.Today.UTC().Format(time.RFC3339),
		CodeGuardVersion:  version.Number,
		ConfigFingerprint: sc.ConfigHash,
		ScanMode:          string(sc.Opts.Mode),
		BaseRef:           sc.Opts.BaseRef,
		TargetPath:        sc.Opts.TargetPath,
		Waivers:           cloneWaiverAuditEntries(waivers),
	}
	previous, hasPrevious := latestWaiverAuditHistory(history)
	annotated := annotateWaiverAuditWithHistory(current, previous, hasPrevious)
	current.Waivers = cloneWaiverAuditEntries(annotated)
	if err := AppendWaiverAuditHistory(path, current, DefaultWaiverAuditHistoryLimit); err != nil {
		return annotated, []core.Diagnostic{HistoryWriteDiagnostic("waiver-audit", path, err)}
	}
	return annotated, nil
}

func annotateWaiverAuditWithHistory(current core.WaiverAuditHistoryEntry, previous core.WaiverAuditHistoryEntry, hasPrevious bool) []core.WaiverAuditEntry {
	entries := cloneWaiverAuditEntries(current.Waivers)
	if !hasPrevious || strings.TrimSpace(previous.CodeGuardVersion) == "" || previous.CodeGuardVersion == current.CodeGuardVersion {
		return entries
	}
	comparable := waiverAuditHistoryComparable(current, previous)
	previousByKey := map[string]core.WaiverAuditEntry{}
	for _, entry := range previous.Waivers {
		previousByKey[waiverAuditEntryKey(entry)] = entry
	}
	for idx := range entries {
		entry := &entries[idx]
		if entry.Status != core.WaiverAuditStatusUnused {
			continue
		}
		prev, ok := previousByKey[waiverAuditEntryKey(*entry)]
		if !ok || prev.MatchedFindings == 0 {
			continue
		}
		entry.PreviousVersion = previous.CodeGuardVersion
		entry.PreviousMatches = prev.MatchedFindings
		if comparable {
			entry.UpgradeStatus = core.WaiverAuditUpgradeStatusStaleAfterUpgrade
			entry.UpgradeReason = fmt.Sprintf("matched %d finding(s) on CodeGuard %s and 0 on %s with comparable config and scan scope", prev.MatchedFindings, previous.CodeGuardVersion, current.CodeGuardVersion)
			continue
		}
		entry.UpgradeStatus = core.WaiverAuditUpgradeStatusInconclusive
		entry.UpgradeReason = "previous audit used a different config fingerprint or scan scope"
	}
	return entries
}

func latestWaiverAuditHistory(history []core.WaiverAuditHistoryEntry) (core.WaiverAuditHistoryEntry, bool) {
	if len(history) == 0 {
		return core.WaiverAuditHistoryEntry{}, false
	}
	return history[len(history)-1], true
}

func waiverAuditHistoryComparable(current core.WaiverAuditHistoryEntry, previous core.WaiverAuditHistoryEntry) bool {
	return current.ConfigFingerprint == previous.ConfigFingerprint &&
		current.ScanMode == previous.ScanMode &&
		current.BaseRef == previous.BaseRef &&
		current.TargetPath == previous.TargetPath
}

func waiverAuditEntryKey(entry core.WaiverAuditEntry) string {
	return strings.Join([]string{
		fmt.Sprintf("%d", entry.Index),
		entry.Rule,
		entry.Path,
		entry.ExpiresOn,
	}, "\x00")
}

func cloneWaiverAuditEntries(entries []core.WaiverAuditEntry) []core.WaiverAuditEntry {
	out := make([]core.WaiverAuditEntry, len(entries))
	for idx, entry := range entries {
		out[idx] = entry
		out[idx].MatchedFingerprints = append([]string(nil), entry.MatchedFingerprints...)
	}
	return out
}
