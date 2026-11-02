package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
	"github.com/devr-tools/codeguard/internal/codeguard/suggest"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// errConfigPathNotPermitted is returned when a caller-supplied config_path
// resolves outside the allowed roots. The message is intentionally generic so
// the HTTP transport does not become a filesystem oracle.
var errConfigPathNotPermitted = errors.New("config_path is not within an allowed root")

type mcpScanArgs struct {
	ConfigPath  string `json:"config_path"`
	Profile     string `json:"profile"`
	Mode        string `json:"mode"`
	BaseRef     string `json:"base_ref"`
	TargetPath  string `json:"target_path"`
	ChangedOnly bool   `json:"changed_only"`
	mcpFindingQuery
}

func (s *mcpToolService) callScan(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args mcpScanArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid scan arguments")
	}
	format, err := resolveFindingsFormat(raw, mcpScanOnlySummaryInputKeys...)
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	filters, err := args.normalize()
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	// A cursor pages through the cached report it was issued for, so the
	// scan-shaping arguments are not re-read and no new scan runs.
	if strings.TrimSpace(args.Cursor) != "" {
		return s.resumeFindingsResult("scan", args.Cursor, filters), nil
	}

	confinedPath, err := confineConfigArg(ctx, s, args.ConfigPath)
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	cfg, err := s.loadConfig(confinedPath, args.Profile)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("load config: %v", err)), nil
	}
	applyMinConfidence(&cfg, filters.MinConfidence)
	opts, errMsg := s.scanOptions(ctx, args)
	if errMsg != "" {
		return toolErrorResult(errMsg), nil
	}
	attachSectionProgress(ctx, cfg, &opts)

	report, err := service.RunWithOptions(ctx, cfg, opts)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("scan failed: %v", err)), nil
	}
	return s.findingsToolResult("scan", format, report, filters), nil
}

// applyMinConfidence maps the min_confidence argument onto the same config
// policy the CLI -min-confidence flag sets: it replaces
// checks.min_confidence.default while per-section overrides still apply.
func applyMinConfidence(cfg *service.Config, level string) {
	if level != "" {
		cfg.Checks.MinConfidence.Default = level
	}
}

// scanOptions validates the scan-shaping arguments at the trust boundary and
// returns the runner options, or a caller-facing error message.
func (s *mcpToolService) scanOptions(ctx context.Context, args mcpScanArgs) (service.ScanOptions, string) {
	mode, err := parseScanMode(args.Mode)
	if err != nil && strings.TrimSpace(args.Mode) != "" {
		return service.ScanOptions{}, err.Error()
	}
	if args.ChangedOnly {
		if mode == service.ScanModeFull && strings.TrimSpace(args.Mode) != "" {
			return service.ScanOptions{}, "changed_only conflicts with mode \"full\"; omit mode or set it to \"diff\""
		}
		mode = service.ScanModeDiff
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
	if err = runnersupport.ValidateBaseRef(baseRef); err != nil {
		return service.ScanOptions{}, err.Error()
	}
	targetPath := strings.TrimSpace(args.TargetPath)
	if targetPath != "" {
		targetPath, err = confinePath(allowedRoots(ctx, s), targetPath)
		if err != nil {
			return service.ScanOptions{}, err.Error()
		}
	}
	return service.ScanOptions{Mode: mode, BaseRef: baseRef, TargetPath: targetPath}, ""
}

// attachSectionProgress streams one progress notification per completed
// section when the caller supplied a progress token.
func attachSectionProgress(ctx context.Context, cfg service.Config, opts *service.ScanOptions) {
	emit := progressFrom(ctx)
	if emit == nil {
		return
	}
	total := countEnabledSections(cfg, opts.Mode)
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
		mcpFindingQuery
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid validate_patch arguments")
	}
	format, err := resolveFindingsFormat(raw)
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	filters, err := args.normalize()
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(args.Cursor) != "" {
		return s.resumeFindingsResult("validate_patch", args.Cursor, filters), nil
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
	applyMinConfidence(&cfg, filters.MinConfidence)
	report, err := service.RunPatch(ctx, cfg, args.Diff)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("patch validation failed: %v", err)), nil
	}
	return s.findingsToolResult("validate_patch", format, report, filters), nil
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
	rule, ec, ok, err := s.resolveExplainRule(confinedPath, args.Profile, args.RuleID)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("load config: %v", err)), nil
	}
	if !ok {
		return toolErrorResult(fmt.Sprintf("unknown rule %q%s", args.RuleID, suggest.Hint(args.RuleID, ruleIDs(service.RulesForConfig(ec.cfg))))), nil
	}
	return toolSuccessResult(buildExplainOutput(rule, ec)), nil
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
