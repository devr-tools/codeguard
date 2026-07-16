package triage

import (
	"context"
	"encoding/json"
	"net/http"
)

const (
	anthropicDefaultBaseURL = "https://api.anthropic.com/v1"
	anthropicDefaultModel   = "claude-sonnet-4-6"
	anthropicVersion        = "2023-06-01"
	anthropicMaxTokens      = 4096
)

type anthropicProvider struct {
	cfg runtimeConfig
}

func (provider anthropicProvider) Triage(ctx context.Context, candidates []candidate) (map[string]providerVerdict, error) {
	return triageViaHTTP(ctx, candidates, provider.requestBody, provider.doRequest, decodeAnthropicVerdicts)
}

func (provider anthropicProvider) requestBody(prompt string) ([]byte, error) {
	payload := anthropicRequest{
		Model:     provider.cfg.Model,
		MaxTokens: anthropicMaxTokens,
		System:    triageSystemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}
	return json.Marshal(payload)
}

func (provider anthropicProvider) doRequest(ctx context.Context, body []byte) (*http.Response, error) {
	headers := map[string]string{"anthropic-version": anthropicVersion}
	if provider.cfg.APIKey != "" {
		headers["x-api-key"] = provider.cfg.APIKey
	}
	return postTriageJSON(ctx, provider.cfg, provider.baseURL()+"/messages", body, headers)
}

func (provider anthropicProvider) baseURL() string {
	return defaultBaseURL(provider.cfg.BaseURL, anthropicDefaultBaseURL)
}

func decodeAnthropicVerdicts(resp *http.Response) (map[string]providerVerdict, error) {
	return decodeJSONVerdicts(resp, func(decoder *json.Decoder) (string, error) {
		var decoded anthropicResponse
		if err := decoder.Decode(&decoded); err != nil {
			return "", err
		}
		if len(decoded.Content) == 0 {
			return "", errNoContentBlocks
		}
		return decoded.Content[0].Text, nil
	})
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}
