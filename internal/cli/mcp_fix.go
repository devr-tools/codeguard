package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// mcp_fix.go exposes codeguard's verified auto-fix flow as MCP tools:
//   - verify_fix:  the caller supplies a candidate diff; the server applies it
//     in a throwaway worktree, re-scans the changed lines, runs the nearest
//     inferred tests, and returns the result only if it verifies (fails closed).
//   - propose_fix: the server first generates the candidate diff (via the
//     client's LLM when the client supports MCP sampling, else a configured AI
//     provider) and then runs the same verification.
//
// Neither tool mutates the working tree — verification happens in a temp copy
// and the verified diff is returned for the agent to apply — so both are
// annotated read-only / non-destructive.

type fixToolArgs struct {
	ConfigPath      string                           `json:"config_path"`
	Profile         string                           `json:"profile"`
	Finding         service.Finding                  `json:"finding"`
	Diff            string                           `json:"diff"`
	BaseRef         string                           `json:"base_ref"`
	MaxNearestTests int                              `json:"max_nearest_tests"`
	TestCommands    []service.FixVerificationCommand `json:"test_commands"`
}

func (a fixToolArgs) options() service.FixOptions {
	return service.FixOptions{
		BaseRef:         strings.TrimSpace(a.BaseRef),
		MaxNearestTests: a.MaxNearestTests,
		TestCommands:    a.TestCommands,
	}
}

func (s *mcpToolService) callVerifyFix(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	fixCtx, err := loadFixContext(ctx, s, raw)
	if err != nil {
		if errors.Is(err, errInvalidFixArgs) {
			return nil, fmt.Errorf("invalid verify_fix arguments")
		}
		return toolResultFromError(err), nil
	}
	if result, stop := requireFixDiff("verify_fix", fixCtx.args.Diff); stop {
		return result, nil
	}

	result, failure, ok, verifyErr := verifyFixCandidate(ctx, fixCtx.cfg, fixCtx.args)
	if ok {
		return toolSuccessResult(result), nil
	}
	return toolErrorResultData(fmt.Sprintf("fix did not verify: %v", verifyErr), failure), nil
}

func (s *mcpToolService) callProposeFix(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	fixCtx, err := loadFixContext(ctx, s, raw)
	if err != nil {
		if errors.Is(err, errInvalidFixArgs) {
			return nil, fmt.Errorf("invalid propose_fix arguments")
		}
		return toolResultFromError(err), nil
	}
	if result, stop := requireFixFinding(fixCtx.args.Finding.RuleID, fixCtx.args.Finding.Message); stop {
		return result, nil
	}

	generator, err := s.resolveFixGenerator(ctx, fixCtx.cfg)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("initialize fix generator: %v", err)), nil
	}
	if generator == nil {
		return toolErrorResult("no fix generator available: the client does not support sampling and no AI provider is configured"), nil
	}

	result, err := service.GenerateVerifiedFix(ctx, service.FixGenerateRequest{
		Config:    fixCtx.cfg,
		Finding:   fixCtx.args.Finding,
		Analysis:  firstNonEmpty(fixCtx.args.Finding.Why, fixCtx.args.Finding.Message),
		Generator: generator,
		Options:   fixCtx.args.options(),
	})
	if err != nil {
		return toolErrorResultData(fmt.Sprintf("generate verified fix: %v", err), map[string]any{
			"verified": false,
			"error":    err.Error(),
		}), nil
	}
	return toolSuccessResult(result), nil
}

func (s *mcpToolService) callApplyFix(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	fixCtx, err := loadFixContext(ctx, s, raw)
	if err != nil {
		if errors.Is(err, errInvalidFixArgs) {
			return nil, fmt.Errorf("invalid apply_fix arguments")
		}
		return toolResultFromError(err), nil
	}
	if result, stop := requireFixDiff("apply_fix", fixCtx.args.Diff); stop {
		return result, nil
	}

	verified, failure, ok, verifyErr := verifyFixCandidate(ctx, fixCtx.cfg, fixCtx.args)
	if !ok {
		return toolErrorResultData(fmt.Sprintf("fix did not verify, not applied: %v", verifyErr), failure), nil
	}
	if reply := confirmApplyResult(ctx, clientCallerFrom(ctx), verified.ChangedFiles, verified.Diff); reply != nil {
		return reply, nil
	}
	if err := runnersupport.ApplyUnifiedDiff(fixCtx.cfg, verified.Diff); err != nil { //nolint:contextcheck // git helpers use a contained timeout; deeper ctx threading is a tracked follow-up
		return toolErrorResult(fmt.Sprintf("verified fix failed to apply to the working tree: %v", err)), nil
	}
	return toolSuccessResult(map[string]any{
		"applied":       true,
		"diff":          verified.Diff,
		"summary":       verified.Summary,
		"changed_files": verified.ChangedFiles,
		"report":        verified.Report,
		"test_results":  verified.TestResults,
	}), nil
}

func confirmApplyResult(ctx context.Context, caller clientCaller, changedFiles []string, diff string) map[string]any {
	if caller == nil || !caller.supports("elicitation") {
		return nil
	}
	accepted, err := confirmApply(ctx, caller, changedFiles)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("confirmation failed: %v", err))
	}
	if accepted {
		return nil
	}
	return toolSuccessResult(map[string]any{
		"applied":       false,
		"declined":      true,
		"diff":          diff,
		"changed_files": changedFiles,
	})
}
