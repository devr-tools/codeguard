package report

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	rulespkg "github.com/devr-tools/codeguard/internal/codeguard/rules"
)

// sarifRuleFindings groups the findings a run reported for one rule, with the
// id of the section that produced the first of them.
type sarifRuleFindings struct {
	sectionID string
	findings  []core.Finding
}

func buildSARIFRule(catalog map[string]core.RuleMetadata, ruleID string, group sarifRuleFindings) sarifRule {
	meta, inCatalog := catalog[ruleID]
	first := group.findings[0]
	meta.Title = firstNonEmpty(first.Title, meta.Title, ruleID)
	meta.HowToFix = firstNonEmpty(first.HowToFix, meta.HowToFix)
	meta.Description = firstNonEmpty(meta.Description, first.Why, first.Message)

	rule := sarifRule{
		ID:               ruleID,
		ShortDescription: sarifTextBlock{Text: meta.Title},
		FullDescription:  sarifTextBlock{Text: meta.Description},
		DefaultConfiguration: &sarifRuleConfiguration{
			Level: sarifLevel(firstNonEmpty(meta.DefaultLevel, first.Level)),
		},
	}
	if inCatalog {
		rule.HelpURI = rulespkg.DocsRuleURL(ruleID)
	}
	rule.Help = &sarifMultiformatMessage{
		Text:     sarifHelpText(meta),
		Markdown: sarifHelpMarkdown(meta, rule.HelpURI),
	}
	rule.Properties = buildSARIFRuleProperties(meta, ruleID, group)
	if meta.OWASPCategory != "" {
		rule.Relationships = []sarifRelationship{{
			Target: sarifReportingDescriptorReference{
				ID:            meta.OWASPCategory.Code(),
				ToolComponent: sarifToolComponent{Name: owaspTaxonomyName, GUID: owaspTaxonomyGUID},
			},
			Kinds: []string{"superset"},
		}}
	}
	return rule
}

// buildSARIFRuleProperties derives the rule-level property bag. precision and
// security-severity are rule-level in SARIF, so they take the strongest value
// among the rule's findings in this run.
func buildSARIFRuleProperties(meta core.RuleMetadata, ruleID string, group sarifRuleFindings) *sarifRuleProperties {
	props := &sarifRuleProperties{OWASP: string(meta.OWASPCategory)}
	var tags []string
	if section := sectionTag(firstNonEmpty(group.sectionID, meta.Section, strings.SplitN(ruleID, ".", 2)[0])); section != "" {
		tags = append(tags, section)
	}
	security := isSecurityRule(meta, ruleID)
	if security && !containsString(tags, "security") {
		tags = append(tags, "security")
	}
	for _, cwe := range meta.CWE {
		if tag, ok := cweTag(cwe); ok {
			tags = append(tags, tag)
			props.CWE = append(props.CWE, cwe)
		}
	}
	if meta.OWASPCategory != "" {
		tags = append(tags, "OWASP:"+meta.OWASPCategory.Code())
	}
	props.Tags = tags

	bestRank := 0
	bestScore := 0.0
	for _, finding := range group.findings {
		if rank := core.ConfidenceRank(finding.Confidence); rank > bestRank {
			bestRank = rank
			props.Precision = confidenceOrMedium(finding.Confidence)
		}
		if score := securitySeverityScore(finding.Level, finding.Confidence); score > bestScore {
			bestScore = score
		}
	}
	if security {
		props.SecuritySeverity = formatSecuritySeverity(bestScore)
	}
	return props
}

func sarifHelpText(meta core.RuleMetadata) string {
	parts := []string{meta.Description}
	if meta.HowToFix != "" {
		parts = append(parts, "How to fix: "+meta.HowToFix)
	}
	if meta.FixTemplate.Text != "" {
		parts = append(parts, meta.FixTemplate.Text)
	}
	return strings.Join(parts, "\n\n")
}

