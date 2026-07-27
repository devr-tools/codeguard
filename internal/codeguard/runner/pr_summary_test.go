package runner

import (
	"bytes"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/report"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func TestAddPRSummaryArtifactScoresProductionRisk(t *testing.T) {
	enabled := true
	sc := runnersupport.Context{
		Opts: core.ScanOptions{Mode: core.ScanModeDiff},
		Cfg: core.Config{Checks: core.CheckConfig{ProductionRisk: core.ProductionRiskConfig{
			Enabled: &enabled, WarnThreshold: 35, FailThreshold: 70,
			ReliabilityWeight: 12, DataWeight: 15, FailWeight: 25, WarnWeight: 10,
		}}},
		Artifacts: runnersupport.NewArtifactStore(),
	}
	sections := []core.SectionResult{
		{Name: "Reliability", Findings: []core.Finding{
			{RuleID: "reliability.missing-timeout", Level: "fail", Path: "client.go"},
			{RuleID: "reliability.unbounded-work", Level: "warn", Path: "worker.go"},
		}},
		{Name: "Data Correctness", Findings: []core.Finding{
			{RuleID: "data.missing-outbox-strategy", Level: "fail", Path: "service.go"},
		}},
	}

	addPRSummaryArtifact(sc, sections)

	artifact := requirePRSummaryArtifact(t, sc.Artifacts.List())
	if artifact.ProductionRisk == nil {
		t.Fatal("expected production risk metric")
	}
	if artifact.ProductionRisk.Score != 99 {
		t.Fatalf("production risk score = %d, want 99", artifact.ProductionRisk.Score)
	}
	if artifact.ProductionRisk.Level != "fail" {
		t.Fatalf("production risk level = %q, want fail", artifact.ProductionRisk.Level)
	}
	if len(artifact.ProductionRisk.Components) != 2 {
		t.Fatalf("components = %#v, want reliability and data", artifact.ProductionRisk.Components)
	}
	if artifact.ProductionRisk.Components[0].Label != "reliability" {
		t.Fatalf("first component = %#v, want reliability first by contribution", artifact.ProductionRisk.Components[0])
	}
}

func TestAddPRSummaryArtifactAddsChangeSafetyMetrics(t *testing.T) {
	enabled := true
	sc := runnersupport.Context{
		Opts: core.ScanOptions{Mode: core.ScanModeDiff},
		Cfg: core.Config{Checks: core.CheckConfig{ProductionRisk: core.ProductionRiskConfig{
			Enabled: &enabled, WarnThreshold: 35, FailThreshold: 70,
			ReliabilityWeight: 12, DataWeight: 15, FailWeight: 25, WarnWeight: 10,
		}}},
		Artifacts: runnersupport.NewArtifactStore(),
	}
	sections := []core.SectionResult{
		{Name: "Change Safety", Findings: []core.Finding{
			{RuleID: "change.oversized-diff", Level: "warn", Confidence: "high", Path: "b.go"},
			{RuleID: "testing.behavior-change-without-test", Level: "fail", Confidence: "high", Path: "a.go"},
			{RuleID: "change.mixed-refactor-and-behavior", Level: "warn", Path: "c.go"},
		}},
		{Name: "Maintainability", Findings: []core.Finding{
			{RuleID: "maintainability.public-surface-growth", Level: "warn", Path: "api.go"},
			{RuleID: "function.excessive-parameters", Level: "warn", Confidence: "low", Path: "service.go"},
			{RuleID: "defensive.unchecked-type-assertion", Level: "fail", Path: "types.go"},
		}},
		{Name: "Refactors", Findings: []core.Finding{
			{RuleID: "refactor.behavior-change-detected", Level: "fail", Confidence: "high", Path: "refactor.go"},
		}},
		{Name: "Reliability", Findings: []core.Finding{
			{RuleID: "reliability.missing-timeout", Level: "fail", Path: "client.go"},
		}},
	}

	addPRSummaryArtifact(sc, sections)

	artifact := requirePRSummaryArtifact(t, sc.Artifacts.List())
	if artifact.ProductionRisk == nil {
		t.Fatal("expected production risk metric to be preserved")
	}
	if artifact.ChangeSafety == nil {
		t.Fatal("expected change safety metric")
	}
	if artifact.ChangeSafety.Score != 81 {
		t.Fatalf("change safety score = %d, want 81", artifact.ChangeSafety.Score)
	}
	if artifact.ChangeSafety.Level != "fail" {
		t.Fatalf("change safety level = %q, want fail", artifact.ChangeSafety.Level)
	}
	if labels := componentLabels(artifact.ChangeSafety.Components); labels != "change_scope,test_evidence" {
		t.Fatalf("change safety component labels = %q, want deterministic contribution order", labels)
	}
	if artifact.MaintainabilityDelta == nil {
		t.Fatal("expected maintainability delta metric")
	}
	if labels := componentLabels(artifact.MaintainabilityDelta.Components); labels != "defensive_programming,maintainability,code_quality" {
		t.Fatalf("maintainability component labels = %q, want deterministic contribution order", labels)
	}
	if artifact.RefactorConfidence == nil {
		t.Fatal("expected refactor confidence metric")
	}
	if artifact.RefactorConfidence.Score != 58 {
		t.Fatalf("refactor confidence score = %d, want 58", artifact.RefactorConfidence.Score)
	}
	if labels := componentLabels(artifact.RefactorConfidence.Components); labels != "behavior_preservation,mixed_refactor" {
		t.Fatalf("refactor confidence component labels = %q, want deterministic contribution order", labels)
	}
}

