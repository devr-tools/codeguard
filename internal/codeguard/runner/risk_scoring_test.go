package runner

import (
	"reflect"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func TestAddRiskArtifactsRanksFilesDeterministicallyAndExplainsSignals(t *testing.T) {
	enabled := true
	sc := runnersupport.Context{
		Opts: core.ScanOptions{Mode: core.ScanModeDiff},
		Cfg: core.Config{Checks: core.CheckConfig{QualityRules: core.QualityRulesConfig{RiskScoring: core.RiskScoringConfig{
			Enabled: &enabled, MaxHotspots: 1, ChangedFileWeight: 5, FailFindingWeight: 30, WarnFindingWeight: 15,
			SecurityWeight: 10, SupplyChainWeight: 10, CoverageGapWeight: 15, AIProvenanceWeight: 15, AISignalWeight: 10, SlopScoreDivisor: 10,
		}}}},
		Diff:      map[string]runnersupport.LineRanges{"a.go": {}, "b.go": {}},
		Artifacts: runnersupport.NewArtifactStore(),
	}
	sc.Artifacts.Put(core.Artifact{ID: "slop", Kind: "slop_score", Target: ".", SlopScore: &core.SlopScoreArtifact{Score: 25}})
	sc.Artifacts.Put(core.Artifact{ID: "change", Kind: "change_risk", Target: ".", ChangeRisk: &core.ChangeRiskArtifact{ProvenanceActive: true}})
	sections := []core.SectionResult{{ID: "security", Findings: []core.Finding{
		{RuleID: "security.go.sql-injection", Section: "security", Level: "fail", Path: "b.go"},
		{RuleID: "quality.coverage-delta", Section: "quality", Level: "warn", Path: "a.go"},
	}}}

	addRiskArtifacts(sc, sections)
	fileRisk, hotspots := riskArtifacts(t, sc.Artifacts.List())
	if len(fileRisk.Files) != 2 {
		t.Fatalf("file risk entries = %#v, want 2", fileRisk.Files)
	}
	if fileRisk.Files[0].Path != "b.go" || fileRisk.Files[0].Rank != 1 || fileRisk.Files[0].Score != 62 {
		t.Fatalf("top risk = %#v, want b.go ranked first with score 62", fileRisk.Files[0])
	}
	if fileRisk.Files[1].Path != "a.go" || fileRisk.Files[1].Rank != 2 || fileRisk.Files[1].Score != 52 {
		t.Fatalf("second risk = %#v, want a.go ranked second with score 52", fileRisk.Files[1])
	}
	labels := make([]string, 0, len(fileRisk.Files[0].Components))
	for _, component := range fileRisk.Files[0].Components {
		labels = append(labels, component.Label)
	}
	if want := []string{"ai_provenance", "changed_file", "fail_finding", "security_finding", "slop_score"}; !reflect.DeepEqual(labels, want) {
		t.Fatalf("component labels = %#v, want %#v", labels, want)
	}
	if len(hotspots.Hotspots) != 1 || hotspots.Hotspots[0].Path != "b.go" {
		t.Fatalf("hotspots = %#v, want only b.go", hotspots.Hotspots)
	}
}

func TestAddRiskArtifactsSkipsFullScans(t *testing.T) {
	enabled := true
	sc := runnersupport.Context{
		Opts:      core.ScanOptions{Mode: core.ScanModeFull},
		Cfg:       core.Config{Checks: core.CheckConfig{QualityRules: core.QualityRulesConfig{RiskScoring: core.RiskScoringConfig{Enabled: &enabled}}}},
		Diff:      map[string]runnersupport.LineRanges{"a.go": {}},
		Artifacts: runnersupport.NewArtifactStore(),
	}
	addRiskArtifacts(sc, nil)
	if got := sc.Artifacts.List(); len(got) != 0 {
		t.Fatalf("artifacts = %#v, want none for full scan", got)
	}
}

func riskArtifacts(t *testing.T, artifacts []core.Artifact) (*core.FileRiskArtifact, *core.PRHotspotsArtifact) {
	t.Helper()
	var fileRisk *core.FileRiskArtifact
	var hotspots *core.PRHotspotsArtifact
	for _, artifact := range artifacts {
		if artifact.FileRisk != nil {
			fileRisk = artifact.FileRisk
		}
		if artifact.PRHotspots != nil {
			hotspots = artifact.PRHotspots
		}
	}
	if fileRisk == nil || hotspots == nil {
		t.Fatalf("artifacts = %#v, want file_risk and pr_hotspots", artifacts)
	}
	return fileRisk, hotspots
}
