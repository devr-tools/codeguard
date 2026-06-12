package fix

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	airuntime "github.com/devr-tools/codeguard/internal/codeguard/ai/runtime"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type aiGenerator struct {
	session *airuntime.Session
}

func NewAIGenerator(cfg core.AIConfig) (Generator, bool, error) {
	session, err := airuntime.NewSession(cfg, core.ScanOptions{EnableAI: true})
	if err != nil {
		return nil, false, err
	}
	if session == nil || !session.Enabled() {
		return nil, false, nil
	}
	return aiGenerator{session: session}, true, nil
}

func (g aiGenerator) GenerateFix(ctx context.Context, input GenerateInput) (Candidate, error) {
	requestPayload, err := json.Marshal(map[string]any{
		"finding": map[string]any{
			"rule_id":    input.Finding.RuleID,
			"title":      input.Finding.Title,
			"message":    input.Finding.Message,
			"why":        input.Finding.Why,
			"how_to_fix": input.Finding.HowToFix,
			"path":       input.Finding.Path,
			"line":       input.Finding.Line,
			"column":     input.Finding.Column,
		},
		"analysis":     input.Analysis,
		"instructions": input.Instructions,
		"excerpt":      sourceExcerpt(input.Config, input.Finding),
	})
	if err != nil {
		return Candidate{}, err
	}
	resp, _, err := g.session.EvaluateCached(ctx, airuntime.Request{
		Kind:      "autofix",
		System:    "Return JSON only with the shape {\"summary\":string,\"diff\":string}. The diff must be a valid unified diff against the current repository state and must only fix the reported issue.",
		Prompt:    "Generate a minimal patch that fixes the finding and keeps unrelated code unchanged.",
		InputJSON: string(requestPayload),
	})
	if err != nil {
		return Candidate{}, err
	}
	var candidate Candidate
	if err := json.Unmarshal([]byte(resp.Raw), &candidate); err != nil {
		return Candidate{}, fmt.Errorf("ai fix generator returned invalid JSON: %w", err)
	}
	if strings.TrimSpace(candidate.Diff) == "" {
		return Candidate{}, fmt.Errorf("ai fix generator returned an empty diff")
	}
	return candidate, nil
}

func sourceExcerpt(cfg core.Config, finding core.Finding) string {
	if finding.Path == "" {
		return ""
	}
	for _, target := range cfg.Targets {
		fullPath := filepath.Join(target.Path, filepath.FromSlash(finding.Path))
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		if finding.Line <= 0 || finding.Line > len(lines) {
			return string(data)
		}
		start := maxInt(finding.Line-4, 1)
		end := minInt(finding.Line+4, len(lines))
		return strings.Join(lines[start-1:end], "\n")
	}
	return ""
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
