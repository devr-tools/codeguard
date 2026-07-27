package supplychain

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	artifactBuildPattern = regexp.MustCompile(`(?i)\b(docker\s+build|docker/build-push-action|goreleaser|npm\s+publish|cargo\s+publish|twine\s+upload|publish|release|upload-artifact|push\s+.+image|buildx)\b`)
	provenancePattern    = regexp.MustCompile(`(?i)\b(slsa|provenance|attest|attestation|cosign\s+attest|attest-build-provenance|sbom|cyclonedx|sigstore)\b`)
)

func missingProvenanceFindings(env support.Context, target core.TargetConfig, manifests []core.SupplyChainManifest) []core.Finding {
	if env.Config.Checks.SupplyChainRules.DetectProvenance == nil || !*env.Config.Checks.SupplyChainRules.DetectProvenance || len(manifests) == 0 {
		return nil
	}
	files := provenanceEvidenceFiles(env, target)
	if len(files) == 0 {
		return nil
	}
	for _, file := range files {
		if provenancePattern.MatchString(file.text) {
			return nil
		}
	}
	for _, file := range files {
		if !artifactBuildPattern.MatchString(file.text) {
			continue
		}
		return []core.Finding{env.NewFinding(support.FindingInput{
			RuleID:     "supply_chain.missing-provenance",
			Level:      "fail",
			Path:       file.rel,
			Line:       firstProvenanceLine(file.text),
			Column:     1,
			Message:    "artifact build or publish workflow lacks provenance or attestation evidence",
			Confidence: core.ConfidenceHigh,
			Metadata: map[string]string{
				"artifact_evidence": "build_or_publish",
			},
		})}
	}
	return nil
}

type provenanceFile struct {
	rel  string
	text string
}

func provenanceEvidenceFiles(env support.Context, target core.TargetConfig) []provenanceFile {
	files := make([]provenanceFile, 0)
	if env.VisitTargetFiles == nil {
		return files
	}
	env.VisitTargetFiles(target, isProvenanceEvidencePath, func(rel string, data []byte) {
		files = append(files, provenanceFile{rel: filepath.ToSlash(rel), text: string(data)})
	})
	return files
}

func isProvenanceEvidencePath(rel string) bool {
	normalized := strings.ToLower(filepath.ToSlash(rel))
	base := filepath.Base(normalized)
	return strings.HasPrefix(normalized, ".github/workflows/") ||
		strings.HasPrefix(normalized, ".buildkite/") ||
		strings.HasPrefix(normalized, "buildkite") ||
		strings.Contains(normalized, "release") ||
		strings.Contains(normalized, "deploy") ||
		strings.HasPrefix(base, "dockerfile") ||
		base == ".goreleaser.yaml" ||
		base == ".goreleaser.yml"
}

func firstProvenanceLine(text string) int {
	lines := strings.Split(text, "\n")
	for idx, line := range lines {
		if artifactBuildPattern.MatchString(line) {
			return idx + 1
		}
	}
	return 1
}
