package triage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
	baseURL := strings.TrimRight(provider.cfg.BaseURL, "/")
	if baseURL == "" {
		return anthropicDefaultBaseURL
	}
	return baseURL
}

func decodeAnthropicVerdicts(resp *http.Response) (map[string]providerVerdict, error) {
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai triage provider returned %s", resp.Status)
	}

	var decoded anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if len(decoded.Content) == 0 {
		return nil, fmt.Errorf("ai triage provider returned no content blocks")
	}
	return parseVerdictText(decoded.Content[0].Text)
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
