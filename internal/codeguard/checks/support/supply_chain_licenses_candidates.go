package support

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func parseSupplyChainLicenseCommandOutput(output string) []SupplyChainLicenseCommandResult {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}
	var array []SupplyChainLicenseCommandResult
	if err := json.Unmarshal([]byte(output), &array); err == nil {
		return array
	}
	var wrapped struct {
		Dependencies []SupplyChainLicenseCommandResult `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(output), &wrapped); err == nil {
		return wrapped.Dependencies
	}
	return nil
}

func supplyChainLicenseCandidatesFromResult(result SupplyChainLicenseCommandResult) []core.SupplyChainLicenseCandidate {
	candidates := normalizeSupplyChainLicenseCandidates(result.Candidates)
	if len(candidates) != 0 {
		return candidates
	}
	if strings.TrimSpace(result.License) == "" {
		return nil
	}
	return []core.SupplyChainLicenseCandidate{{
		License:    strings.TrimSpace(result.License),
		Confidence: "high",
		Provenance: "command-output",
		Source:     FirstNonEmptyTrimmedString(result.Source, "license-command"),
	}}
}

func normalizeSupplyChainLicenseCandidates(candidates []core.SupplyChainLicenseCandidate) []core.SupplyChainLicenseCandidate {
	if len(candidates) == 0 {
		return nil
	}
	normalized := make([]core.SupplyChainLicenseCandidate, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		normalizedCandidate := core.SupplyChainLicenseCandidate{
			License:    strings.TrimSpace(candidate.License),
			Confidence: strings.TrimSpace(candidate.Confidence),
			Provenance: strings.TrimSpace(candidate.Provenance),
			Source:     strings.TrimSpace(candidate.Source),
		}
		if normalizedCandidate.License == "" {
			continue
		}
		normalizedCandidate.Source = FirstNonEmptyTrimmedString(normalizedCandidate.Source, "license-command")
		key := strings.ToLower(normalizedCandidate.License) + "|" + strings.ToLower(normalizedCandidate.Confidence) + "|" + strings.ToLower(normalizedCandidate.Provenance) + "|" + strings.ToLower(normalizedCandidate.Source)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, normalizedCandidate)
	}
	slices.SortFunc(normalized, compareSupplyChainLicenseCandidates)
	return normalized
}

func selectBestSupplyChainLicenseCandidate(candidates []core.SupplyChainLicenseCandidate) core.SupplyChainLicenseCandidate {
	if len(candidates) == 0 {
		return core.SupplyChainLicenseCandidate{}
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if compareSupplyChainLicenseCandidates(candidate, best) < 0 {
			best = candidate
		}
	}
	return best
}

func compareSupplyChainLicenseCandidates(a core.SupplyChainLicenseCandidate, b core.SupplyChainLicenseCandidate) int {
	if rankA, rankB := SupplyChainLicenseCandidateRank(a), SupplyChainLicenseCandidateRank(b); rankA != rankB {
		if rankA > rankB {
			return -1
		}
		return 1
	}
	if cmp := strings.Compare(strings.ToLower(a.License), strings.ToLower(b.License)); cmp != 0 {
		return cmp
	}
	if cmp := strings.Compare(strings.ToLower(a.Provenance), strings.ToLower(b.Provenance)); cmp != 0 {
		return cmp
	}
	return strings.Compare(strings.ToLower(a.Source), strings.ToLower(b.Source))
}

func supplyChainLicenseResultMatchesDependency(result SupplyChainLicenseCommandResult, dep core.SupplyChainDependency) bool {
	resultCoordinate := strings.TrimSpace(result.Coordinate)
	depCoordinate := strings.TrimSpace(SupplyChainDependencyCoordinate(dep))
	if resultCoordinate != "" && depCoordinate != "" {
		return strings.EqualFold(resultCoordinate, depCoordinate)
	}
	return strings.EqualFold(strings.TrimSpace(result.Name), strings.TrimSpace(dep.Name))
}
