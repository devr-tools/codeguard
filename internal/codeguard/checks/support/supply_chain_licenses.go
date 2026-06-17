package support

import (
	"context"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type SupplyChainLicenseCommandResult struct {
	Name       string                             `json:"name"`
	Coordinate string                             `json:"coordinate,omitempty"`
	License    string                             `json:"license,omitempty"`
	Source     string                             `json:"source,omitempty"`
	Candidates []core.SupplyChainLicenseCandidate `json:"candidates,omitempty"`
}

type SupplyChainLicenseCommandContext struct {
	Ecosystem              string                     `json:"ecosystem"`
	ManifestPath           string                     `json:"manifest_path"`
	TargetName             string                     `json:"target_name,omitempty"`
	TargetPath             string                     `json:"target_path"`
	UnresolvedDependencies []SupplyChainDependencyRef `json:"unresolved_dependencies"`
}

type SupplyChainDependencyRef struct {
	Coordinate  string   `json:"coordinate,omitempty"`
	Name        string   `json:"name"`
	Requirement string   `json:"requirement,omitempty"`
	Version     string   `json:"version,omitempty"`
	Scope       string   `json:"scope,omitempty"`
	Groups      []string `json:"groups,omitempty"`
	Indirect    bool     `json:"indirect,omitempty"`
	Pinned      bool     `json:"pinned,omitempty"`
	Line        int      `json:"line,omitempty"`
}

func ResolveSupplyChainLicenses(ctx context.Context, env Context, target core.TargetConfig, manifests []core.SupplyChainManifest) []core.SupplyChainManifest {
	if len(manifests) == 0 {
		return nil
	}
	resolved := make([]core.SupplyChainManifest, 0, len(manifests))
	for _, manifest := range manifests {
		updated := manifest
		updated.Dependencies = append([]core.SupplyChainDependency(nil), manifest.Dependencies...)
		resolveLocalDependencyLicenses(target.Path, manifest, &updated)
		fillSupplyChainLicensesFromCommand(ctx, env, target, &updated)
		resolved = append(resolved, updated)
	}
	return resolved
}

func resolveLocalDependencyLicenses(root string, manifest core.SupplyChainManifest, updated *core.SupplyChainManifest) {
	if updated == nil {
		return
	}
	for i := range updated.Dependencies {
		dep := &updated.Dependencies[i]
		license, source := resolveDependencyLicense(root, manifest, *dep)
		if strings.TrimSpace(license) == "" {
			continue
		}
		setSupplyChainDependencyLicense(dep, core.SupplyChainLicenseCandidate{
			License:    strings.TrimSpace(license),
			Confidence: "high",
			Provenance: "local-metadata",
			Source:     strings.TrimSpace(source),
		})
	}
}

func setSupplyChainDependencyLicense(dep *core.SupplyChainDependency, candidates ...core.SupplyChainLicenseCandidate) {
	if dep == nil {
		return
	}
	normalized := normalizeSupplyChainLicenseCandidates(candidates)
	if len(normalized) == 0 {
		return
	}
	selected := selectBestSupplyChainLicenseCandidate(normalized)
	dep.License = selected.License
	dep.LicenseSource = FirstNonEmptyTrimmedString(selected.Source, dep.LicenseSource)
	dep.LicenseCandidates = normalized
}

func resolveDependencyLicense(root string, manifest core.SupplyChainManifest, dep core.SupplyChainDependency) (string, string) {
	switch manifest.Ecosystem {
	case "npm":
		return resolveNodeDependencyLicense(root, manifest, dep)
	case "cargo":
		return resolveCargoDependencyLicense(root, manifest, dep)
	case "python":
		return resolvePythonDependencyLicense(root, manifest, dep)
	default:
		return "", ""
	}
}
