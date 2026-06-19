package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	internalfix "github.com/devr-tools/codeguard/internal/codeguard/ai/fix"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

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
	var content struct {
		Apply *bool `json:"apply"`
	}
	if json.Unmarshal(res.Content, &content) == nil && content.Apply != nil {
		return *content.Apply, nil
	}
	return true, nil
}

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
		"maxTokens":      2048,
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
		rest = rest[nl+1:]
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
