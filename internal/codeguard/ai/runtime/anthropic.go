package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const (
	anthropicDefaultBaseURL = "https://api.anthropic.com/v1"
	anthropicDefaultModel   = "claude-sonnet-4-6"
	anthropicVersion        = "2023-06-01"
	anthropicAPIKeyEnv      = "ANTHROPIC_API_KEY"
	anthropicMaxTokens      = 4096
)

type anthropicProvider struct {
	baseURL string
	model   string
	apiKey  string
}

func anthropicProviderFromConfig(cfg core.AIProviderConfig) (Provider, bool) {
	keyEnv := strings.TrimSpace(cfg.APIKeyEnv)
	if keyEnv == "" {
		keyEnv = anthropicAPIKeyEnv
	}
	apiKey := strings.TrimSpace(os.Getenv(keyEnv))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv(anthropicAPIKeyEnv))
	}
	if apiKey == "" {
		return nil, false
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = anthropicDefaultBaseURL
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = anthropicDefaultModel
	}
	return anthropicProvider{baseURL: baseURL, model: model, apiKey: apiKey}, true
}

func (p anthropicProvider) Name() string { return "anthropic" }

func (p anthropicProvider) Evaluate(ctx context.Context, req Request) (Response, error) {
	body := anthropicRequest{
		Model:     p.model,
		MaxTokens: anthropicMaxTokens,
		System:    strings.TrimSpace(req.System),
		Messages: []anthropicMessage{
			{Role: "user", Content: openAIUserPrompt(req)},
		},
	}
	respData, err := postProviderJSON(ctx, p.Name(), p.baseURL+"/messages", map[string]string{
		"x-api-key":         p.apiKey,
		"anthropic-version": anthropicVersion,
	}, body)
	if err != nil {
		return Response{}, err
	}
	text, err := anthropicResponseText(respData)
	if err != nil {
		return Response{}, fmt.Errorf("ai provider %s: %w", p.Name(), err)
	}
	return Response{Raw: text}, nil
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

func anthropicResponseText(data []byte) (string, error) {
	var payload struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	if len(payload.Content) == 0 {
		return "", fmt.Errorf("returned no content blocks")
	}
	return strings.TrimSpace(payload.Content[0].Text), nil
}
