package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

type fixContext struct {
	args fixToolArgs
	cfg  service.Config
}

var errInvalidFixArgs = errors.New("invalid fix arguments")

func loadFixContext(ctx context.Context, s *mcpToolService, raw json.RawMessage) (fixContext, error) {
	var args fixToolArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return fixContext{}, errInvalidFixArgs
	}
	confinedPath, err := confineConfigArg(ctx, s, args.ConfigPath)
	if err != nil {
		return fixContext{}, err
	}
	cfg, err := s.loadConfig(confinedPath, args.Profile)
	if err != nil {
		return fixContext{}, fmt.Errorf("load config: %w", err)
	}
	return fixContext{args: args, cfg: cfg}, nil
}

func requireFixDiff(name string, diff string) (map[string]any, bool) {
	if strings.TrimSpace(diff) == "" {
		return toolErrorResult(name + " requires a unified diff"), true
	}
	return nil, false
}

func requireFixFinding(ruleID string, message string) (map[string]any, bool) {
	if strings.TrimSpace(ruleID) == "" && strings.TrimSpace(message) == "" {
		return toolErrorResult("propose_fix requires a finding to fix"), true
	}
	return nil, false
}

func verifyFixCandidate(ctx context.Context, cfg service.Config, args fixToolArgs) (service.VerifiedFix, map[string]any, error, bool) {
	result, err := service.VerifyFix(ctx, cfg, args.Finding, service.FixCandidate{Diff: args.Diff}, args.options())
	if err == nil {
		return result, nil, nil, true
	}
	data := map[string]any{
		"verified":       false,
		"error":          err.Error(),
		"attempted_diff": args.Diff,
	}
	if report, perr := service.RunPatch(ctx, cfg, args.Diff); perr == nil {
		data["remaining_findings"] = report
	}
	return service.VerifiedFix{}, data, err, false
}

func toolResultFromError(err error) map[string]any {
	return toolErrorResult(err.Error())
}