func TestAddPRSummaryArtifactPublishesChangeMetricsWithoutProductionRisk(t *testing.T) {
	sc := runnersupport.Context{
		Opts:      core.ScanOptions{Mode: core.ScanModeDiff},
		Cfg:       core.Config{Checks: core.CheckConfig{}},
		Artifacts: runnersupport.NewArtifactStore(),
	}

	addPRSummaryArtifact(sc, []core.SectionResult{{Findings: []core.Finding{{RuleID: "change.oversized-diff", Level: "warn"}}}})

	artifact := requirePRSummaryArtifact(t, sc.Artifacts.List())
	if artifact.ProductionRisk != nil {
		t.Fatalf("production risk metric = %#v, want nil when disabled", artifact.ProductionRisk)
	}
	if artifact.ChangeSafety == nil {
		t.Fatal("expected change safety metric")
	}
}

func TestAddPRSummaryArtifactSkipsFullScans(t *testing.T) {
	enabled := true
	sc := runnersupport.Context{
		Opts:      core.ScanOptions{Mode: core.ScanModeFull},
		Cfg:       core.Config{Checks: core.CheckConfig{ProductionRisk: core.ProductionRiskConfig{Enabled: &enabled}}},
		Artifacts: runnersupport.NewArtifactStore(),
	}

	addPRSummaryArtifact(sc, []core.SectionResult{{Findings: []core.Finding{{RuleID: "reliability.missing-timeout", Level: "fail"}}}})

	if got := sc.Artifacts.List(); len(got) != 0 {
		t.Fatalf("artifacts = %#v, want none for full scan", got)
	}
}

func TestPRSummaryMetricsAreArtifactOnlyForGitHubAnnotations(t *testing.T) {
	reportData := core.Report{
		Name: "sample",
		Artifacts: []core.Artifact{{
			ID:   "pr_summary",
			Kind: core.ReportArtifactKindPRSummary,
			PRSummary: &core.PRSummaryArtifact{
				ChangeSafety: &core.PRSummaryMetric{
					Score: 18,
					Level: "pass",
					Components: []core.PRSummaryComponent{{
						Label:        "change_scope",
						Weight:       18,
						Count:        1,
						Contribution: 18,
					}},
				},
			},
		}},
		Sections: []core.SectionResult{{Name: "Change Safety"}},
	}

	var out bytes.Buffer
	if err := report.Write(&out, reportData, "github"); err != nil {
		t.Fatalf("write github report: %v", err)
	}
	if bytes.Contains(out.Bytes(), []byte("change_safety")) || bytes.Contains(out.Bytes(), []byte("pr_summary")) {
		t.Fatalf("github annotations included metrics artifact:\n%s", out.String())
	}
}

func requirePRSummaryArtifact(t *testing.T, artifacts []core.Artifact) *core.PRSummaryArtifact {
	t.Helper()
	for _, artifact := range artifacts {
		if artifact.Kind == core.ReportArtifactKindPRSummary {
			if artifact.PRSummary == nil {
				t.Fatal("pr_summary artifact missing payload")
			}
			return artifact.PRSummary
		}
	}
	t.Fatalf("pr_summary artifact not found: %#v", artifacts)
	return nil
}

func componentLabels(components []core.PRSummaryComponent) string {
	labels := make([]byte, 0, len(components)*16)
	for i, component := range components {
		if i > 0 {
			labels = append(labels, ',')
		}
		labels = append(labels, component.Label...)
	}
	return string(labels)
}
