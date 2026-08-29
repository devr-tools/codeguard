package cli

import (
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type PolicyViolation struct {
	Kind    string              `json:"kind"`
	Message string              `json:"message"`
	Entry   *core.BaselineEntry `json:"entry,omitempty"`
}

type PolicyResult struct {
	Added      []core.BaselineEntry `json:"added"`
	Removed    []core.BaselineEntry `json:"removed"`
	Violations []PolicyViolation    `json:"violations"`
}

func ComparePolicy(current, comparison core.BaselineFile, policy core.BaselineGovernanceConfig) PolicyResult {
	result := PolicyResult{
		Added:   multisetDifference(current.Entries, comparison.Entries),
		Removed: multisetDifference(comparison.Entries, current.Entries),
	}
	if policy.MaxEntries > 0 && len(current.Entries) > policy.MaxEntries {
		result.Violations = append(result.Violations, PolicyViolation{Kind: "max_entries", Message: "baseline exceeds configured maximum"})
	}
	if policy.ForbidGrowth && len(current.Entries) > len(comparison.Entries) {
		result.Violations = append(result.Violations, PolicyViolation{Kind: "growth", Message: "baseline entry count increased"})
	}
	for idx := range result.Added {
		entry := &result.Added[idx]
		for _, prefix := range policy.ProhibitedNewRulePrefixes {
			if strings.HasPrefix(entry.RuleID, prefix) {
				result.Violations = append(result.Violations, PolicyViolation{Kind: "prohibited_rule", Message: "new baseline entry belongs to a prohibited rule family", Entry: entry})
				break
			}
		}
	}
	sort.Slice(result.Violations, func(i, j int) bool {
		left, right := result.Violations[i], result.Violations[j]
		leftKey, rightKey := left.Kind, right.Kind
		if left.Entry != nil {
			leftKey += entryKey(*left.Entry)
		}
		if right.Entry != nil {
			rightKey += entryKey(*right.Entry)
		}
		return leftKey < rightKey
	})
	return result
}

func multisetDifference(left, right []core.BaselineEntry) []core.BaselineEntry {
	counts := map[string]int{}
	for _, entry := range right {
		counts[entryKey(entry)]++
	}
	out := make([]core.BaselineEntry, 0)
	for _, entry := range left {
		key := entryKey(entry)
		if counts[key] > 0 {
			counts[key]--
			continue
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return entryKey(out[i]) < entryKey(out[j]) })
	return out
}
