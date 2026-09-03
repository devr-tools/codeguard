package core

import (
	"sort"
	"strings"
)

// Section ids as they appear in scan output (SectionResult.ID) and in
// per-section configuration such as checks.disabled and
// ConfidencePolicyConfig.Sections.
//
// These are the ids sections finalize with, which is what users see in a
// report — note that supply chain finalizes as "supply_chain" while the
// runner's internal registry id is "supply-chain". tests/codeguard asserts
// that every section a real scan reports is present here, so a new section
// cannot silently become unconfigurable.
var sectionKeys = map[string]struct{}{
	"change":        {},
	"quality":       {},
	"performance":   {},
	"reliability":   {},
	"data":          {},
	"observability": {},
	"operations":    {},
	"design":        {},
	"security":      {},
	"prompts":       {},
	"ci":            {},
	"delivery":      {},
	"supply_chain":  {},
	"context":       {},
	"contracts":     {},
	"custom":        {},
}

// NormalizedSectionKey trims and lowercases a section id so configuration keys
// match the runner's ids regardless of casing or surrounding space.
func NormalizedSectionKey(sectionID string) string {
	return strings.TrimSpace(strings.ToLower(sectionID))
}

// KnownSectionKey reports whether sectionID names a registered check section.
func KnownSectionKey(sectionID string) bool {
	_, ok := sectionKeys[NormalizedSectionKey(sectionID)]
	return ok
}

// SectionKeys returns the registered section ids in sorted order, for error
// messages and documentation.
func SectionKeys() []string {
	keys := make([]string, 0, len(sectionKeys))
	for key := range sectionKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
