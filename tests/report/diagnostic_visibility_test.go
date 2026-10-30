package report_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func diagnosticReport(diagnostics ...core.Diagnostic) codeguard.Report {
	return codeguard.Report{
		Sections: []codeguard.SectionResult{{
			ID:          "quality",
			Name:        "Code Quality",
			Status:      codeguard.StatusWarn,
			Diagnostics: diagnostics,
		}},
	}
}

func renderDiagnosticText(t *testing.T, report codeguard.Report) string {
	t.Helper()
	var buf bytes.Buffer
	if err := codeguard.WriteReport(&buf, report, "text"); err != nil {
		t.Fatalf("write text report: %v", err)
	}
	return buf.String()
}

// A diagnostic that says the scan did not cover everything has to reach the
// operator in the format they actually read. The text report used to drop every
// diagnostic, so "the walk stopped early" or "govulncheck could not run" was
// visible only in JSON or SARIF output.
func TestTextReportShowsDiagnosticsThatSayTheScanWasIncomplete(t *testing.T) {
	output := renderDiagnosticText(t, diagnosticReport(core.Diagnostic{
		ID:          "scan.file-limit",
		Level:       "warn",
		Kind:        "analysis",
		Message:     "repository walk stopped at the 100000 file limit",
		Operational: true,
	}))

	if !strings.Contains(output, "scan.file-limit") {
		t.Fatalf("text report omits the diagnostic id:\n%s", output)
	}
	if !strings.Contains(output, "repository walk stopped at the 100000 file limit") {
		t.Fatalf("text report omits the diagnostic message:\n%s", output)
	}
}

func TestTextReportShowsFailingOperationalDiagnostics(t *testing.T) {
	output := renderDiagnosticText(t, diagnosticReport(core.Diagnostic{
		ID:          "scan.govulncheck.module-unavailable",
		Level:       "fail",
		Kind:        "infrastructure",
		Message:     "no Go module is available for govulncheck",
		Operational: true,
	}))

	if !strings.Contains(output, "scan.govulncheck.module-unavailable") {
		t.Fatalf("text report omits a failing operational diagnostic:\n%s", output)
	}
}

// Informational diagnostics stay out of the default report. They are routine
// per-finding classifications and status lines, and a scan emits enough of them
// to bury the findings they sit next to.
func TestTextReportKeepsInformationalDiagnosticsOutOfTheDefaultOutput(t *testing.T) {
	output := renderDiagnosticText(t, diagnosticReport(core.Diagnostic{
		ID:      "scan.corpus-budget",
		Level:   "info",
		Kind:    "analysis",
		Message: "repository analysis exceeded a bounded corpus cache budget",
	}))

	if strings.Contains(output, "scan.corpus-budget") {
		t.Fatalf("text report should not render informational diagnostics:\n%s", output)
	}
}
