package codeguard_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestValidateConfigRejectsBlankTargetPath(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: "", Language: "go"}}

	err := codeguard.ValidateConfig(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "target path is required") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestWriteReportTextIncludesSummary(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	report := codeguard.Report{
		Name:        "sample",
		GeneratedAt: "2026-06-10T19:30:00Z",
		Sections: []codeguard.SectionResult{
			{
				Name:   "Code Quality",
				Status: "warn",
				Findings: []codeguard.Finding{{
					RuleID:      "quality.max-function-lines",
					Level:       "warn",
					Path:        "main.go",
					Line:        12,
					Message:     "function is too long",
					Severity:    "warn",
					Fingerprint: "abc123",
				}},
			},
		},
		Summary: codeguard.ReportSummary{
			WarnedSections: 1,
			TotalFindings:  1,
		},
	}

	var out bytes.Buffer
	if err := codeguard.WriteReport(&out, report, "text"); err != nil {
		t.Fatalf("write text report: %v", err)
	}

	rendered := out.String()
	rendered = stripANSI(rendered)
	if !strings.Contains(rendered, "sample") {
		t.Fatalf("missing header in report:\n%s", rendered)
	}
	if !strings.Contains(rendered, "quality.max-function-lines") {
		t.Fatalf("missing grouped finding subsection in report:\n%s", rendered)
	}
	if !strings.Contains(rendered, "1. at: main.go:12") {
		t.Fatalf("missing finding location in report:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Summary: 0 pass, 1 warn, 0 fail, 1 findings, 0 suppressed") {
		t.Fatalf("missing summary in report:\n%s", rendered)
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}
