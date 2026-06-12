package triage

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func filterSections(sections []core.SectionResult, candidates []candidate, outcomes map[string]providerVerdict) []core.SectionResult {
	dismissed := make(map[string]struct{}, len(candidates))
	for _, item := range candidates {
		if verdict := normalizeVerdict(outcomes[item.hash]); verdict.Decision == "dismiss" {
			dismissed[candidatePositionKey(item.sectionIndex, item.findingIndex)] = struct{}{}
		}
	}

	filtered := make([]core.SectionResult, len(sections))
	for sectionIndex, section := range sections {
		filtered[sectionIndex] = section
		filtered[sectionIndex].Findings = make([]core.Finding, 0, len(section.Findings))
		filtered[sectionIndex].Status = core.StatusPass
		for findingIndex, finding := range section.Findings {
			if _, ok := dismissed[candidatePositionKey(sectionIndex, findingIndex)]; ok {
				continue
			}
			filtered[sectionIndex].Findings = append(filtered[sectionIndex].Findings, finding)
			switch finding.Level {
			case "fail":
				filtered[sectionIndex].Status = core.StatusFail
			case "warn":
				if filtered[sectionIndex].Status != core.StatusFail {
					filtered[sectionIndex].Status = core.StatusWarn
				}
			}
		}
	}
	return filtered
}

func buildArtifactVerdict(item candidate, verdict providerVerdict, cached bool) core.AIAnalysisVerdict {
	status := "verified"
	if verdict.Decision == "dismiss" {
		status = "dismissed"
	}
	if cached {
		status = "cached-" + status
	}
	return core.AIAnalysisVerdict{
		ID:          item.finding.Fingerprint,
		Kind:        "triage",
		RuleID:      item.finding.RuleID,
		Path:        item.finding.Path,
		Fingerprint: item.finding.Fingerprint,
		ContentHash: item.hash,
		Status:      status,
		Summary:     verdict.Summary,
	}
}

func loadCachedVerdict(cache VerdictCache, item candidate, runtime runtimeConfig) (providerVerdict, bool) {
	if cache == nil {
		return providerVerdict{}, false
	}
	cached, ok := cache.GetTriageVerdict(item.hash)
	if !ok {
		return providerVerdict{}, false
	}
	if cached.Provider != runtime.Provider || cached.Model != runtime.Model {
		return providerVerdict{}, false
	}
	return normalizeVerdict(providerVerdict{
		Decision: cached.Decision,
		Summary:  cached.Summary,
	}), true
}

func storeCachedVerdict(cache VerdictCache, item candidate, runtime runtimeConfig, verdict providerVerdict) {
	if cache == nil {
		return
	}
	cache.PutTriageVerdict(item.hash, core.AITriageCacheVerdict{
		Provider: runtime.Provider,
		Model:    runtime.Model,
		Decision: verdict.Decision,
		Summary:  verdict.Summary,
	})
}

func normalizeVerdict(verdict providerVerdict) providerVerdict {
	switch verdict.Decision {
	case "dismiss":
		return verdict
	default:
		verdict.Decision = "keep"
		if strings.TrimSpace(verdict.Summary) == "" {
			verdict.Summary = "finding remains plausible from the available local context"
		}
		return verdict
	}
}

func candidatePositionKey(sectionIndex int, findingIndex int) string {
	return fmt.Sprintf("%d:%d", sectionIndex, findingIndex)
}

func sortVerdicts(verdicts []core.AIAnalysisVerdict) {
	sort.Slice(verdicts, func(i int, j int) bool {
		if verdicts[i].Path == verdicts[j].Path {
			return verdicts[i].Fingerprint < verdicts[j].Fingerprint
		}
		return verdicts[i].Path < verdicts[j].Path
	})
}