func sarifHelpMarkdown(meta core.RuleMetadata, helpURI string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\n%s\n", meta.Title, meta.Description)
	if meta.HowToFix != "" {
		fmt.Fprintf(&b, "\n### How to fix\n\n%s\n", meta.HowToFix)
	}
	if meta.FixTemplate.Text != "" {
		fence := strings.Repeat("~", longestRun(meta.FixTemplate.Text, '~')+3)
		fmt.Fprintf(&b, "\n### Example\n\n%s\n%s\n%s\n", fence, meta.FixTemplate.Text, fence)
	}
	var refs []string
	for _, cwe := range meta.CWE {
		if number, ok := cweNumber(cwe); ok {
			refs = append(refs, fmt.Sprintf("[CWE-%d](https://cwe.mitre.org/data/definitions/%d.html)", number, number))
		}
	}
	if meta.OWASPCategory != "" {
		refs = append(refs, "OWASP "+string(meta.OWASPCategory))
	}
	if helpURI != "" {
		refs = append(refs, fmt.Sprintf("[codeguard rule reference](%s)", helpURI))
	}
	if len(refs) > 0 {
		b.WriteString("\n### References\n\n")
		for _, ref := range refs {
			fmt.Fprintf(&b, "- %s\n", ref)
		}
	}
	return b.String()
}

func buildSARIFResult(finding core.Finding) sarifResult {
	result := sarifResult{
		RuleID:  finding.RuleID,
		Level:   sarifLevelForFinding(finding),
		Message: sarifMessage{Text: firstNonEmpty(finding.Why, finding.Message)},
	}
	if finding.Path != "" {
		result.Locations = append(result.Locations, newSARIFLocation(finding))
	}
	result.CodeFlows = sarifCodeFlows(finding)
	result.PartialFingerprints = sarifPartialFingerprints(finding)
	if finding.Confidence != "" {
		result.Properties = &sarifResultProperties{Confidence: finding.Confidence}
	}
	return result
}

func buildSARIFDiagnostic(diagnostic core.Diagnostic) sarifResult {
	result := sarifResult{
		RuleID:     diagnostic.ID,
		Level:      sarifLevel(diagnostic.Level),
		Message:    sarifMessage{Text: diagnostic.Message},
		Properties: &sarifResultProperties{Kind: diagnostic.Kind, Evidence: diagnostic.Evidence, Metadata: diagnostic.Metadata},
	}
	if diagnostic.Path != "" {
		result.Locations = []sarifLocation{{PhysicalLocation: &sarifPhysicalLocation{ArtifactLocation: sarifArtifactLocation{URI: diagnostic.Path}}}}
	}
	return result
}

// sarifPartialFingerprints exposes both codeguard fingerprints to SARIF
// consumers. GitHub code scanning deduplicates alerts across commits by
// partialFingerprints, so the line-shift-resilient context fingerprint keeps
// an alert stable when unrelated edits move the finding within its file, while
// the legacy line-based fingerprint preserves continuity with older uploads.
func sarifPartialFingerprints(finding core.Finding) map[string]string {
	fingerprints := map[string]string{}
	if finding.ContextFingerprint != "" {
		fingerprints["codeguardContext/v1"] = finding.ContextFingerprint
	}
	if finding.Fingerprint != "" {
		fingerprints["codeguardLegacy/v1"] = finding.Fingerprint
	}
	if len(fingerprints) == 0 {
		return nil
	}
	return fingerprints
}

// sarifLevelForFinding keeps findings at error or warning: a finding with an
// unexpected level is still something to act on, so it never drops to note.
func sarifLevelForFinding(finding core.Finding) string {
	if finding.Level == "fail" {
		return "error"
	}
	return "warning"
}

// newSARIFLocation always carries a region so GitHub can anchor the alert;
// file-level findings (no line) point at line 1. The column is only emitted
// when the check reported one.
func newSARIFLocation(finding core.Finding) sarifLocation {
	region := &sarifRegion{StartLine: max(1, finding.Line)}
	if finding.Column > 0 {
		region.StartColumn = finding.Column
	}
	return sarifLocation{
		PhysicalLocation: &sarifPhysicalLocation{
			ArtifactLocation: sarifArtifactLocation{URI: finding.Path},
			Region:           region,
		},
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

// longestRun returns the length of the longest run of r in text, so a code
// fence can be made longer than any fence-like run inside the fenced text.
func longestRun(text string, r rune) int {
	longest, current := 0, 0
	for _, c := range text {
		if c != r {
			current = 0
			continue
		}
		current++
		longest = max(longest, current)
	}
	return longest
}
