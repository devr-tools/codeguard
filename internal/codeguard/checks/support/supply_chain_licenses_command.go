package support

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func fillSupplyChainLicensesFromCommand(ctx context.Context, env Context, target core.TargetConfig, manifest *core.SupplyChainManifest) {
	command, unresolved, ok := supplyChainLicenseCommandInputs(env, manifest)
	if !ok {
		return
	}
	ctxPath, cleanup, err := writeSupplyChainLicenseContext(newSupplyChainLicenseCommandContext(target, manifest, unresolved))
	if err != nil {
		return
	}
	defer cleanup()

	output, err := env.RunCommandCheckWithEnv(ctx, manifestWorkdir(target.Path, manifest.Path), command, supplyChainLicenseCommandEnv(target, manifest, unresolved, ctxPath))
	if err != nil {
		return
	}
	applySupplyChainLicenseCommandResults(manifest, parseSupplyChainLicenseCommandOutput(output))
}

func supplyChainLicenseCommandInputs(env Context, manifest *core.SupplyChainManifest) (core.CommandCheckConfig, []SupplyChainDependencyRef, bool) {
	if manifest == nil {
		return core.CommandCheckConfig{}, nil, false
	}
	command, ok := env.Config.Checks.SupplyChainRules.LicenseCommands[manifest.Ecosystem]
	if !ok {
		return core.CommandCheckConfig{}, nil, false
	}
	unresolved := unresolvedDependencies(manifest.Dependencies)
	if len(unresolved) == 0 {
		return core.CommandCheckConfig{}, nil, false
	}
	return command, unresolved, true
}

func newSupplyChainLicenseCommandContext(target core.TargetConfig, manifest *core.SupplyChainManifest, unresolved []SupplyChainDependencyRef) SupplyChainLicenseCommandContext {
	return SupplyChainLicenseCommandContext{
		Ecosystem:              manifest.Ecosystem,
		ManifestPath:           manifest.Path,
		TargetName:             target.Name,
		TargetPath:             target.Path,
		UnresolvedDependencies: unresolved,
	}
}

func supplyChainLicenseCommandEnv(target core.TargetConfig, manifest *core.SupplyChainManifest, unresolved []SupplyChainDependencyRef, ctxPath string) []string {
	return []string{
		"CODEGUARD_SUPPLY_CHAIN_ECOSYSTEM=" + manifest.Ecosystem,
		"CODEGUARD_SUPPLY_CHAIN_MANIFEST_PATH=" + manifest.Path,
		"CODEGUARD_SUPPLY_CHAIN_MANIFEST_DIR=" + manifestRelativeDir(manifest.Path),
		"CODEGUARD_SUPPLY_CHAIN_TARGET_NAME=" + target.Name,
		"CODEGUARD_SUPPLY_CHAIN_TARGET_PATH=" + target.Path,
		"CODEGUARD_SUPPLY_CHAIN_UNRESOLVED_NAMES=" + strings.Join(unresolvedDependencyRefNames(unresolved), ","),
		"CODEGUARD_SUPPLY_CHAIN_UNRESOLVED_COORDINATES=" + strings.Join(unresolvedDependencyRefCoordinates(unresolved), ","),
		"CODEGUARD_SUPPLY_CHAIN_CONTEXT_FILE=" + ctxPath,
	}
}

func applySupplyChainLicenseCommandResults(manifest *core.SupplyChainManifest, results []SupplyChainLicenseCommandResult) {
	if manifest == nil || len(results) == 0 {
		return
	}
	for i := range manifest.Dependencies {
		dep := &manifest.Dependencies[i]
		if strings.TrimSpace(dep.License) != "" {
			continue
		}
		if candidates := matchingSupplyChainLicenseCandidates(results, *dep); len(candidates) != 0 {
			setSupplyChainDependencyLicense(dep, candidates...)
		}
	}
}

func matchingSupplyChainLicenseCandidates(results []SupplyChainLicenseCommandResult, dep core.SupplyChainDependency) []core.SupplyChainLicenseCandidate {
	for _, result := range results {
		if !supplyChainLicenseResultMatchesDependency(result, dep) {
			continue
		}
		if candidates := supplyChainLicenseCandidatesFromResult(result); len(candidates) != 0 {
			return candidates
		}
	}
	return nil
}

func unresolvedDependencies(deps []core.SupplyChainDependency) []SupplyChainDependencyRef {
	out := make([]SupplyChainDependencyRef, 0)
	seen := map[string]struct{}{}
	for _, dep := range deps {
		if strings.TrimSpace(dep.Name) == "" || strings.TrimSpace(dep.License) != "" {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(SupplyChainDependencyCoordinate(dep)))
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(dep.Name))
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, SupplyChainDependencyRef{
			Coordinate:  SupplyChainDependencyCoordinate(dep),
			Name:        dep.Name,
			Requirement: dep.Requirement,
			Version:     dep.Version,
			Scope:       dep.Scope,
			Groups:      append([]string(nil), dep.Groups...),
			Indirect:    dep.Indirect,
			Pinned:      dep.Pinned,
			Line:        dep.Line,
		})
	}
	slices.SortFunc(out, func(a, b SupplyChainDependencyRef) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

func unresolvedDependencyRefNames(deps []SupplyChainDependencyRef) []string {
	names := make([]string, 0, len(deps))
	for _, dep := range deps {
		if strings.TrimSpace(dep.Name) != "" {
			names = append(names, dep.Name)
		}
	}
	slices.Sort(names)
	return names
}

func unresolvedDependencyRefCoordinates(deps []SupplyChainDependencyRef) []string {
	coords := make([]string, 0, len(deps))
	for _, dep := range deps {
		if strings.TrimSpace(dep.Coordinate) != "" {
			coords = append(coords, dep.Coordinate)
		}
	}
	slices.Sort(coords)
	return coords
}

func writeSupplyChainLicenseContext(ctx SupplyChainLicenseCommandContext) (string, func(), error) {
	data, err := json.Marshal(ctx)
	if err != nil {
		return "", func() {}, err
	}
	file, err := os.CreateTemp("", "codeguard-supply-chain-license-*.json")
	if err != nil {
		return "", func() {}, err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", func() {}, err
	}
	return file.Name(), func() { _ = os.Remove(file.Name()) }, nil
}

func manifestWorkdir(root string, manifestPath string) string {
	return filepath.Join(root, filepath.FromSlash(manifestRelativeDir(manifestPath)))
}
