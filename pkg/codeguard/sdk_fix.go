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
