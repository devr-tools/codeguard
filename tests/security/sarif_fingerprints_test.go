package security_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/report"
)

// SARIF results must carry partialFingerprints so GitHub code scanning can
// deduplicate alerts across commits: the context fingerprint survives line
// shifts, the legacy fingerprint preserves continuity with older uploads.
func TestSARIFCarriesPartialFingerprints(t *testing.T) {
	rep := core.Report{
		GeneratedAt: "2026-07-02T12:00:00Z",
		Sections: []core.SectionResult{{
			Name: "Security",
			Findings: []core.Finding{{
				RuleID:             "security.taint.go",
				Level:              "fail",
				Message:            "tainted input reaches exec.Command",
				Path:               "main.go",
				Line:               10,
				Fingerprint:        "legacy-fp",
				ContextFingerprint: "context-fp",
			}},
		}},
	}

	var buf bytes.Buffer
	if err := report.Write(&buf, rep, "sarif"); err != nil {
		t.Fatalf("write sarif: %v", err)
	}

	var doc struct {
		Runs []struct {
			Results []struct {
				PartialFingerprints map[string]string `json:"partialFingerprints"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("parse sarif: %v\n%s", err, buf.String())
	}
	if len(doc.Runs) != 1 || len(doc.Runs[0].Results) != 1 {
		t.Fatalf("expected 1 run with 1 result, got %s", buf.String())
	}
	got := doc.Runs[0].Results[0].PartialFingerprints
	want := map[string]string{
		"codeguardContext/v1": "context-fp",
		"codeguardLegacy/v1":  "legacy-fp",
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("partialFingerprints[%q] = %q, want %q", key, got[key], value)
		}
	}
	if len(got) != len(want) {
		t.Errorf("partialFingerprints = %v, want exactly %v", got, want)
	}
}
