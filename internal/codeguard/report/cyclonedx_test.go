package report

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestWriteCycloneDXSerializesSupplyChainArtifactsDeterministically(t *testing.T) {
	report := core.Report{Artifacts: []core.Artifact{{
		Kind: "supply_chain",
		SupplyChain: &core.SupplyChainArtifact{Manifests: []core.SupplyChainManifest{{
			Ecosystem: "npm", Path: "web/package.json", Dependencies: []core.SupplyChainDependency{
				{Name: "zod", Requirement: "^3.0.0", Scope: "runtime"},
				{Name: "@scope/lib", Version: "1.2.3", License: "MIT", Groups: []string{"test", "dev"}, Indirect: true},
			},
		}}},
	}}}

	var first, second bytes.Buffer
	if err := Write(&first, report, "cyclonedx"); err != nil {
		t.Fatalf("write CycloneDX report: %v", err)
	}
	if err := Write(&second, report, "cyclonedx-json"); err != nil {
		t.Fatalf("write CycloneDX alias: %v", err)
	}
	if first.String() != second.String() {
		t.Fatalf("CycloneDX output was not deterministic:\nfirst=%s\nsecond=%s", first.String(), second.String())
	}

	var bom struct {
		BOMFormat   string `json:"bomFormat"`
		SpecVersion string `json:"specVersion"`
		Version     int    `json:"version"`
		Components  []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			PURL    string `json:"purl"`
		} `json:"components"`
	}
	if err := json.Unmarshal(first.Bytes(), &bom); err != nil {
		t.Fatalf("parse CycloneDX JSON: %v\n%s", err, first.String())
	}
	if bom.BOMFormat != "CycloneDX" || bom.SpecVersion != "1.6" || bom.Version != 1 {
		t.Fatalf("unexpected BOM header: %#v", bom)
	}
	if len(bom.Components) != 2 || bom.Components[0].Name != "@scope/lib" || bom.Components[0].PURL != "pkg:npm/%40scope/lib@1.2.3" {
		t.Fatalf("unexpected components: %#v", bom.Components)
	}
	if bom.Components[1].Name != "zod" || bom.Components[1].Version != "^3.0.0" || bom.Components[1].PURL != "pkg:npm/zod@%5E3.0.0" {
		t.Fatalf("requirement-backed component missing: %#v", bom.Components[1])
	}
}
