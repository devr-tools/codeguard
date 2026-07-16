package quality

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/semanticreview"
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// semanticFindings runs the shared command-backed semantic review and emits
// the quality.* verdicts. The request itself is built centrally in
// semanticreview.Options and is shared with the performance section (which
// emits the performance.* verdicts from the same cached response).
func semanticFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return semanticreview.Findings(ctx, env, target, "quality.", "quality.ai.semantic-runtime")
}
