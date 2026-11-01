package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// Page-size bounds for MCP tools that return findings. The default keeps a
// first response small enough for an agent's context window on a large repo;
// the ceiling stops a single call from re-creating the unbounded dump the
// pagination exists to avoid.
const (
	mcpDefaultMaxFindings = 50
	mcpMaxFindingsCeiling = 500
)

// Embedding one filter struct in every findings-returning tool's arguments
// (scan, validate_patch) keeps their filter inputs from drifting apart.
type mcpFindingQuery struct {
	Sections      []string `json:"sections"`
	Rules         []string `json:"rules"`
	MinLevel      string   `json:"min_level"`
	MinConfidence string   `json:"min_confidence"`
	Paths         []string `json:"paths"`
	MaxFindings   *int     `json:"max_findings"`
	Cursor        string   `json:"cursor"`
	Verbose       bool     `json:"verbose"`
}

// mcpFindingFilters is the normalized, validated form of mcpFindingQuery.
type mcpFindingFilters struct {
	Sections      []string `json:"sections,omitempty"`
	Rules         []string `json:"rules,omitempty"`
	MinLevel      string   `json:"min_level,omitempty"`
	MinConfidence string   `json:"min_confidence,omitempty"`
	Paths         []string `json:"paths,omitempty"`
	MaxFindings   int      `json:"max_findings"`
	Verbose       bool     `json:"verbose,omitempty"`
}

// normalize validates the query and returns the filters plus the page size.
func (q mcpFindingQuery) normalize() (mcpFindingFilters, error) {
	filters := mcpFindingFilters{
		Rules:   cleanStringList(q.Rules),
		Paths:   cleanStringList(q.Paths),
		Verbose: q.Verbose,
	}
	sections, err := normalizeSectionFilter(q.Sections)
	if err != nil {
		return filters, err
	}
	filters.Sections = sections
	if filters.MinLevel, err = normalizeMinLevel(q.MinLevel); err != nil {
		return filters, err
	}
	if filters.MinConfidence, err = normalizeMinConfidence(q.MinConfidence); err != nil {
		return filters, err
	}
	if filters.MaxFindings, err = normalizeMaxFindings(q.MaxFindings); err != nil {
		return filters, err
	}
	return filters, nil
}

func cleanStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func normalizeSectionFilter(values []string) ([]string, error) {
	sections := cleanStringList(values)
	for i, section := range sections {
		key := core.NormalizedSectionKey(section)
		if !core.KnownSectionKey(key) {
			return nil, fmt.Errorf("unknown section %q: must be one of %s", section, strings.Join(core.SectionKeys(), ", "))
		}
		sections[i] = key
	}
	return cleanStringList(sections), nil
}

func normalizeMinLevel(raw string) (string, error) {
	level := strings.ToLower(strings.TrimSpace(raw))
	switch level {
	case "", string(core.StatusWarn), string(core.StatusFail):
		return level, nil
	default:
		return "", fmt.Errorf("invalid min_level %q: must be warn or fail", raw)
	}
}

func normalizeMinConfidence(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	level := core.NormalizedConfidence(raw)
	if level == "" {
		return "", fmt.Errorf("invalid min_confidence %q: must be one of %s, %s, %s", raw,
			core.ConfidenceLow, core.ConfidenceMedium, core.ConfidenceHigh)
	}
	return level, nil
}

func normalizeMaxFindings(raw *int) (int, error) {
	if raw == nil {
		return mcpDefaultMaxFindings, nil
	}
	if *raw < 0 {
		return 0, fmt.Errorf("invalid max_findings %d: must be between 0 and %d", *raw, mcpMaxFindingsCeiling)
	}
	if *raw > mcpMaxFindingsCeiling {
		return mcpMaxFindingsCeiling, nil
	}
	return *raw, nil
}

// fingerprint identifies the filter set that decides which findings match and
// in what order. Page size and verbosity are excluded: they change how much of
// the matched list one page shows, not the list itself, so an agent can change
// them between pages without invalidating its cursor.
func (f mcpFindingFilters) fingerprint() string {
	key := struct {
		Sections      []string `json:"s"`
		Rules         []string `json:"r"`
		MinLevel      string   `json:"l"`
		MinConfidence string   `json:"c"`
		Paths         []string `json:"p"`
	}{f.Sections, f.Rules, f.MinLevel, f.MinConfidence, f.Paths}
	data, _ := json.Marshal(key)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

// Reasons a finding was excluded by an MCP filter, in the order the filters
// are evaluated. A finding excluded by several filters is attributed to the
// first, so the per-reason counts sum to the number filtered out.
const (
	filterReasonNone     = ""
	filterReasonSection  = "sections"
	filterReasonRule     = "rules"
	filterReasonMinLevel = "min_level"
	filterReasonPath     = "paths"
)

func (f mcpFindingFilters) exclusionReason(sectionID string, finding core.Finding) string {
	switch {
	case len(f.Sections) > 0 && !containsString(f.Sections, core.NormalizedSectionKey(sectionID)):
		return filterReasonSection
	case len(f.Rules) > 0 && !matchesRuleFilter(f.Rules, finding.RuleID):
		return filterReasonRule
	case f.MinLevel != "" && levelRank(finding.Level) < levelRank(f.MinLevel):
		return filterReasonMinLevel
	case len(f.Paths) > 0 && !matchesPathFilter(f.Paths, finding.Path):
		return filterReasonPath
	default:
		return filterReasonNone
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// matchesRuleFilter treats an entry ending in "." or "*" as a prefix
// ("security.", "prompts.*") and anything else as an exact rule id.
func matchesRuleFilter(entries []string, ruleID string) bool {
	for _, entry := range entries {
		if prefix, ok := strings.CutSuffix(entry, "*"); ok {
			if strings.HasPrefix(ruleID, prefix) {
				return true
			}
			continue
		}
		if strings.HasSuffix(entry, ".") && strings.HasPrefix(ruleID, entry) {
			return true
		}
		if ruleID == entry {
			return true
		}
	}
	return false
}

// matchesPathFilter uses the same glob dialect as config excludes ("*" within
// a segment, "**" across segments). Findings without a path never match.
func matchesPathFilter(patterns []string, path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	for _, pattern := range patterns {
		if runnersupport.MatchPattern(pattern, path) {
			return true
		}
	}
	return false
}

// levelRank orders finding levels so a higher rank is more severe. Levels
// other than warn and fail (info, pass) rank lowest.
func levelRank(level string) int {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case string(core.StatusFail):
		return 2
	case string(core.StatusWarn):
		return 1
	default:
		return 0
	}
}
