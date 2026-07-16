package triage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const triageSystemPrompt = "You adversarially verify static-analysis findings. Dismiss only when the finding is clearly a false positive from the provided local evidence. If uncertain, keep it. Respond with JSON only: {\"verdicts\":[{\"content_hash\":\"...\",\"decision\":\"keep|dismiss\",\"summary\":\"...\"}]}"

type openAIProvider struct {
	cfg runtimeConfig
}

func (provider openAIProvider) Triage(ctx context.Context, candidates []candidate) (map[string]providerVerdict, error) {
	return triageViaHTTP(ctx, candidates, provider.requestBody, provider.doRequest, decodeVerdicts)
}

func (provider openAIProvider) requestBody(prompt string) ([]byte, error) {
	payload := openAIRequest{
		Model: provider.cfg.Model,
		Messages: []openAIMessage{
			{
				Role:    "system",
				Content: triageSystemPrompt,
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
	headers := map[string]string{}
	if provider.cfg.APIKey != "" {
		headers["Authorization"] = "Bearer " + provider.cfg.APIKey
	}
	return postTriageJSON(ctx, provider.cfg, provider.baseURL()+"/chat/completions", body, headers)
}

func (provider openAIProvider) baseURL() string {
	return defaultBaseURL(provider.cfg.BaseURL, "https://api.openai.com/v1")
}

func decodeVerdicts(resp *http.Response) (map[string]providerVerdict, error) {
	return decodeJSONVerdicts(resp, func(decoder *json.Decoder) (string, error) {
		var decoded openAIResponse
		if err := decoder.Decode(&decoded); err != nil {
			return "", err
		}
		if len(decoded.Choices) == 0 {
			return "", errNoChoices
		}
		return decoded.Choices[0].Message.Content, nil
	})
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
