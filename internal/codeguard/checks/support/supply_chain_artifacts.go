package support

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const ArtifactKindSupplyChain = "supply_chain"

func NewSupplyChainArtifact(id string, target string, manifests []core.SupplyChainManifest) core.Artifact {
	cloned := make([]core.SupplyChainManifest, 0, len(manifests))
	for _, manifest := range manifests {
		deps := append([]core.SupplyChainDependency(nil), manifest.Dependencies...)
		for i := range deps {
			deps[i].Groups = append([]string(nil), deps[i].Groups...)
			deps[i].LicenseCandidates = append([]core.SupplyChainLicenseCandidate(nil), deps[i].LicenseCandidates...)
		}
		lockfiles := append([]string(nil), manifest.Lockfiles...)
		cloned = append(cloned, core.SupplyChainManifest{
			Ecosystem:      manifest.Ecosystem,
			Path:           manifest.Path,
			Name:           manifest.Name,
			License:        manifest.License,
			LicenseLine:    manifest.LicenseLine,
			PackageManager: manifest.PackageManager,
			Lockfiles:      lockfiles,
			Dependencies:   deps,
		})
	}
	return core.Artifact{
		ID:     id,
		Kind:   ArtifactKindSupplyChain,
		Target: target,
		SupplyChain: &core.SupplyChainArtifact{
			Manifests: cloned,
		},
	}
}

func SupplyChainArtifactID(targetName string, targetPath string) string {
	name := strings.TrimSpace(targetName)
	if name == "" {
		name = strings.TrimSpace(targetPath)
	}
	return "supply_chain." + name
}
