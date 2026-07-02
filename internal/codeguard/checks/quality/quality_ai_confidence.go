package quality

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// aiRuleConfidence assigns an explicit confidence to the AI-quality heuristics
// whose precision differs from the medium default. hallucinated-import is high
// because imports are resolved against the target's real dependency manifests
// (go.mod, package.json, requirements/pyproject); the drift/style heuristics
// compare against sampled repo conventions and are the noisiest, so they carry
// low confidence. Rules absent from this map (e.g. dead-code, swallowed-error)
// keep the unspecified/medium default.
var aiRuleConfidence = map[string]string{
	"quality.ai.hallucinated-import": core.ConfidenceHigh,
	"quality.ai.narrative-comment":   core.ConfidenceLow,
	"quality.ai.naming-drift":        core.ConfidenceLow,
	"quality.ai.error-style-drift":   core.ConfidenceLow,
	"quality.ai.local-idiom-drift":   core.ConfidenceLow,
	"quality.ai.over-mocked-test":    core.ConfidenceLow,
}
