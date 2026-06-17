package supplychain

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Run wires the supply-chain family into the scan pipeline. Rule execution is
// intentionally staged: config, metadata, and reporting land before manifest
// parsing and policy enforcement are added.
func Run(ctx context.Context, env support.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range env.Config.Targets {
		manifests := support.ResolveSupplyChainLicenses(ctx, env, target, support.CollectSupplyChainManifests(env, target))
		if len(manifests) == 0 {
			continue
		}
		if env.PutArtifact != nil {
			env.PutArtifact(support.NewSupplyChainArtifact(
				support.SupplyChainArtifactID(target.Name, target.Path),
				target.Path,
				manifests,
			))
		}
		findings = append(findings, targetFindings(ctx, env, target, manifests)...)
	}
	return env.FinalizeSection("supply_chain", "Supply Chain", findings)
}
