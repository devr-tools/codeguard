package report

import (
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type sarifRule struct {
	ID               string `json:"id"`
	ShortDescription struct {
		Text string `json:"text"`
	} `json:"shortDescription"`
	FullDescription struct {
		Text string `json:"text"`
	} `json:"fullDescription"`
	Help struct {
		Text string `json:"text,omitempty"`
	} `json:"help,omitempty"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
}

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
	return result
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
