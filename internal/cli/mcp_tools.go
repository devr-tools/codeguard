package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func (s *mcpToolService) callToolWithContext(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &call); err != nil {
		return nil, fmt.Errorf("invalid tool call")
	}

	switch strings.TrimSpace(call.Name) {
	case "scan":
		return s.callScan(ctx, normalizeMCPArguments(call.Arguments))
	case "validate_config":
		return s.callValidateConfig(normalizeMCPArguments(call.Arguments))
	case "validate_patch":
		return s.callValidatePatch(ctx, normalizeMCPArguments(call.Arguments))
	case "explain":
		return s.callExplain(normalizeMCPArguments(call.Arguments))
	case "list_rules":
		return s.callListRules(normalizeMCPArguments(call.Arguments))
	default:
		return nil, fmt.Errorf("unknown tool: %s", call.Name)
	}
}

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

	cfg, err := s.loadConfig(args.ConfigPath, args.Profile)
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

	report, err := service.RunWithOptions(ctx, cfg, service.ScanOptions{Mode: mode, BaseRef: baseRef})
	if err != nil {
		return toolErrorResult(fmt.Sprintf("scan failed: %v", err)), nil
	}
	return toolSuccessResult(report), nil
}

func (s *mcpToolService) callValidateConfig(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		ConfigPath string `json:"config_path"`
		Profile    string `json:"profile"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid validate_config arguments")
	}

	cfg, err := s.loadConfig(args.ConfigPath, args.Profile)
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

	cfg, err := s.loadConfig(args.ConfigPath, args.Profile)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("load config: %v", err)), nil
	}
	report, err := service.RunPatch(ctx, cfg, args.Diff)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("patch validation failed: %v", err)), nil
	}
	return toolSuccessResult(report), nil
}

func (s *mcpToolService) callExplain(raw json.RawMessage) (map[string]any, error) {
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

	rule, ok, err := s.resolveExplainRule(args.ConfigPath, args.Profile, args.RuleID)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("load config: %v", err)), nil
	}
	if !ok {
		return toolErrorResult(fmt.Sprintf("unknown rule %q", args.RuleID)), nil
	}
	return toolSuccessResult(buildExplainAgentOutput(rule)), nil
}

func (s *mcpToolService) callListRules(raw json.RawMessage) (map[string]any, error) {
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
	cfg, err := s.loadConfig(args.ConfigPath, args.Profile)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("load config: %v", err)), nil
	}
	return toolSuccessResult(map[string]any{"rules": service.RulesForConfig(cfg)}), nil
}

func (s *mcpToolService) loadConfig(configPath string, profile string) (service.Config, error) {
	path := strings.TrimSpace(configPath)
	if path == "" {
		path = s.defaultConfigPath
	}
	overrideProfile := strings.TrimSpace(profile)
	if overrideProfile == "" {
		overrideProfile = strings.TrimSpace(s.defaultProfile)
	}
	return loadConfigWithProfile(path, overrideProfile)
}

func (s *mcpToolService) resolveExplainRule(configPath string, profile string, ruleID string) (service.RuleMetadata, bool, error) {
	if strings.TrimSpace(configPath) != "" {
		cfg, err := s.loadConfig(configPath, profile)
		if err != nil {
			return service.RuleMetadata{}, false, err
		}
		rule, ok := service.ExplainRuleForConfig(cfg, ruleID)
		return rule, ok, nil
	}

	rule, ok := service.ExplainRule(ruleID)
	if strings.TrimSpace(profile) == "" {
		return rule, ok, nil
	}
	cfg, err := s.loadConfig("", profile)
	if err != nil {
		return service.RuleMetadata{}, false, err
	}
	rule, ok = service.ExplainRuleForConfig(cfg, ruleID)
	return rule, ok, nil
}
