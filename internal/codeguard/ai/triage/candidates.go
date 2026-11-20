package triage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/security"
	checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func collectCandidates(cfg core.Config, sections []core.SectionResult) []candidate {
	resolver := newSourceResolver(cfg)
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
	paths   map[string]string
	secrets security.Scanner
}

// newSourceResolver indexes the regular files of every target by
// target-relative path. Symlinks are not followed: a snippet is read only from
// a file the scan itself could list.
func newSourceResolver(cfg core.Config) sourceResolver {
	paths := make(map[string]string)
	for _, target := range cfg.Targets {
		root := filepath.Clean(target.Path)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || !info.Mode().IsRegular() {
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
	return sourceResolver{paths: paths, secrets: snippetSecretScanner(cfg)}
}

// snippetSecretScanner returns the configured secret scanner (so custom
// patterns are redacted too), or the built-in one when secret scanning is
// turned off: redaction must not depend on whether the scan reports secrets.
func snippetSecretScanner(cfg core.Config) security.Scanner {
	if scanner, _ := security.BuildScanner(cfg.Checks.SecurityRules.Secrets); scanner.Enabled() {
		return scanner
	}
	scanner, _ := security.BuildScanner(nil)
	return scanner
}

// redactedSnippetLine replaces a source line that may hold secret material.
const redactedSnippetLine = "<redacted: possible secret>"

// secretRuleIDs are the rules whose finding line is itself the secret.
var secretRuleIDs = map[string]bool{
	"security.hardcoded-credential": true,
	"security.private-key":          true,
	"security.high-entropy-string":  true,
}

// snippetForFinding returns the source window sent to the triage provider.
// The report and SARIF mask secret values, and the snippet leaves this
// machine, so every line in the window that the secret scanner flags, every
// line of a PEM private-key block, and the finding's own line for a secret
// rule are redacted before it is sent.
func (resolver sourceResolver) snippetForFinding(finding core.Finding) string {
	if resolver.paths == nil || strings.TrimSpace(finding.Path) == "" {
		return ""
	}
	path, ok := resolver.paths[filepath.ToSlash(finding.Path)]
	if !ok {
		return ""
	}
	data, err := checkSupport.ReadCappedFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	start, end, numbered := 0, minInt(8, len(lines)), false
	if finding.Line > 0 && finding.Line <= len(lines) {
		start, end, numbered = maxInt(0, finding.Line-4), minInt(len(lines), finding.Line+3), true
	}
	redact := resolver.secretLines(lines, start, end)
	if secretRuleIDs[finding.RuleID] || finding.Metadata["secret_type"] != "" {
		if finding.Line > 0 {
			redact[finding.Line-1] = true
		} else {
			return ""
		}
	}
	window := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		text := lines[i]
		if redact[i] {
			text = redactedSnippetLine
		}
		if numbered {
			text = fmt.Sprintf("%d: %s", i+1, text)
		}
		window = append(window, text)
	}
	return strings.TrimSpace(strings.Join(window, "\n"))
}

// secretLines returns the 0-based indexes in [start, end) that may hold secret
// material: lines the secret scanner matches, and every line of a PEM
// private-key block (whose base64 body has no marker of its own).
func (resolver sourceResolver) secretLines(lines []string, start int, end int) map[int]bool {
	redact := map[int]bool{}
	for _, match := range resolver.secrets.ScanContent(strings.Join(lines[start:end], "\n")) {
		redact[start+match.Line-1] = true
	}
	inKey := false
	for i := 0; i < end; i++ {
		line := lines[i]
		if strings.Contains(line, "-----BEGIN") && strings.Contains(line, "PRIVATE KEY") {
			inKey = true
		}
		if inKey && i >= start {
			redact[i] = true
		}
		if inKey && strings.Contains(line, "-----END") {
			inKey = false
		}
	}
	return redact
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
