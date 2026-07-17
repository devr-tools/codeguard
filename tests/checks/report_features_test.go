package checks_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestWriteReportSupportsSARIFAndGitHub(t *testing.T) {
	report := formatReport()
	if report.Name != "format-test" {
		t.Fatalf("report name = %q, want %q", report.Name, "format-test")
	}

	testColoredTextReport(t, report)
	testPlainTextReport(t, report)
	testJSONReport(t, report)
	testSARIFReport(t, report)
	testGitHubReport(t, report)
	testGitHubCommentReport(t, report)
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
