package cli

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type Options struct {
	SampleLimit int
	Ownership   []OwnershipMapping
}

type OwnershipMapping struct {
	Pattern string `json:"pattern" yaml:"pattern"`
	Owner   string `json:"owner" yaml:"owner"`
}

type Counts struct {
	Before        int `json:"before"`
	Active        int `json:"active"`
	ActiveExact   int `json:"active_exact"`
	ActiveContext int `json:"active_context"`
	ActiveContent int `json:"active_content"`
	Stale         int `json:"stale"`
	Removed       int `json:"removed"`
	Final         int `json:"final"`
	Invalid       int `json:"invalid"`
}

type EntryAudit struct {
	Entry   core.BaselineEntry `json:"entry"`
	Status  string             `json:"status"`
	Matches []FindingRef       `json:"matches,omitempty"`
}

type FindingRef struct {
	RuleID     string `json:"rule_id"`
	Path       string `json:"path,omitempty"`
	Line       int    `json:"line,omitempty"`
	Message    string `json:"message,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	Language   string `json:"language,omitempty"`
}

type Collision struct {
	Kind            string `json:"kind"`
	Fingerprint     string `json:"fingerprint"`
	BaselineEntries int    `json:"baseline_entries"`
	CurrentFindings int    `json:"current_findings"`
}

type Duplicate struct {
	Kind        string `json:"kind"`
	Fingerprint string `json:"fingerprint"`
	Count       int    `json:"count"`
}

type Group struct {
	Name       string       `json:"name"`
	Count      int          `json:"count"`
	Samples    []FindingRef `json:"samples,omitempty"`
	Confidence []NamedCount `json:"confidence_distribution,omitempty"`
	Languages  []NamedCount `json:"language_distribution,omitempty"`
}

type NamedCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type AuditResult struct {
	Counts     Counts       `json:"counts"`
	Entries    []EntryAudit `json:"entries"`
	Collisions []Collision  `json:"collisions,omitempty"`
	Duplicates []Duplicate  `json:"duplicates,omitempty"`
	ByRule     []Group      `json:"by_rule"`
	ByOwner    []Group      `json:"by_owner"`
	ByRisk     []Group      `json:"by_risk"`
}

func Audit(file core.BaselineFile, findings []core.Finding, opts Options) AuditResult {
	result := AuditResult{Counts: Counts{Before: len(file.Entries)}}
	exactCurrent := indexFindings(findings, func(f core.Finding) string { return f.Fingerprint })
	contextCurrent := indexFindings(findings, func(f core.Finding) string { return f.ContextFingerprint })
	contentCurrent := indexFindings(findings, func(f core.Finding) string { return f.ContentFingerprint })
	matches := matchBaselineEntries(file.Entries, findings)
	for idx, entry := range file.Entries {
		audit := EntryAudit{Entry: entry, Status: "stale"}
		if strings.TrimSpace(entry.Fingerprint) == "" {
			audit.Status = "invalid"
			result.Counts.Invalid++
		} else if match, ok := matches[idx]; ok {
			audit.Status = "active_" + match.kind
			audit.Matches = refs([]core.Finding{findings[match.finding]})
			switch match.kind {
			case "exact":
				result.Counts.ActiveExact++
			case "context":
				result.Counts.ActiveContext++
			case "content":
				result.Counts.ActiveContent++
			}
		} else {
			result.Counts.Stale++
		}
		result.Entries = append(result.Entries, audit)
	}
	sort.Slice(result.Entries, func(i, j int) bool { return entryKey(result.Entries[i].Entry) < entryKey(result.Entries[j].Entry) })
	result.Duplicates, result.Collisions = fingerprintDiagnostics(file.Entries, exactCurrent, contextCurrent, contentCurrent)
	result.Counts.Active = result.Counts.ActiveExact + result.Counts.ActiveContext + result.Counts.ActiveContent
	result.Counts.Removed = result.Counts.Stale
	result.Counts.Final = result.Counts.Active
	result.ByRule = groupActive(result.Entries, opts, func(e core.BaselineEntry) string { return fallback(e.RuleID, "unknown") })
	result.ByOwner = groupActive(result.Entries, opts, func(e core.BaselineEntry) string { return ownerFor(e.Path, opts.Ownership) })
	result.ByRisk = groupActive(result.Entries, opts, func(e core.BaselineEntry) string { return riskFamily(e.RuleID) })
	sort.SliceStable(result.ByRisk, func(i, j int) bool { return riskRank(result.ByRisk[i].Name) < riskRank(result.ByRisk[j].Name) })
	return result
}

type baselineMatch struct {
	finding int
	kind    string
}

func matchBaselineEntries(entries []core.BaselineEntry, findings []core.Finding) map[int]baselineMatch {
	matches := map[int]baselineMatch{}
	usedFindings := make([]bool, len(findings))
	tiers := []struct {
		kind       string
		entryKey   func(core.BaselineEntry) string
		findingKey func(core.Finding) string
	}{
		{"exact", func(e core.BaselineEntry) string { return e.Fingerprint }, func(f core.Finding) string { return f.Fingerprint }},
		{"context", func(e core.BaselineEntry) string { return e.ContextFingerprint }, func(f core.Finding) string { return f.ContextFingerprint }},
		{"content", func(e core.BaselineEntry) string { return e.ContentFingerprint }, func(f core.Finding) string { return f.ContentFingerprint }},
	}
	for _, tier := range tiers {
		entryGroups := map[string][]int{}
		findingGroups := map[string][]int{}
		for idx, entry := range entries {
			if _, matched := matches[idx]; matched || strings.TrimSpace(entry.Fingerprint) == "" {
				continue
			}
			if key := tier.entryKey(entry); key != "" {
				entryGroups[key] = append(entryGroups[key], idx)
			}
		}
		for idx, finding := range findings {
			if usedFindings[idx] {
				continue
			}
			if key := tier.findingKey(finding); key != "" {
				findingGroups[key] = append(findingGroups[key], idx)
			}
		}
		for key, entryIndexes := range entryGroups {
			findingIndexes := findingGroups[key]
			sort.Slice(entryIndexes, func(i, j int) bool { return entryKey(entries[entryIndexes[i]]) < entryKey(entries[entryIndexes[j]]) })
			sort.Slice(findingIndexes, func(i, j int) bool {
				return findingKey(findings[findingIndexes[i]]) < findingKey(findings[findingIndexes[j]])
			})
			limit := len(entryIndexes)
			if len(findingIndexes) < limit {
				limit = len(findingIndexes)
			}
			for pair := 0; pair < limit; pair++ {
				entryIdx, findingIdx := entryIndexes[pair], findingIndexes[pair]
				matches[entryIdx] = baselineMatch{finding: findingIdx, kind: tier.kind}
				usedFindings[findingIdx] = true
			}
		}
	}
	return matches
}

func findingKey(f core.Finding) string {
	return strings.Join([]string{f.Fingerprint, f.ContextFingerprint, f.ContentFingerprint, f.RuleID, f.Path, f.Message}, "\x00")
}

func (result AuditResult) ActiveEntries() []core.BaselineEntry {
	return entriesWithStatus(result.Entries, "active_")
}

func (result AuditResult) PrunableEntries() []core.BaselineEntry {
	return entriesWithStatus(result.Entries, "stale")
}

func entriesWithStatus(entries []EntryAudit, prefix string) []core.BaselineEntry {
	out := make([]core.BaselineEntry, 0)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Status, prefix) {
			out = append(out, entry.Entry)
		}
	}
	return out
}

func indexFindings(findings []core.Finding, key func(core.Finding) string) map[string][]core.Finding {
	out := map[string][]core.Finding{}
	for _, finding := range findings {
		if value := key(finding); value != "" {
			out[value] = append(out[value], finding)
		}
	}
	for value := range out {
		sortFindings(out[value])
	}
	return out
}

func refs(findings []core.Finding) []FindingRef {
	out := make([]FindingRef, 0, len(findings))
	for _, f := range findings {
		out = append(out, FindingRef{RuleID: f.RuleID, Path: f.Path, Line: f.Line, Message: f.Message, Confidence: f.Confidence, Language: languageFor(f.Path)})
	}
	return out
}

func sortFindings(findings []core.Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Fingerprint < findings[j].Fingerprint
	})
}

func fingerprintDiagnostics(entries []core.BaselineEntry, indexes ...map[string][]core.Finding) ([]Duplicate, []Collision) {
	types := []string{"exact", "context", "content"}
	baselineIndexes := []map[string]int{{}, {}, {}}
	for _, e := range entries {
		for idx, value := range []string{e.Fingerprint, e.ContextFingerprint, e.ContentFingerprint} {
			if value != "" {
				baselineIndexes[idx][value]++
			}
		}
	}
	var duplicates []Duplicate
	var collisions []Collision
	for idx, baselineIndex := range baselineIndexes {
		for fingerprint, count := range baselineIndex {
			if idx == 0 && count > 1 {
				duplicates = append(duplicates, Duplicate{Kind: types[idx], Fingerprint: fingerprint, Count: count})
			}
			currentCount := len(indexes[idx][fingerprint])
			if currentCount > 0 && (count > 1 || currentCount > 1) {
				collisions = append(collisions, Collision{Kind: types[idx], Fingerprint: fingerprint, BaselineEntries: count, CurrentFindings: currentCount})
			}
		}
	}
	sort.Slice(duplicates, func(i, j int) bool {
		return duplicates[i].Kind+duplicates[i].Fingerprint < duplicates[j].Kind+duplicates[j].Fingerprint
	})
	sort.Slice(collisions, func(i, j int) bool {
		return collisions[i].Kind+collisions[i].Fingerprint < collisions[j].Kind+collisions[j].Fingerprint
	})
	return duplicates, collisions
}

func groupActive(entries []EntryAudit, opts Options, name func(core.BaselineEntry) string) []Group {
	type accumulator struct {
		count   int
		samples []FindingRef
	}
	groups := map[string]*accumulator{}
	for _, audited := range entries {
		if !strings.HasPrefix(audited.Status, "active_") {
			continue
		}
		key := name(audited.Entry)
		if groups[key] == nil {
			groups[key] = &accumulator{}
		}
		groups[key].count++
		groups[key].samples = append(groups[key].samples, audited.Matches...)
	}
	out := make([]Group, 0, len(groups))
	for name, group := range groups {
		sort.Slice(group.samples, func(i, j int) bool { return sampleKey(group.samples[i]) < sampleKey(group.samples[j]) })
		confidence := distribution(group.samples, func(sample FindingRef) string { return fallback(sample.Confidence, "unknown") })
		languages := distribution(group.samples, func(sample FindingRef) string { return fallback(sample.Language, "unknown") })
		limit := opts.SampleLimit
		if limit <= 0 {
			limit = 3
		}
		if len(group.samples) > limit {
			group.samples = group.samples[:limit]
		}
		out = append(out, Group{Name: name, Count: group.count, Samples: group.samples, Confidence: confidence, Languages: languages})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func distribution(samples []FindingRef, name func(FindingRef) string) []NamedCount {
	counts := map[string]int{}
	for _, sample := range samples {
		counts[name(sample)]++
	}
	out := make([]NamedCount, 0, len(counts))
	for name, count := range counts {
		out = append(out, NamedCount{Name: name, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func ownerFor(path string, mappings []OwnershipMapping) string {
	path = filepath.ToSlash(path)
	for _, mapping := range mappings {
		if matchGlob(mapping.Pattern, path) {
			return mapping.Owner
		}
	}
	if idx := strings.IndexByte(path, '/'); idx >= 0 {
		return path[:idx]
	}
	return fallback(path, "root")
}

func matchGlob(pattern, path string) bool {
	pattern = filepath.ToSlash(pattern)
	if strings.HasSuffix(pattern, "/**") {
		return strings.HasPrefix(path, strings.TrimSuffix(pattern, "**"))
	}
	matched, _ := filepath.Match(pattern, path)
	return matched
}

func riskFamily(rule string) string {
	switch {
	case strings.HasPrefix(rule, "security."):
		return "security"
	case strings.HasPrefix(rule, "defensive."):
		return "boundary"
	case strings.HasPrefix(rule, "error."):
		return "error-handling"
	case hasAnyPrefix(rule, "quality.", "function.", "smell.", "naming."):
		return "structural-quality"
	default:
		return "other"
	}
}

func riskRank(name string) int {
	for idx, value := range []string{"security", "boundary", "error-handling", "structural-quality", "other"} {
		if name == value {
			return idx
		}
	}
	return 99
}
func hasAnyPrefix(value string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
func languageFor(path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	return fallback(ext, "unknown")
}
func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return value
}
func entryKey(e core.BaselineEntry) string {
	return strings.Join([]string{e.Fingerprint, e.ContextFingerprint, e.ContentFingerprint, e.RuleID, e.Path, e.Message}, "\x00")
}
func sampleKey(s FindingRef) string {
	return strings.Join([]string{s.Path, s.RuleID, s.Message}, "\x00")
}
