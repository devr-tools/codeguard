package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// errConfigPathNotPermitted is returned when a caller-supplied config_path
// resolves outside the allowed roots. The message is intentionally generic so
// the HTTP transport does not become a filesystem oracle.
var errConfigPathNotPermitted = errors.New("config_path is not within an allowed root")

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
		return s.callValidateConfig(ctx, normalizeMCPArguments(call.Arguments))
	case "validate_patch":
		return s.callValidatePatch(ctx, normalizeMCPArguments(call.Arguments))
	case "explain":
		return s.callExplain(ctx, normalizeMCPArguments(call.Arguments))
	case "list_rules":
		return s.callListRules(ctx, normalizeMCPArguments(call.Arguments))
	case "verify_fix":
		return s.callVerifyFix(ctx, normalizeMCPArguments(call.Arguments))
	case "propose_fix":
		return s.callProposeFix(ctx, normalizeMCPArguments(call.Arguments))
	case "apply_fix":
		return s.callApplyFix(ctx, normalizeMCPArguments(call.Arguments))
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

	confinedPath, err := s.confineConfigArg(ctx, args.ConfigPath)
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

	confinedPath, err := s.confineConfigArg(ctx, args.ConfigPath)
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

	confinedPath, err := s.confineConfigArg(ctx, args.ConfigPath)
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

	confinedPath, err := s.confineConfigArg(ctx, args.ConfigPath)
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
	confinedPath, err := s.confineConfigArg(ctx, args.ConfigPath)
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	cfg, err := s.loadConfig(confinedPath, args.Profile)
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

// confineConfigArg validates a caller-supplied config_path against the allowed
// roots and returns it resolved to an absolute path. An empty argument returns
// "" so the server's trusted default config is used. The server's own default
// config path is never passed through here, so a trusted out-of-tree --config
// keeps working.
func (s *mcpToolService) confineConfigArg(ctx context.Context, configPath string) (string, error) {
	candidate := strings.TrimSpace(configPath)
	if candidate == "" {
		return "", nil
	}
	return confinePath(s.allowedRoots(ctx), candidate)
}

// rootsFetchTimeout bounds the server→client roots/list round trip done while
// resolving a config path, so a config load never blocks on a slow client.
const rootsFetchTimeout = 10 * time.Second

// allowedRoots is the set of directories a caller-supplied config_path may live
// under: the directory of the server's default config, the working directory,
// any roots advertised by the connected client (via the roots capability), and
// any roots injected explicitly (tests).
func (s *mcpToolService) allowedRoots(ctx context.Context) []string {
	roots := make([]string, 0, 4)
	if def := strings.TrimSpace(s.defaultConfigPath); def != "" {
		roots = append(roots, configDirOf(def))
	}
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots, wd)
	}
	if caller := clientCallerFrom(ctx); caller != nil && caller.supports("roots") {
		rctx, cancel := context.WithTimeout(ctx, rootsFetchTimeout)
		if clientRoots, err := caller.listRoots(rctx); err == nil {
			for _, root := range clientRoots {
				if p := rootURIToPath(root.URI); p != "" {
					roots = append(roots, p)
				}
			}
		}
		cancel()
	}
	roots = append(roots, clientRootsFrom(ctx)...)
	return roots
}

// rootURIToPath converts a roots entry to a filesystem path. MCP roots are
// file:// URIs; a bare path is accepted as-is.
func rootURIToPath(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return ""
	}
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	if strings.Contains(uri, "://") {
		return "" // non-file scheme is not a local path
	}
	return uri
}

// configDirOf returns the directory that should anchor confinement for a config
// path: the path itself if it is a directory, otherwise its parent.
func configDirOf(path string) string {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path
	}
	return filepath.Dir(path)
}

// confinePath resolves candidate to an absolute, cleaned path and requires it to
// sit within one of allowedRoots. Symlinks are resolved best-effort to defeat
// link escapes; on resolution failure it falls back to the cleaned path.
func confinePath(allowedRoots []string, candidate string) (string, error) {
	abs := resolvePath(candidate)
	for _, root := range allowedRoots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		rootAbs := resolvePath(root)
		if abs == rootAbs || strings.HasPrefix(abs, rootAbs+string(os.PathSeparator)) {
			return abs, nil
		}
	}
	return "", errConfigPathNotPermitted
}

func resolvePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
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
