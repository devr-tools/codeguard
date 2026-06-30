package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// errConfigPathNotPermitted is returned when a caller-supplied config_path
// resolves outside the allowed roots. The message is intentionally generic so
// the HTTP transport does not become a filesystem oracle.
var errConfigPathNotPermitted = errors.New("config_path is not within an allowed root")

func (s *mcpToolService) callScan(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		ConfigPath string `json:"config_path"`
		Profile    string `json:"profile"`
		Mode       string `json:"mode"`
		BaseRef    string `json:"base_ref"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid scan arguments")
	}

	confinedPath, err := confineConfigArg(ctx, s, args.ConfigPath)
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	cfg, err := s.loadConfig(confinedPath, args.Profile)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("load config: %v", err)), nil
	}
	mode, err := parseScanMode(args.Mode)
	if err != nil && strings.TrimSpace(args.Mode) != "" {
		return toolErrorResult(err.Error()), nil
	}
	if mode == "" {
		mode = service.ScanModeFull
	}
	baseRef := strings.TrimSpace(args.BaseRef)
	if baseRef == "" {
		baseRef = "main"
	}
	// Validate the caller-supplied ref once at the trust boundary so a value
	// beginning with "-" cannot be parsed by git as an option, and so only a
	// conservative ref/SHA charset reaches the git invocations downstream.
	if err := runnersupport.ValidateBaseRef(baseRef); err != nil {
		return toolErrorResult(err.Error()), nil
	}

	opts := service.ScanOptions{Mode: mode, BaseRef: baseRef}
	if emit := progressFrom(ctx); emit != nil {
		total := countEnabledSections(cfg, mode)
		var mu sync.Mutex
		var done float64
		opts.OnSectionComplete = func(section service.SectionResult) {
			mu.Lock()
			done++
			progress := done
			mu.Unlock()
			emit(progress, total, fmt.Sprintf("%s: %s (%d findings)", section.Name, section.Status, len(section.Findings)))
		}
	}

	report, err := service.RunWithOptions(ctx, cfg, opts)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("scan failed: %v", err)), nil
	}
	return toolSuccessResult(report), nil
}

func (s *mcpToolService) callValidateConfig(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		ConfigPath string `json:"config_path"`
		Profile    string `json:"profile"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid validate_config arguments")
	}

	confinedPath, err := confineConfigArg(ctx, s, args.ConfigPath)
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	cfg, err := s.loadConfig(confinedPath, args.Profile)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("load config: %v", err)), nil
	}
	if err := service.ValidateConfig(cfg); err != nil {
		return toolErrorResult(fmt.Sprintf("invalid config: %v", err)), nil
	}
	return toolSuccessResult(map[string]any{
		"ok":          true,
		"profile":     cfg.Profile,
		"config_name": cfg.Name,
	}), nil
}

func (s *mcpToolService) callValidatePatch(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		ConfigPath string `json:"config_path"`
		Profile    string `json:"profile"`
		Diff       string `json:"diff"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid validate_patch arguments")
	}
	if strings.TrimSpace(args.Diff) == "" {
		return toolErrorResult("validate_patch requires a unified diff"), nil
	}

	confinedPath, err := confineConfigArg(ctx, s, args.ConfigPath)
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	cfg, err := s.loadConfig(confinedPath, args.Profile)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("load config: %v", err)), nil
	}
	report, err := service.RunPatch(ctx, cfg, args.Diff)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("patch validation failed: %v", err)), nil
	}
	return toolSuccessResult(report), nil
}

func (s *mcpToolService) callExplain(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		ConfigPath string `json:"config_path"`
		Profile    string `json:"profile"`
		RuleID     string `json:"rule_id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid explain arguments")
	}
	if strings.TrimSpace(args.RuleID) == "" {
		return toolErrorResult("explain requires rule_id"), nil
	}

	confinedPath, err := confineConfigArg(ctx, s, args.ConfigPath)
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	rule, ok, err := s.resolveExplainRule(confinedPath, args.Profile, args.RuleID)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("load config: %v", err)), nil
	}
	if !ok {
		return toolErrorResult(fmt.Sprintf("unknown rule %q", args.RuleID)), nil
	}
	return toolSuccessResult(buildExplainAgentOutput(rule)), nil
}

func (s *mcpToolService) callListRules(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		ConfigPath string `json:"config_path"`
		Profile    string `json:"profile"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid list_rules arguments")
	}

	if strings.TrimSpace(args.ConfigPath) == "" && strings.TrimSpace(args.Profile) == "" {
		return toolSuccessResult(map[string]any{"rules": service.Rules()}), nil
	}
	confinedPath, err := confineConfigArg(ctx, s, args.ConfigPath)
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	cfg, err := s.loadConfig(confinedPath, args.Profile)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("load config: %v", err)), nil
	}
	return toolSuccessResult(map[string]any{"rules": service.RulesForConfig(cfg)}), nil
}
