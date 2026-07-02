package security_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/report"
	"github.com/devr-tools/codeguard/internal/version"
)

// SARIF output must carry the tool version and an invocation record so a CI
// consumer can attribute a results file to a specific codeguard run (SOC 3
// monitoring / audit trail).
func TestSARIFCarriesVersionAndInvocation(t *testing.T) {
	rep := core.Report{
		Profile:     "default",
		GeneratedAt: "2026-07-02T12:00:00Z",
		Sections: []core.SectionResult{{
			Name: "Security",
			Findings: []core.Finding{{
				RuleID:  "security.taint.go",
				Level:   "fail",
				Message: "tainted input reaches exec.Command",
				Path:    "main.go",
				Line:    10,
			}},
		}},
	}

	var buf bytes.Buffer
	if err := report.Write(&buf, rep, "sarif"); err != nil {
		t.Fatalf("write sarif: %v", err)
	}

	var doc struct {
		Runs []struct {
			Tool struct {
				Driver struct {
					Name           string `json:"name"`
					Version        string `json:"version"`
					InformationURI string `json:"informationUri"`
				} `json:"driver"`
			} `json:"tool"`
			Invocations []struct {
				ExecutionSuccessful bool   `json:"executionSuccessful"`
				EndTimeUtc          string `json:"endTimeUtc"`
			} `json:"invocations"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("parse sarif: %v\n%s", err, buf.String())
	}
	if len(doc.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(doc.Runs))
	}
	d := doc.Runs[0].Tool.Driver
	if d.Name != "codeguard" {
		t.Errorf("driver.name = %q, want codeguard", d.Name)
	}
	if d.Version != version.Number {
		t.Errorf("driver.version = %q, want %q", d.Version, version.Number)
	}
	if d.InformationURI == "" {
		t.Error("driver.informationUri must be set")
	}
	inv := doc.Runs[0].Invocations
	if len(inv) != 1 {
		t.Fatalf("expected 1 invocation, got %d", len(inv))
	}
	if !inv[0].ExecutionSuccessful {
		t.Error("invocation.executionSuccessful must be true")
	}
	if inv[0].EndTimeUtc != rep.GeneratedAt {
		t.Errorf("invocation.endTimeUtc = %q, want %q", inv[0].EndTimeUtc, rep.GeneratedAt)
	}
}
