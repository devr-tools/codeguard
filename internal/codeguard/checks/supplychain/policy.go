package supplychain

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func targetFindings(_ context.Context, env support.Context, target core.TargetConfig, manifests []core.SupplyChainManifest) []core.Finding {
	findings := make([]core.Finding, 0)
	changed := changedFilesSet(env.ChangedFiles)
	for _, manifest := range manifests {
		findings = append(findings, unpinnedDependencyFindings(env, manifest)...)
		findings = append(findings, lockfilePolicyFindings(env, target, manifest, changed)...)
		findings = append(findings, licensePolicyFindings(env, manifest)...)
	}
	return findings
}

func unpinnedDependencyFindings(env support.Context, manifest core.SupplyChainManifest) []core.Finding {
	if env.Config.Checks.SupplyChainRules.DetectUnpinned == nil || !*env.Config.Checks.SupplyChainRules.DetectUnpinned {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, dep := range manifest.Dependencies {
		if dep.Pinned {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "supply_chain.unpinned-dependency",
			Level:   "warn",
			Path:    manifest.Path,
			Line:    dep.Line,
			Column:  1,
			Message: "dependency " + dep.Name + " is not pinned to a concrete version or digest",
		}))
	}
	return findings
}

func lockfilePolicyFindings(env support.Context, target core.TargetConfig, manifest core.SupplyChainManifest, changed map[string]struct{}) []core.Finding {
	findings := make([]core.Finding, 0)
	expectLockfile := manifestExpectsLockfile(manifest)
	if expectLockfile && env.Config.Checks.SupplyChainRules.RequireLockfile != nil && *env.Config.Checks.SupplyChainRules.RequireLockfile && len(manifest.Dependencies) > 0 && len(manifest.Lockfiles) == 0 {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "supply_chain.missing-lockfile",
			Level:   "fail",
			Path:    manifest.Path,
			Message: "manifest has dependencies but no expected lockfile is present",
		}))
	}
	if env.Mode != core.ScanModeDiff || env.Config.Checks.SupplyChainRules.DetectLockfileDrift == nil || !*env.Config.Checks.SupplyChainRules.DetectLockfileDrift {
		return append(findings, lockfileContentFindings(env, target, manifest)...)
	}
	if _, ok := changed[manifest.Path]; !ok {
		return append(findings, lockfileContentFindings(env, target, manifest)...)
	}
	if !expectLockfile || len(manifest.Dependencies) == 0 {
		return append(findings, lockfileContentFindings(env, target, manifest)...)
	}
	if len(manifest.Lockfiles) == 0 {
		return findings
	}
	for _, lockfile := range manifest.Lockfiles {
		if _, ok := changed[lockfile]; ok {
			return append(findings, lockfileContentFindings(env, target, manifest)...)
		}
	}
	findings = append(findings, env.NewFinding(support.FindingInput{
		RuleID:  "supply_chain.lockfile-drift",
		Level:   "fail",
		Path:    manifest.Path,
		Message: "manifest changed without a matching lockfile update",
	}))
	return append(findings, lockfileContentFindings(env, target, manifest)...)
}

func lockfileContentFindings(env support.Context, target core.TargetConfig, manifest core.SupplyChainManifest) []core.Finding {
	if env.Config.Checks.SupplyChainRules.DetectLockfileDrift == nil || !*env.Config.Checks.SupplyChainRules.DetectLockfileDrift {
		return nil
	}
	issues := support.SupplyChainLockfileIssues(target.Path, manifest)
	if len(issues) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0, len(issues))
	for _, issue := range issues {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "supply_chain.lockfile-drift",
			Level:   "fail",
			Path:    manifest.Path,
			Message: issue,
		}))
	}
	return findings
}
