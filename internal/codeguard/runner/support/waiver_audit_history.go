package support

import (
	"fmt"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/cachefile"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/version"
)

const waiverAuditHistoryVersion = 1

const DefaultWaiverAuditHistoryLimit = 100

type waiverAuditHistoryFile struct {
	Version int                            `json:"version"`
	Entries []core.WaiverAuditHistoryEntry `json:"entries"`
}

func WaiverAuditHistoryPathForBase(base string) string {
	return derivedCachePath(base, ".waiver-audit-history")
}

func LoadWaiverAuditHistory(path string) []core.WaiverAuditHistoryEntry {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	var file waiverAuditHistoryFile
	if !cachefile.Load(path, &file) || file.Version != waiverAuditHistoryVersion {
		return nil
	}
	return file.Entries
}

func AppendWaiverAuditHistory(path string, entry core.WaiverAuditHistoryEntry, limit int) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if limit <= 0 {
		limit = DefaultWaiverAuditHistoryLimit
	}
	entries := append(LoadWaiverAuditHistory(path), entry)
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	saveWaiverAuditHistory(path, entries)
}

func RecordWaiverAuditHistory(sc Context, waivers []core.WaiverAuditEntry) []core.WaiverAuditEntry {
	if len(waivers) == 0 || strings.TrimSpace(sc.Opts.DiffText) != "" {
		return waivers
	}
	if sc.Cfg.Cache.Enabled != nil && !*sc.Cfg.Cache.Enabled {
		return waivers
	}
	path := WaiverAuditHistoryPathForBase(sc.Cfg.Cache.Path)
	if path == "" {
		return waivers
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
	AppendWaiverAuditHistory(path, current, DefaultWaiverAuditHistoryLimit)
	return annotated
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

func saveWaiverAuditHistory(path string, entries []core.WaiverAuditHistoryEntry) {
	payload := waiverAuditHistoryFile{Version: waiverAuditHistoryVersion, Entries: entries}
	writeHistoryFile(path, payload)
}
