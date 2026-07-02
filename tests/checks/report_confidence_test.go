package checks_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func confidenceReport() codeguard.Report {
	return codeguard.Report{
		Name: "confidence-test",
		Sections: []codeguard.SectionResult{{
			ID:     "security",
			Name:   "Security",
			Status: codeguard.StatusWarn,
			Findings: []codeguard.Finding{
				{
					RuleID:      "security.hardcoded-secret",
					Level:       "warn",
					Confidence:  "low",
					Title:       "Hardcoded secret",
					Message:     "possible hardcoded secret detected",
					Why:         "possible hardcoded secret detected",
					Path:        "config.go",
					Line:        3,
					Fingerprint: "conf-low",
				},
				{
					RuleID:      "security.hardcoded-credential",
					Level:       "warn",
					Confidence:  "high",
					Title:       "Hardcoded credential",
					Message:     "possible hardcoded credential detected",
					Why:         "possible hardcoded credential detected",
					Path:        "config.go",
					Line:        7,
					Fingerprint: "conf-high",
				},
				{
					RuleID:      "security.shell-execution",
					Level:       "warn",
					Title:       "Shell execution review",
					Message:     "shell execution primitive should be reviewed",
					Why:         "shell execution primitive should be reviewed",
					Path:        "run.go",
					Line:        9,
					Fingerprint: "conf-unset",
				},
			},
		}},
		Summary: codeguard.ReportSummary{WarnedSections: 1, TotalFindings: 3},
	}
}

func TestTextReportMarksOnlyLowConfidenceFindings(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var out bytes.Buffer
	if err := codeguard.WriteReport(&out, confidenceReport(), "text"); err != nil {
		t.Fatalf("write text: %v", err)
	}

	rendered := out.String()
	assertContains(t, rendered, "why: possible hardcoded secret detected (low confidence)",
		"expected low-confidence suffix on the low-confidence finding")
	if got := strings.Count(rendered, "(low confidence)"); got != 1 {
		t.Fatalf("expected exactly one low-confidence marker, got %d in:\n%s", got, rendered)
	}
}

func TestSARIFReportCarriesConfidenceProperty(t *testing.T) {
	var out bytes.Buffer
	if err := codeguard.WriteReport(&out, confidenceReport(), "sarif"); err != nil {
		t.Fatalf("write sarif: %v", err)
	}

	rendered := out.String()
	assertContains(t, rendered, `"confidence": "low"`, "expected low confidence in the SARIF property bag")
	assertContains(t, rendered, `"confidence": "high"`, "expected high confidence in the SARIF property bag")
	// The unset finding must not carry a property bag entry.
	if got := strings.Count(rendered, `"confidence"`); got != 2 {
		t.Fatalf("expected exactly two confidence properties, got %d in:\n%s", got, rendered)
	}
}

func TestJSONReportIncludesConfidenceField(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := codeguard.WriteReport(&out, confidenceReport(), "json"); err != nil {
		t.Fatalf("write json: %v", err)
	}

	rendered := out.String()
	assertContains(t, rendered, `"confidence": "low"`, "expected confidence field in JSON output")
	// omitempty: the unset finding carries no confidence key.
	if got := strings.Count(rendered, `"confidence"`); got != 2 {
		t.Fatalf("expected exactly two confidence fields, got %d in:\n%s", got, rendered)
	}
}
