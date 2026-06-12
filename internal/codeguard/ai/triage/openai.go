package triage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type openAIProvider struct {
	cfg runtimeConfig
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
}

type openAIVerdictPayload struct {
	Verdicts []struct {
		ContentHash string `json:"content_hash"`
		Decision    string `json:"decision"`
		Summary     string `json:"summary"`
	} `json:"verdicts"`
}

func (provider openAIProvider) Triage(ctx context.Context, candidates []candidate) (map[string]providerVerdict, error) {
	baseURL := strings.TrimRight(provider.cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	prompt, err := buildPrompt(candidates)
	if err != nil {
		return nil, err
	}
	payload := openAIRequest{
		Model: provider.cfg.Model,
		Messages: []openAIMessage{
			{
				Role:    "system",
				Content: "You adversarially verify static-analysis findings. Dismiss only when the finding is clearly a false positive from the provided local evidence. If uncertain, keep it. Respond with JSON only: {\"verdicts\":[{\"content_hash\":\"...\",\"decision\":\"keep|dismiss\",\"summary\":\"...\"}]}",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: provider.cfg.Timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if provider.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.cfg.APIKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai triage provider returned %s", resp.Status)
	}

	var decoded openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if len(decoded.Choices) == 0 {
		return nil, fmt.Errorf("ai triage provider returned no choices")
	}

	text := strings.TrimSpace(decoded.Choices[0].Message.Content)
	var verdictPayload openAIVerdictPayload
	if err := json.Unmarshal([]byte(text), &verdictPayload); err != nil {
		return nil, fmt.Errorf("ai triage provider returned invalid JSON verdicts: %w", err)
	}

	verdicts := make(map[string]providerVerdict, len(verdictPayload.Verdicts))
	for _, verdict := range verdictPayload.Verdicts {
		verdicts[verdict.ContentHash] = providerVerdict{
			Decision: strings.ToLower(strings.TrimSpace(verdict.Decision)),
			Summary:  strings.TrimSpace(verdict.Summary),
		}
	}
	return verdicts, nil
}

func buildPrompt(candidates []candidate) (string, error) {
	items := make([]map[string]any, 0, len(candidates))
	for _, item := range candidates {
		items = append(items, map[string]any{
			"content_hash": item.hash,
			"section":      item.sectionName,
			"rule_id":      item.finding.RuleID,
			"level":        item.finding.Level,
			"title":        item.finding.Title,
			"message":      item.finding.Message,
			"why":          item.finding.Why,
			"how_to_fix":   item.finding.HowToFix,
			"path":         item.finding.Path,
			"line":         item.finding.Line,
			"column":       item.finding.Column,
			"source":       item.snippet,
		})
	}
	data, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
