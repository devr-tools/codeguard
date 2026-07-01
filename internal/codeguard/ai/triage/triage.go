package triage

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const (
	analysisArtifactID   = "ai_analysis.triage"
	analysisArtifactKind = "ai_analysis"
)

type VerdictCache interface {
	GetTriageVerdict(contentHash string) (core.AITriageCacheVerdict, bool)
	PutTriageVerdict(contentHash string, verdict core.AITriageCacheVerdict)
}

type candidate struct {
	hash         string
	sectionIndex int
	findingIndex int
	sectionName  string
	finding      core.Finding
	snippet      string
}

type providerVerdict struct {
	Decision string
	Summary  string
}

func Apply(ctx context.Context, cfg core.Config, opts core.ScanOptions, sections []core.SectionResult, cache VerdictCache) ([]core.SectionResult, *core.Artifact) {
	runtime := discoverRuntime(cfg.AI, opts)
	if !runtime.enabled() {
		return sections, nil
	}

	artifact := &core.Artifact{
		ID:   analysisArtifactID,
		Kind: analysisArtifactKind,
		AIAnalysis: &core.AIAnalysisArtifact{
			Provider: runtime.displayName(),
			Mode:     "triage",
		},
	}
	if err := runtime.validate(); err != nil {
		artifact.AIAnalysis.Verdicts = []core.AIAnalysisVerdict{{
			ID:      "ai-triage-config",
			Kind:    "triage",
			Status:  "error",
			Summary: err.Error(),
		}}
		return sections, artifact
	}

	candidates := collectCandidates(cfg, sections)
	if len(candidates) == 0 {
		return sections, artifact
	}

	provider := newProvider(runtime)
	outcomes := make(map[string]providerVerdict, len(candidates))
	verdicts := make([]core.AIAnalysisVerdict, 0, len(candidates))
	pending := make([]candidate, 0, len(candidates))

	for _, item := range candidates {
		if cached, ok := loadCachedVerdict(cache, item, runtime); ok {
			outcomes[item.hash] = cached
			verdicts = append(verdicts, buildArtifactVerdict(item, cached, true))
			continue
		}
		pending = append(pending, item)
	}

	if len(pending) > 0 {
		fresh, err := provider.Triage(ctx, pending)
		if err != nil {
			//nolint:gocritic // intentional: cached verdicts plus an error verdict, written to a different slice
			artifact.AIAnalysis.Verdicts = append(verdicts, core.AIAnalysisVerdict{
				ID:      "ai-triage-provider",
				Kind:    "triage",
				Status:  "error",
				Summary: err.Error(),
			})
			return sections, artifact
		}
		for _, item := range pending {
			verdict, ok := fresh[item.hash]
			if !ok {
				verdict = providerVerdict{
					Decision: "keep",
					Summary:  "provider returned no verdict; kept conservatively",
				}
			}
			verdict = normalizeVerdict(verdict)
			outcomes[item.hash] = verdict
			storeCachedVerdict(cache, item, runtime, verdict)
			verdicts = append(verdicts, buildArtifactVerdict(item, verdict, false))
		}
	}

	sortVerdicts(verdicts)
	artifact.AIAnalysis.Verdicts = verdicts
	filtered := filterSections(sections, candidates, outcomes)
	return filtered, artifact
}
