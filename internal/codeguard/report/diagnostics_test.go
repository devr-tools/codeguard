package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestSARIFIncludesDiagnosticsAndMarksOperationalFailure(t *testing.T) {
	t.Parallel()
	report := core.Report{Sections: []core.SectionResult{{ID: "security", Diagnostics: []core.Diagnostic{{
		ID: "scan.govulncheck.module", Level: "fail", Kind: "infrastructure", Message: "module timed out", Operational: true,
		Evidence: []string{"status:timed_out"}, Metadata: map[string]string{"module": "example.com/api"},
	}}}}}
	var output bytes.Buffer
	if err := writeSARIF(&output, report); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{`"ruleId": "scan.govulncheck.module"`, `"executionSuccessful": false`, `"kind": "infrastructure"`, `"status:timed_out"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("SARIF missing %s:\n%s", want, text)
		}
	}
}
