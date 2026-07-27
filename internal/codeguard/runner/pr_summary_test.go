package runner

import (
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
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
