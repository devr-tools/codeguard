package checks_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/version"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func testColoredTextReport(t *testing.T, report codeguard.Report) {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	output := writeFormatReport(t, report, "text")
	assertTextReportFormatting(t, &output)
	assertReportVersion(t, "text", output.String(), "CodeGuard version: "+version.Number)
}

func testPlainTextReport(t *testing.T, report codeguard.Report) {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	output := writeFormatReport(t, report, "text")
	assertPlainTextReportFormatting(t, &output)
}

func testJSONReport(t *testing.T, report codeguard.Report) {
	t.Helper()
	output := writeFormatReport(t, report, "json")
	var jsonPayload struct {
		CodeGuardVersion string `json:"codeguard_version"`
	}
	if err := json.Unmarshal(output.Bytes(), &jsonPayload); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if jsonPayload.CodeGuardVersion != version.Number {
		t.Fatalf("json codeguard_version = %q, want %q", jsonPayload.CodeGuardVersion, version.Number)
	}
}

func testSARIFReport(t *testing.T, report codeguard.Report) {
	t.Helper()
	output := writeFormatReport(t, report, "sarif")
	if !strings.Contains(output.String(), `"version": "2.1.0"`) {
		t.Fatalf("expected SARIF payload, got: %s", output.String())
	}
	assertReportVersion(t, "sarif", output.String(), `"version": "`+version.Number+`"`)
}

func testGitHubReport(t *testing.T, report codeguard.Report) {
	t.Helper()
	output := writeFormatReport(t, report, "github")
	if !strings.Contains(output.String(), "::warning file=tests/checks/test_helpers_test.go,line=58,col=1::") {
		t.Fatalf("expected GitHub annotation, got: %s", output.String())
	}
	assertReportVersion(t, "github", output.String(), "::notice title=CodeGuard::version "+version.Number)
}

func testGitHubCommentReport(t *testing.T, report codeguard.Report) {
	t.Helper()
	output := writeFormatReport(t, report, "github-comment")
	if !strings.Contains(output.String(), "## CodeGuard Fix Suggestions") {
		t.Fatalf("expected GitHub comment heading, got: %s", output.String())
	}
	assertReportVersion(t, "github-comment", output.String(), "CodeGuard version "+version.Number)
	if !strings.Contains(output.String(), "Fix: Reduce branching in the function or refactor logic into smaller units.") {
		t.Fatalf("expected concrete fix guidance, got: %s", output.String())
	}
}

func writeFormatReport(t *testing.T, report codeguard.Report, format string) bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	if err := codeguard.WriteReport(&out, report, format); err != nil {
		t.Fatalf("write %s: %v", format, err)
	}
	return out
}

func assertReportVersion(t *testing.T, format string, output string, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Fatalf("expected %s output to include version %q, got: %s", format, want, output)
	}
}
