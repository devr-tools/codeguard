package report

import "github.com/devr-tools/codeguard/internal/codeguard/core"

func buildSARIFRule(catalog map[string]core.RuleMetadata, finding core.Finding) sarifRule {
	meta := catalog[finding.RuleID]
	if finding.Title != "" || finding.HowToFix != "" {
		meta.Title = firstNonEmpty(finding.Title, meta.Title)
		meta.HowToFix = firstNonEmpty(finding.HowToFix, meta.HowToFix)
	}
	meta.Description = firstNonEmpty(meta.Description, finding.Message)

	rule := sarifRule{ID: finding.RuleID}
	rule.ShortDescription.Text = meta.Title
	rule.FullDescription.Text = meta.Description
	rule.Help.Text = meta.HowToFix
	if meta.OWASPCategory != "" {
		rule.Properties = &sarifRuleProperties{
			Tags:  []string{"security", "OWASP:" + meta.OWASPCategory.Code()},
			OWASP: string(meta.OWASPCategory),
		}
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

func buildSARIFResult(finding core.Finding) sarifResult {
	result := sarifResult{
		RuleID:  finding.RuleID,
		Level:   sarifLevelForFinding(finding),
		Message: sarifMessage{Text: firstNonEmpty(finding.Why, finding.Message)},
	}
	if finding.Path != "" {
		result.Locations = append(result.Locations, newSARIFLocation(finding))
	}
	result.PartialFingerprints = sarifPartialFingerprints(finding)
	if finding.Confidence != "" {
		result.Properties = &sarifResultProperties{Confidence: finding.Confidence}
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

func sarifLevelForFinding(finding core.Finding) string {
	if finding.Level == "fail" {
		return "error"
	}
	return "warning"
}

func newSARIFLocation(finding core.Finding) sarifLocation {
	return sarifLocation{
		PhysicalLocation: sarifPhysicalLocation{
			ArtifactLocation: sarifArtifactLocation{URI: finding.Path},
			Region: sarifRegion{
				StartLine:   max(1, finding.Line),
				StartColumn: max(1, finding.Column),
			},
		},
	}
}
