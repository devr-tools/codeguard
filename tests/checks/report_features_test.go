package checks_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestWriteReportSupportsSARIFAndGitHub(t *testing.T) {
	report := formatReport()

	var text bytes.Buffer
	t.Setenv("NO_COLOR", "")
	if err := codeguard.WriteReport(&text, report, "text"); err != nil {
		t.Fatalf("write text: %v", err)
	}
	assertTextReportFormatting(t, &text)

	t.Setenv("NO_COLOR", "1")
	var plain bytes.Buffer
	if err := codeguard.WriteReport(&plain, report, "text"); err != nil {
		t.Fatalf("write plain text: %v", err)
	}
	assertPlainTextReportFormatting(t, &plain)

	var sarif bytes.Buffer
	if err := codeguard.WriteReport(&sarif, report, "sarif"); err != nil {
		t.Fatalf("write sarif: %v", err)
	}
	if !strings.Contains(sarif.String(), `"version": "2.1.0"`) {
		t.Fatalf("expected SARIF payload, got: %s", sarif.String())
	}

	var github bytes.Buffer
	if err := codeguard.WriteReport(&github, report, "github"); err != nil {
		t.Fatalf("write github: %v", err)
	}
	if !strings.Contains(github.String(), "::warning file=tests/checks/test_helpers_test.go,line=58,col=1::") {
		t.Fatalf("expected GitHub annotation, got: %s", github.String())
	}

	var githubComment bytes.Buffer
	if err := codeguard.WriteReport(&githubComment, report, "github-comment"); err != nil {
		t.Fatalf("write github comment: %v", err)
	}
	if !strings.Contains(githubComment.String(), "## CodeGuard Fix Suggestions") {
		t.Fatalf("expected GitHub comment heading, got: %s", githubComment.String())
	}
	if !strings.Contains(githubComment.String(), "Fix: Reduce branching in the function or refactor logic into smaller units.") {
		t.Fatalf("expected concrete fix guidance, got: %s", githubComment.String())
	}
}

func TestWriteReportUsesSameGroupedLayoutAcrossSections(t *testing.T) {
	var out bytes.Buffer
	t.Setenv("NO_COLOR", "1")
	if err := codeguard.WriteReport(&out, groupedLayoutReport(), "text"); err != nil {
		t.Fatalf("write text: %v", err)
	}

	rendered := out.String()
	for _, want := range groupedLayoutFragments() {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected grouped layout fragment %q, got:\n%s", want, rendered)
		}
	}
}

func formatReport() codeguard.Report {
	return codeguard.Report{
		Name: "format-test",
		Sections: []codeguard.SectionResult{{
			ID:     "quality",
			Name:   "Code Quality",
			Status: codeguard.StatusWarn,
			Findings: []codeguard.Finding{
				{
					RuleID:      "quality.cyclomatic-complexity",
					Level:       "warn",
					Title:       "Cyclomatic complexity",
					Message:     "function assertTextReportFormatting has cyclomatic complexity 11; max is 10",
					Why:         "function assertTextReportFormatting has cyclomatic complexity 11; max is 10",
					HowToFix:    "Reduce branching in the function or refactor logic into smaller units.",
					Path:        "tests/checks/test_helpers_test.go",
					Line:        58,
					Column:      1,
					Fingerprint: "abc123",
				},
				{
					RuleID:      "quality.dependency-direction",
					Level:       "warn",
					Title:       "Dependency direction",
					Message:     "non-CLI package imports internal implementation detail",
					Why:         "non-CLI package imports internal implementation detail",
					HowToFix:    "Move shared logic into reusable packages and keep internal or CLI details out of library code.",
					Path:        "pkg/codeguard/sdk_types_state.go",
					Line:        3,
					Column:      1,
					Fingerprint: "def456",
				},
			},
		}},
		Summary: codeguard.ReportSummary{
			WarnedSections: 1,
			TotalFindings:  2,
		},
	}
}

func groupedLayoutReport() codeguard.Report {
	return codeguard.Report{
		Name: "layout-test",
		Sections: []codeguard.SectionResult{
			groupedSection(sectionFixture{"quality", "Code Quality", "quality.cyclomatic-complexity", "Cyclomatic complexity", "quality why", "quality fix", "quality.go", 10, "quality-1"}),
			groupedSection(sectionFixture{"design", "Design Patterns", "design.max-methods-per-type", "Methods per type", "design why", "design fix", "design.go", 20, "design-1"}),
			groupedSection(sectionFixture{"security", "Security", "security.shell-execution", "Shell execution review", "security why", "security fix", "security.go", 30, "security-1"}),
			groupedSection(sectionFixture{"prompts", "AI Prompts", "prompts.unsafe-instructions", "Unsafe instructions", "prompts why", "prompts fix", "prompts.md", 40, "prompts-1"}),
			groupedSection(sectionFixture{"ci", "CI/CD", "ci.workflow-content", "Workflow content", "ci why", "ci fix", ".github/workflows/ci.yml", 50, "ci-1"}),
		},
		Summary: codeguard.ReportSummary{
			WarnedSections: 5,
			TotalFindings:  5,
		},
	}
}

type sectionFixture struct {
	id          string
	name        string
	ruleID      string
	title       string
	why         string
	fix         string
	path        string
	line        int
	fingerprint string
}

func groupedSection(fixture sectionFixture) codeguard.SectionResult {
	return codeguard.SectionResult{
		ID:     fixture.id,
		Name:   fixture.name,
		Status: codeguard.StatusWarn,
		Findings: []codeguard.Finding{{
			RuleID:      fixture.ruleID,
			Level:       "warn",
			Title:       fixture.title,
			Message:     fixture.why,
			Why:         fixture.why,
			HowToFix:    fixture.fix,
			Path:        fixture.path,
			Line:        fixture.line,
			Fingerprint: fixture.fingerprint,
		}},
	}
}

func groupedLayoutFragments() []string {
	return []string{
		"[⚠️ WARN] Code Quality",
		"  Cyclomatic complexity\n  1. at: quality.go:10",
		"[⚠️ WARN] Design Patterns",
		"  Methods per type\n  1. at: design.go:20",
		"[⚠️ WARN] Security",
		"  Shell execution review\n  1. at: security.go:30",
		"[⚠️ WARN] AI Prompts",
		"  Unsafe instructions\n  1. at: prompts.md:40",
		"[⚠️ WARN] CI/CD",
		"  Workflow content\n  1. at: .github/workflows/ci.yml:50",
	}
}
