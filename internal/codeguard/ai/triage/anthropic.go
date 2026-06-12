package triage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/httpretry"
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
	return decodeAnthropicVerdicts(resp)
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
	baseURL := provider.baseURL()
	httpClient := &http.Client{Timeout: provider.cfg.Timeout}
	return httpretry.Do(ctx, httpClient, httpretry.FromEnv(), func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/messages", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("anthropic-version", anthropicVersion)
		if provider.cfg.APIKey != "" {
			req.Header.Set("x-api-key", provider.cfg.APIKey)
		}
		return req, nil
	})
}

func (provider anthropicProvider) baseURL() string {
	baseURL := strings.TrimRight(provider.cfg.BaseURL, "/")
	if baseURL == "" {
		return anthropicDefaultBaseURL
	}
	return baseURL
}

func decodeAnthropicVerdicts(resp *http.Response) (map[string]providerVerdict, error) {
	defer resp.Body.Close()
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
