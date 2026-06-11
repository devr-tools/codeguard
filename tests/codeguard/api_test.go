package codeguard_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/codeguard"
)

func TestValidateConfigRejectsBlankWorkflowNeedle(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.Checks.CIRules.WorkflowContentRules = []codeguard.WorkflowRuleConfig{{
		Path:             ".github/workflows/ci.yml",
		RequiredContains: []string{"make test", "  "},
	}}

	err := codeguard.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "required_contains[1]") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestWriteReportTextIncludesSummary(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	report := codeguard.Report{
		Name:        "sample",
		GeneratedAt: time.Date(2026, 6, 10, 19, 30, 0, 0, time.UTC),
		Sections: []codeguard.SectionResult{
			{
				Name:   "Code Quality",
				Status: "warn",
				Note:   "Maintainability warning",
				Findings: []codeguard.Finding{{
					Path:     "main.go",
					Message:  "function is too long",
					Severity: "warn",
				}},
			},
		},
	}

	var out bytes.Buffer
	if err := codeguard.WriteReport(&out, report, "text"); err != nil {
		t.Fatalf("write text report: %v", err)
	}

	rendered := out.String()
	rendered = stripANSI(rendered)
	if !strings.Contains(rendered, "CodeGuard Report sample") {
		t.Fatalf("missing header in report:\n%s", rendered)
	}
	if !strings.Contains(rendered, "✓ 0 pass  0 warn  0 fail  0 skip") {
		t.Fatalf("missing summary in report:\n%s", rendered)
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}
