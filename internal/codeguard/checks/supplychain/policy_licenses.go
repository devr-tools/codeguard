package supplychain

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func licensePolicyFindings(env support.Context, manifest core.SupplyChainManifest) []core.Finding {
	allowed := normalizeLicenseList(env.Config.Checks.SupplyChainRules.AllowedLicenses)
	denied := normalizeLicenseList(env.Config.Checks.SupplyChainRules.DeniedLicenses)
	findings := make([]core.Finding, 0)
	if finding, ok := manifestLicenseFinding(env, manifest, allowed, denied); ok {
		findings = append(findings, finding)
	}
	for _, dep := range manifest.Dependencies {
		if finding, ok := dependencyLicenseFinding(env, manifest, dep, allowed, denied); ok {
			findings = append(findings, finding)
		}
	}
	return findings
}

func manifestLicenseFinding(env support.Context, manifest core.SupplyChainManifest, allowed []string, denied []string) (core.Finding, bool) {
	license, tokens := normalizeLicenseExpression(manifest.License)
	if license == "" && len(tokens) == 0 {
		return core.Finding{}, false
	}
	switch {
	case licenseDenied(license, tokens, denied):
		return env.NewFinding(support.FindingInput{
			RuleID:  "supply_chain.denied-license",
			Level:   "fail",
			Path:    manifest.Path,
			Line:    manifest.LicenseLine,
			Column:  1,
			Message: "manifest declares denied license " + manifest.License,
		}), true
	case licenseOutsideAllowed(license, tokens, allowed):
		return env.NewFinding(support.FindingInput{
			RuleID:  "supply_chain.denied-license",
			Level:   "fail",
			Path:    manifest.Path,
			Line:    manifest.LicenseLine,
			Column:  1,
			Message: "manifest license " + manifest.License + " is not in the allowed license policy",
		}), true
	default:
		return core.Finding{}, false
	}
}

func dependencyLicenseFinding(env support.Context, manifest core.SupplyChainManifest, dep core.SupplyChainDependency, allowed []string, denied []string) (core.Finding, bool) {
	evidence := selectDependencyLicenseEvidence(dep)
	license, tokens := normalizeLicenseExpression(evidence.License)
	if license == "" && len(tokens) == 0 {
		return core.Finding{}, false
	}
	level := "fail"
	if !evidence.Definitive {
		level = "warn"
	}
	switch {
	case licenseDenied(license, tokens, denied):
		return env.NewFinding(support.FindingInput{
			RuleID:  "supply_chain.denied-license",
			Level:   level,
			Path:    manifest.Path,
			Line:    dep.Line,
			Column:  1,
			Message: "dependency " + dep.Name + " resolves to denied license " + evidence.License + dependencyLicenseEvidenceSuffix(evidence),
		}), true
	case licenseOutsideAllowed(license, tokens, allowed):
		return env.NewFinding(support.FindingInput{
			RuleID:  "supply_chain.denied-license",
			Level:   level,
			Path:    manifest.Path,
			Line:    dep.Line,
			Column:  1,
			Message: "dependency " + dep.Name + " resolves to license " + evidence.License + " which is not in the allowed license policy" + dependencyLicenseEvidenceSuffix(evidence),
		}), true
	default:
		return core.Finding{}, false
	}
}

type dependencyLicenseEvidence struct {
	License    string
	Source     string
	Confidence string
	Provenance string
	Definitive bool
}

func selectDependencyLicenseEvidence(dep core.SupplyChainDependency) dependencyLicenseEvidence {
	candidates := dep.LicenseCandidates
	if len(candidates) == 0 {
		return dependencyLicenseEvidence{
			License:    dep.License,
			Source:     dep.LicenseSource,
			Definitive: strings.TrimSpace(dep.License) != "",
		}
	}
	best := candidates[0]
	bestScore := support.SupplyChainLicenseCandidateRank(best)
	for _, candidate := range candidates[1:] {
		score := support.SupplyChainLicenseCandidateRank(candidate)
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	return dependencyLicenseEvidence{
		License:    best.License,
		Source:     support.FirstNonEmptyTrimmedString(best.Source, dep.LicenseSource),
		Confidence: best.Confidence,
		Provenance: best.Provenance,
		Definitive: support.SupplyChainLicenseCandidateDefinitive(best),
	}
}

func dependencyLicenseEvidenceSuffix(evidence dependencyLicenseEvidence) string {
	parts := make([]string, 0, 3)
	if source := strings.TrimSpace(evidence.Source); source != "" {
		parts = append(parts, source)
	}
	if provenance := strings.TrimSpace(evidence.Provenance); provenance != "" {
		parts = append(parts, "provenance="+provenance)
	}
	if confidence := strings.TrimSpace(evidence.Confidence); confidence != "" {
		parts = append(parts, "confidence="+confidence)
	}
	if len(parts) == 0 {
		return ""
	}
	if !evidence.Definitive {
		parts = append(parts, "heuristic")
	}
	return " (" + strings.Join(parts, ", ") + ")"
}
