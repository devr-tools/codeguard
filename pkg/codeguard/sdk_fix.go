package codeguard

import (
	"context"

	internalfix "github.com/devr-tools/codeguard/internal/codeguard/ai/fix"
)

func VerifyFix(ctx context.Context, cfg Config, finding Finding, candidate FixCandidate, opts FixOptions) (VerifiedFix, error) {
	return internalfix.Verify(ctx, cfg, finding, candidate, opts)
}

func GenerateVerifiedFix(ctx context.Context, req FixGenerateRequest) (VerifiedFix, error) {
	return internalfix.GenerateVerified(ctx, req)
}

// VerifyFixBatch verifies compatible deterministic fixes in one isolated
// workspace and returns their aggregate patch. It never changes the working
// tree.
func VerifyFixBatch(ctx context.Context, req FixBatchRequest) (FixBatchResult, error) {
	return internalfix.VerifyBatch(ctx, req)
}
