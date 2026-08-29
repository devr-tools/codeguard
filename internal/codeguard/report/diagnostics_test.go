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

func TestSARIFStructuralUnresolvedDiagnosticsAreNotFindingFingerprints(t *testing.T) {
	t.Parallel()
	report := core.Report{Sections: []core.SectionResult{{ID: "quality", Diagnostics: []core.Diagnostic{
		{ID: "quality.structural-unresolved-symbols", Level: "info", Kind: "analysis", Message: "unresolved mutation symbols", Metadata: map[string]string{"language": "c++", "count": "1"}},
		{ID: "quality.structural-unresolved-symbols", Level: "info", Kind: "analysis", Message: "unresolved mutation symbols", Metadata: map[string]string{"language": "go", "count": "2"}},
	}}}}
	var output bytes.Buffer
	if err := writeSARIF(&output, report); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Count(text, `"ruleId": "quality.structural-unresolved-symbols"`) != 2 {
		t.Fatalf("SARIF must retain separate per-language diagnostics:\n%s", text)
	}
	for _, want := range []string{`"language": "c++"`, `"count": "1"`, `"language": "go"`, `"count": "2"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("SARIF missing %s:\n%s", want, text)
		}
	}
	if strings.Contains(text, `"partialFingerprints"`) {
		t.Fatalf("diagnostics must not become baselinable finding fingerprints:\n%s", text)
	}
}
