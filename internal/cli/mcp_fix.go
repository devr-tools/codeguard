package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	internalfix "github.com/devr-tools/codeguard/internal/codeguard/ai/fix"
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
	var args fixToolArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid verify_fix arguments")
	}
	if strings.TrimSpace(args.Diff) == "" {
		return toolErrorResult("verify_fix requires a unified diff"), nil
	}

	confinedPath, err := s.confineConfigArg(ctx, args.ConfigPath)
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	cfg, err := s.loadConfig(confinedPath, args.Profile)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("load config: %v", err)), nil
	}

	result, err := service.VerifyFix(ctx, cfg, args.Finding, service.FixCandidate{Diff: args.Diff}, args.options())
	if err != nil {
		data := map[string]any{
			"verified":       false,
			"error":          err.Error(),
			"attempted_diff": args.Diff,
		}
		// Surface what still fails so an agent can iterate, by re-evaluating the
		// diff against policy (does not mutate the working tree).
		if report, perr := service.RunPatch(ctx, cfg, args.Diff); perr == nil {
			data["remaining_findings"] = report
		}
		return toolErrorResultData(fmt.Sprintf("fix did not verify: %v", err), data), nil
	}
	return toolSuccessResult(result), nil
}

func (s *mcpToolService) callProposeFix(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args fixToolArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid propose_fix arguments")
	}
	if strings.TrimSpace(args.Finding.RuleID) == "" && strings.TrimSpace(args.Finding.Message) == "" {
		return toolErrorResult("propose_fix requires a finding to fix"), nil
	}

	confinedPath, err := s.confineConfigArg(ctx, args.ConfigPath)
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	cfg, err := s.loadConfig(confinedPath, args.Profile)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("load config: %v", err)), nil
	}

	generator, err := s.resolveFixGenerator(ctx, cfg)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("initialize fix generator: %v", err)), nil
	}
	if generator == nil {
		return toolErrorResult("no fix generator available: the client does not support sampling and no AI provider is configured"), nil
	}

	result, err := service.GenerateVerifiedFix(ctx, service.FixGenerateRequest{
		Config:    cfg,
		Finding:   args.Finding,
		Analysis:  firstNonEmpty(args.Finding.Why, args.Finding.Message),
		Generator: generator,
		Options:   args.options(),
	})
	if err != nil {
		return toolErrorResultData(fmt.Sprintf("generate verified fix: %v", err), map[string]any{
			"verified": false,
			"error":    err.Error(),
		}), nil
	}
	return toolSuccessResult(result), nil
}

// callApplyFix verifies a candidate diff and, only if it passes, writes it to
// the working tree. When the client supports elicitation it first asks the user
// to confirm. This is the one tool that mutates the repo (destructiveHint).
func (s *mcpToolService) callApplyFix(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args fixToolArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid apply_fix arguments")
	}
	if strings.TrimSpace(args.Diff) == "" {
		return toolErrorResult("apply_fix requires a unified diff"), nil
	}

	confinedPath, err := s.confineConfigArg(ctx, args.ConfigPath)
	if err != nil {
		return toolErrorResult(err.Error()), nil
	}
	cfg, err := s.loadConfig(confinedPath, args.Profile)
	if err != nil {
		return toolErrorResult(fmt.Sprintf("load config: %v", err)), nil
	}

	result, err := service.VerifyFix(ctx, cfg, args.Finding, service.FixCandidate{Diff: args.Diff}, args.options())
	if err != nil {
		data := map[string]any{"verified": false, "applied": false, "error": err.Error(), "attempted_diff": args.Diff}
		if report, perr := service.RunPatch(ctx, cfg, args.Diff); perr == nil {
			data["remaining_findings"] = report
		}
		return toolErrorResultData(fmt.Sprintf("fix did not verify, not applied: %v", err), data), nil
	}

	// Confirm with the user before writing, when the client supports it.
	if caller := clientCallerFrom(ctx); caller != nil && caller.supports("elicitation") {
		accepted, err := confirmApply(ctx, caller, result.ChangedFiles)
		if err != nil {
			return toolErrorResult(fmt.Sprintf("confirmation failed: %v", err)), nil
		}
		if !accepted {
			return toolSuccessResult(map[string]any{
				"applied":       false,
				"declined":      true,
				"diff":          result.Diff,
				"changed_files": result.ChangedFiles,
			}), nil
		}
	}

	if err := runnersupport.ApplyUnifiedDiff(cfg, result.Diff); err != nil {
		return toolErrorResult(fmt.Sprintf("verified fix failed to apply to the working tree: %v", err)), nil
	}
	return toolSuccessResult(map[string]any{
		"applied":       true,
		"diff":          result.Diff,
		"summary":       result.Summary,
		"changed_files": result.ChangedFiles,
		"report":        result.Report,
		"test_results":  result.TestResults,
	}), nil
}

func confirmApply(ctx context.Context, caller clientCaller, changedFiles []string) (bool, error) {
	message := fmt.Sprintf("Apply the verified fix to %d file(s): %s?", len(changedFiles), strings.Join(changedFiles, ", "))
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"apply": map[string]any{"type": "boolean", "description": "Apply the verified fix to the working tree"},
		},
		"required": []string{"apply"},
	}
	res, err := caller.elicit(ctx, message, schema)
	if err != nil {
		return false, err
	}
	if !res.accepted() {
		return false, nil
	}
	// If the host returned content, honor an explicit apply:false.
	var content struct {
		Apply *bool `json:"apply"`
	}
	if json.Unmarshal(res.Content, &content) == nil && content.Apply != nil {
		return *content.Apply, nil
	}
	return true, nil
}

