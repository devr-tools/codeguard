package triage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func collectCandidates(cfg core.Config, sections []core.SectionResult) []candidate {
	resolver := newSourceResolver(cfg.Targets)
	filter := newCandidateFilter(cfg.AI.HybridTriage)
	candidates := make([]candidate, 0)
	for sectionIndex, section := range sections {
		if !filter.section(section) {
			continue
		}
		for findingIndex, finding := range section.Findings {
			if !filter.severity(finding) {
				continue
			}
			item := candidate{
				sectionIndex: sectionIndex,
				findingIndex: findingIndex,
				sectionName:  section.Name,
				finding:      finding,
				snippet:      resolver.snippetForFinding(finding),
			}
			item.hash = contentHash(item)
			candidates = append(candidates, item)
		}
	}
	return candidates
}

// candidateFilter narrows triage to ai.hybrid_triage.candidate_sections and
// candidate_severities. An empty list places no restriction, so SDK callers
// that build a Config without defaults keep triaging every finding.
//
// Before 1.12 candidate_sections was parsed but never applied, and `init`
// wrote legacyDefaultCandidateSections into every generated config. That
// exact list is therefore treated as "no restriction" in 1.x, so upgrading
// does not silently stop triaging Reliability, CI/CD and the other sections;
// a list a user actually customised is honoured.
type candidateFilter struct {
	sections   map[string]struct{}
	severities map[string]struct{}
}

var legacyDefaultCandidateSections = []string{"Code Quality", "Design Patterns", "Security", "Custom Rules"}

func newCandidateFilter(cfg core.AIHybridTriageConfig) candidateFilter {
	sections := lowerSet(cfg.CandidateSections)
	if sameSet(sections, lowerSet(legacyDefaultCandidateSections)) {
		sections = nil
	}
	return candidateFilter{sections: sections, severities: lowerSet(cfg.CandidateSeverities)}
}

func sameSet(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for key := range a {
		if _, ok := b[key]; !ok {
			return false
		}
	}
	return true
}

// section matches a section by display name ("Code Quality") or id
// ("quality"), case-insensitively.
func (filter candidateFilter) section(section core.SectionResult) bool {
	if len(filter.sections) == 0 {
		return true
	}
	_, byName := filter.sections[strings.ToLower(strings.TrimSpace(section.Name))]
	_, byID := filter.sections[strings.ToLower(strings.TrimSpace(section.ID))]
	return byName || byID
}

func (filter candidateFilter) severity(finding core.Finding) bool {
	if len(filter.severities) == 0 {
		return true
	}
	_, ok := filter.severities[strings.ToLower(strings.TrimSpace(finding.Level))]
	return ok
}

func lowerSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		if trimmed := strings.ToLower(strings.TrimSpace(value)); trimmed != "" {
			out[trimmed] = struct{}{}
		}
	}
	return out
}

func contentHash(item candidate) string {
	payload := map[string]any{
		"section":    item.sectionName,
		"rule_id":    item.finding.RuleID,
		"level":      item.finding.Level,
		"severity":   item.finding.Severity,
		"title":      item.finding.Title,
		"message":    item.finding.Message,
		"why":        item.finding.Why,
		"how_to_fix": item.finding.HowToFix,
		"path":       item.finding.Path,
		"line":       item.finding.Line,
		"column":     item.finding.Column,
		"snippet":    item.snippet,
	}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type sourceResolver struct {
	paths map[string]string
}

func newSourceResolver(targets []core.TargetConfig) sourceResolver {
	paths := make(map[string]string)
	for _, target := range targets {
		root := filepath.Clean(target.Path)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if _, exists := paths[rel]; !exists {
				paths[rel] = path
			}
			return nil
		})
	}
	return sourceResolver{paths: paths}
}

func (resolver sourceResolver) snippetForFinding(finding core.Finding) string {
	if resolver.paths == nil || strings.TrimSpace(finding.Path) == "" {
		return ""
	}
	path, ok := resolver.paths[filepath.ToSlash(finding.Path)]
	if !ok {
		return ""
	}
	data, err := os.ReadFile(path) //nolint:gosec // path resolved from the scan-target file walk in newSourceResolver
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if finding.Line <= 0 || finding.Line > len(lines) {
		return strings.TrimSpace(strings.Join(lines[:minInt(8, len(lines))], "\n"))
	}
	start := maxInt(0, finding.Line-4)
	end := minInt(len(lines), finding.Line+3)
	window := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		window = append(window, fmt.Sprintf("%d: %s", i+1, lines[i]))
	}
	return strings.TrimSpace(strings.Join(window, "\n"))
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
