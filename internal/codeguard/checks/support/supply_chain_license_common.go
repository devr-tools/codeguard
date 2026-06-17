package support

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func SupplyChainLicenseCandidateRank(candidate core.SupplyChainLicenseCandidate) int {
	confidence := strings.ToLower(strings.TrimSpace(candidate.Confidence))
	provenance := strings.ToLower(strings.TrimSpace(candidate.Provenance))
	score := 0
	switch confidence {
	case "definitive", "certain", "exact", "high":
		score += 40
	case "medium", "probable":
		score += 20
	case "low", "heuristic", "weak":
		score += 5
	}
	switch {
	case strings.Contains(provenance, "spdx"):
		score += 40
	case strings.Contains(provenance, "metadata"), strings.Contains(provenance, "manifest"), strings.Contains(provenance, "package-manager"):
		score += 20
	case strings.Contains(provenance, "heuristic"), strings.Contains(provenance, "guess"):
		score += 5
	}
	if score == 0 {
		score = 10
	}
	return score
}

func SupplyChainLicenseCandidateDefinitive(candidate core.SupplyChainLicenseCandidate) bool {
	confidence := strings.ToLower(strings.TrimSpace(candidate.Confidence))
	if confidence == "definitive" || confidence == "certain" || confidence == "exact" || confidence == "high" {
		return true
	}
	provenance := strings.ToLower(strings.TrimSpace(candidate.Provenance))
	return strings.Contains(provenance, "spdx") || strings.Contains(provenance, "metadata") || strings.Contains(provenance, "manifest")
}

func FirstNonEmptyTrimmedString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func SupplyChainDependencyCoordinate(dep core.SupplyChainDependency) string {
	name := strings.TrimSpace(dep.Name)
	version := strings.TrimSpace(dep.Version)
	if name == "" || version == "" {
		return ""
	}
	return name + "@" + version
}