// resolveFixGenerator picks a fix generator: the connected client's LLM via MCP
// sampling when the client supports it (no API key needed), otherwise a
// configured AI provider. Returns (nil, nil) when neither is available.
func (s *mcpToolService) resolveFixGenerator(ctx context.Context, cfg service.Config) (internalfix.Generator, error) {
	if caller := clientCallerFrom(ctx); caller != nil && caller.supports("sampling") {
		return samplingGenerator{caller: caller}, nil
	}
	generator, available, err := internalfix.NewAIGenerator(cfg.AI)
	if err != nil {
		return nil, err
	}
	if !available {
		return nil, nil
	}
	return generator, nil
}

// samplingGenerator generates a fix candidate by asking the connected client's
// LLM through MCP sampling, so no server-side AI provider or API key is needed.
type samplingGenerator struct {
	caller clientCaller
}

func (g samplingGenerator) GenerateFix(ctx context.Context, input internalfix.GenerateInput) (internalfix.Candidate, error) {
	params := map[string]any{
		"systemPrompt": "You are a precise code-fixing assistant. Return ONLY a valid unified diff (git format) that fixes the reported finding and changes nothing unrelated. Do not include any prose, explanation, or markdown fences.",
		"messages": []map[string]any{{
			"role": "user",
			"content": map[string]any{
				"type": "text",
				"text": buildSamplingFixPrompt(input),
			},
		}},
		"maxTokens": 2048,
		// Ask the host to attach this server's context (the repo it is scanning)
		// so the model can read the affected files, and bias model selection
		// toward capability over speed for a correct patch.
		"includeContext": "thisServer",
		"modelPreferences": map[string]any{
			"intelligencePriority": 0.8,
			"speedPriority":        0.3,
		},
	}
	raw, err := g.caller.sampleMessage(ctx, params)
	if err != nil {
		return internalfix.Candidate{}, err
	}
	text, err := samplingResultText(raw)
	if err != nil {
		return internalfix.Candidate{}, err
	}
	candidate := candidateFromText(text)
	if strings.TrimSpace(candidate.Diff) == "" {
		return internalfix.Candidate{}, fmt.Errorf("sampling response did not contain a diff")
	}
	return candidate, nil
}

func buildSamplingFixPrompt(input internalfix.GenerateInput) string {
	var b strings.Builder
	b.WriteString("Fix this codeguard finding by producing a minimal unified diff.\n\n")
	if input.Finding.RuleID != "" {
		fmt.Fprintf(&b, "Rule: %s\n", input.Finding.RuleID)
	}
	if input.Finding.Path != "" {
		fmt.Fprintf(&b, "File: %s", input.Finding.Path)
		if input.Finding.Line > 0 {
			fmt.Fprintf(&b, ":%d", input.Finding.Line)
		}
		b.WriteString("\n")
	}
	if input.Finding.Message != "" {
		fmt.Fprintf(&b, "Issue: %s\n", input.Finding.Message)
	}
	if analysis := firstNonEmpty(input.Analysis, input.Finding.Why); analysis != "" {
		fmt.Fprintf(&b, "Why: %s\n", analysis)
	}
	if input.Finding.HowToFix != "" {
		fmt.Fprintf(&b, "How to fix: %s\n", input.Finding.HowToFix)
	}
	if input.Instructions != "" {
		fmt.Fprintf(&b, "\n%s\n", input.Instructions)
	}
	b.WriteString("\nReturn only the unified diff.")
	return b.String()
}

// samplingResultText extracts the assistant text from a sampling/createMessage
// result, tolerating both a single content block and a content array.
func samplingResultText(raw json.RawMessage) (string, error) {
	var single struct {
		Content struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &single); err == nil && strings.TrimSpace(single.Content.Text) != "" {
		return single.Content.Text, nil
	}
	var multi struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &multi); err == nil {
		for _, block := range multi.Content {
			if strings.TrimSpace(block.Text) != "" {
				return block.Text, nil
			}
		}
	}
	return "", fmt.Errorf("sampling response had no text content")
}

// candidateFromText turns a model reply into a fix candidate, accepting a raw
// diff, a {"summary","diff"} JSON object, or a fenced code block.
func candidateFromText(text string) internalfix.Candidate {
	trimmed := strings.TrimSpace(text)
	var asJSON internalfix.Candidate
	if json.Unmarshal([]byte(trimmed), &asJSON) == nil && strings.TrimSpace(asJSON.Diff) != "" {
		return asJSON
	}
	if fenced := extractFencedBlock(trimmed); fenced != "" {
		return internalfix.Candidate{Diff: fenced}
	}
	if looksLikeDiff(trimmed) {
		return internalfix.Candidate{Diff: trimmed}
	}
	return internalfix.Candidate{}
}

func extractFencedBlock(text string) string {
	start := strings.Index(text, "```")
	if start < 0 {
		return ""
	}
	rest := text[start+3:]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[nl+1:] // drop the optional language tag line
	}
	end := strings.Index(rest, "```")
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}

func looksLikeDiff(text string) bool {
	return strings.Contains(text, "@@") || strings.HasPrefix(text, "diff ") || strings.HasPrefix(text, "--- ")
}
