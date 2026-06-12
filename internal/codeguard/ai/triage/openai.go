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

func (provider openAIProvider) Triage(ctx context.Context, candidates []candidate) (map[string]providerVerdict, error) {
	prompt, err := buildPrompt(candidates)
	if err != nil {
		return nil, err
	}
	body, err := provider.requestBody(prompt)
	if err != nil {
		return nil, err
	}
	resp, err := provider.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	return decodeVerdicts(resp)
}

func (provider openAIProvider) requestBody(prompt string) ([]byte, error) {
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
	return json.Marshal(payload)
}

func (provider openAIProvider) doRequest(ctx context.Context, body []byte) (*http.Response, error) {
	baseURL := provider.baseURL()
	httpClient := &http.Client{Timeout: provider.cfg.Timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if provider.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.cfg.APIKey)
	}
	return httpClient.Do(req)
}

func (provider openAIProvider) baseURL() string {
	baseURL := strings.TrimRight(provider.cfg.BaseURL, "/")
	if baseURL == "" {
		return "https://api.openai.com/v1"
	}
	return baseURL
}

func decodeVerdicts(resp *http.Response) (map[string]providerVerdict, error) {
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
	return parseVerdictText(decoded.Choices[0].Message.Content)
}

func parseVerdictText(text string) (map[string]providerVerdict, error) {
	text = strings.TrimSpace(text)
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
