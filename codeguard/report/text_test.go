package report

import (
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func TestTextIncludesEmojiAndColorByDefault(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")

	output := Text(sampleReport())

	assertHasPrefix(t, output, "⠀⠀⠀⠀⠀⢀⣠⠤⠶⠲⠦⢤⣀", "expected codeguard banner at top")
	assertContainsAll(t, output,
		"CodeGuard Report",
		"✓ PASS",
		"\x1b[",
		"\x1b[38;2;19;156;254m",
	)
	assertNotContainsAny(t, output, "❌", "⚠️", "⏭️", "🛡️", "ℹ️")
}

func TestTextDisablesColorWithNoColor(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "1")

	output := Text(sampleReport())

	assertNotContainsAny(t, output, "\x1b[", "⠀⠀⠀⠀⠀⢀⣠⠤⠶⠲⠦⢤⣀")
	assertContainsAll(t, output, "✓ PASS")
}

func assertHasPrefix(t *testing.T, text string, prefix string, message string) {
	t.Helper()
	if strings.HasPrefix(text, prefix) {
		return
	}
	t.Fatalf("%s, got:\n%s", message, text)
}

func assertContainsAll(t *testing.T, text string, values ...string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(text, value) {
			continue
		}
		t.Fatalf("expected output to contain %q, got:\n%s", value, text)
	}
}

func assertNotContainsAny(t *testing.T, text string, values ...string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(text, value) {
			continue
		}
		t.Fatalf("expected output to omit %q, got:\n%s", value, text)
	}
}

func sampleReport() core.Report {
	return core.Report{
		Name:        "sample",
		GeneratedAt: time.Date(2026, 6, 10, 20, 0, 0, 0, time.UTC),
		ScanMode:    core.ScanModeFull,
		Sections: []core.SectionResult{
			{
				Name:   "Code Quality",
				Status: core.StatusPass,
				Findings: []core.Finding{
					{Path: "main.go", Message: "looks good", Severity: core.SeverityInfo},
				},
			},
			{
				Name:   "CI/CD",
				Status: core.StatusFail,
				Findings: []core.Finding{
					{Path: ".github/workflows/ci.yml", Message: "missing marker", Severity: core.SeverityError},
				},
			},
		},
		Summary: core.Summary{
			PassedSections: 1,
			FailedSections: 1,
		},
	}
}
