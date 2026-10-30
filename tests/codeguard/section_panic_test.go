package codeguard_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/report"
	runnerchecks "github.com/devr-tools/codeguard/internal/codeguard/runner/checks"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// panickingSecuritySection stands in for a gate-critical section that crashes
// mid-scan. It is a named function so the stack excerpt can be asserted to
// point at it.
func panickingSecuritySection() core.SectionResult {
	var targets []core.TargetConfig
	_ = targets[3] // index out of range, like a parser tripping on untrusted input
	return core.SectionResult{ID: "security", Name: "Security", Status: core.StatusPass}
}

func TestSafeRunTurnsSectionPanicIntoFailingDiagnostic(t *testing.T) {
	result, panicked := runnerchecks.SafeRun("security", "Security", panickingSecuritySection)
	if !panicked {
		t.Fatal("SafeRun reported no panic for a panicking section")
	}
	if result.ID != "security" || result.Name != "Security" {
		t.Fatalf("section identity = %q/%q, want security/Security", result.ID, result.Name)
	}
	if result.Status != core.StatusFail {
		t.Fatalf("status = %q, want %q: a crashed section checked nothing and must not pass or merely warn", result.Status, core.StatusFail)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("findings = %+v, want none: a crash is a scanner diagnostic, never a baselinable finding", result.Findings)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want exactly one", result.Diagnostics)
	}
	diagnostic := result.Diagnostics[0]
	if runnerchecks.SectionPanicDiagnosticID != "scan.section.panic" {
		t.Fatalf("SectionPanicDiagnosticID = %q; it is a stable id consumers match on", runnerchecks.SectionPanicDiagnosticID)
	}
	if diagnostic.ID != runnerchecks.SectionPanicDiagnosticID {
		t.Fatalf("diagnostic id = %q, want scan.section.panic", diagnostic.ID)
	}
	if diagnostic.Level != "fail" {
		t.Fatalf("diagnostic level = %q, want the canonical %q", diagnostic.Level, "fail")
	}
	if !diagnostic.Operational {
		t.Fatal("diagnostic must be operational so section status and SARIF executionSuccessful reflect it")
	}
	if !strings.Contains(diagnostic.Message, "Security check crashed") {
		t.Fatalf("message = %q, want it to name the crashed section", diagnostic.Message)
	}
	if strings.Contains(diagnostic.Message, "index out of range") || strings.Contains(diagnostic.Message, "goroutine") {
		t.Fatalf("message = %q leaks the raw panic; it belongs in metadata/evidence", diagnostic.Message)
	}
	if !strings.Contains(diagnostic.Metadata["panic"], "index out of range") {
		t.Fatalf("metadata panic = %q, want the recovered panic value kept for debugging", diagnostic.Metadata["panic"])
	}
	if diagnostic.Metadata["section"] != "security" {
		t.Fatalf("metadata section = %q, want security", diagnostic.Metadata["section"])
	}
	if len(diagnostic.Evidence) == 0 || len(diagnostic.Evidence) > 8 {
		t.Fatalf("evidence has %d frames, want a short non-empty stack excerpt", len(diagnostic.Evidence))
	}
	if !strings.Contains(diagnostic.Evidence[0], "panickingSecuritySection") {
		t.Fatalf("first evidence frame = %q, want the panicking function, not the recovery machinery", diagnostic.Evidence[0])
	}
}

func TestSafeRunPassesThroughACompletedSection(t *testing.T) {
	want := core.SectionResult{ID: "quality", Name: "Quality", Status: core.StatusWarn}
	got, panicked := runnerchecks.SafeRun("quality", "Quality", func() core.SectionResult { return want })
	if panicked {
		t.Fatal("SafeRun reported a panic for a section that returned normally")
	}
	if got.ID != want.ID || got.Status != want.Status || len(got.Diagnostics) != 0 {
		t.Fatalf("SafeRun altered a completed section: got %+v, want %+v", got, want)
	}
}

// TestSectionPanicFailsTheRun pins the gating outcome: the CLI exits non-zero
// exactly when Summary.FailedSections > 0, so a crashed section among clean
// ones must count as failed rather than collapsing to a passing warn.
func TestSectionPanicFailsTheRun(t *testing.T) {
	clean, _ := runnerchecks.SafeRun("quality", "Quality", func() core.SectionResult {
		return core.SectionResult{ID: "quality", Name: "Quality", Status: core.StatusPass}
	})
	crashed, _ := runnerchecks.SafeRun("security", "Security", panickingSecuritySection)
	sections := []core.SectionResult{clean, crashed}

	summary := runnersupport.SummarizeSections(sections)
	if summary.FailedSections != 1 || summary.PassedSections != 1 || summary.WarnedSections != 0 {
		t.Fatalf("summary = %+v, want 1 failed and 1 passed section", summary)
	}

	rep := core.Report{Name: "panic", Sections: sections, Summary: summary}
	var text bytes.Buffer
	if err := report.Write(&text, rep, "text"); err != nil {
		t.Fatalf("write text report: %v", err)
	}
	if !strings.Contains(text.String(), "scan.section.panic: Security check crashed") {
		t.Fatalf("text report does not surface the crash diagnostic:\n%s", text.String())
	}

	var sarif bytes.Buffer
	if err := report.Write(&sarif, rep, "sarif"); err != nil {
		t.Fatalf("write sarif report: %v", err)
	}
	var doc struct {
		Runs []struct {
			Invocations []struct {
				ExecutionSuccessful bool `json:"executionSuccessful"`
			} `json:"invocations"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(sarif.Bytes(), &doc); err != nil {
		t.Fatalf("decode sarif: %v", err)
	}
	if len(doc.Runs) != 1 || len(doc.Runs[0].Invocations) != 1 || doc.Runs[0].Invocations[0].ExecutionSuccessful {
		t.Fatalf("sarif invocation must report executionSuccessful=false after a section crash: %s", sarif.String())
	}
}
