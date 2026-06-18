package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/safehttp"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/trust"
)

const (
	providerTimeoutEnv     = "CODEGUARD_AI_TIMEOUT"
	defaultProviderTimeout = 30 * time.Second
)

func BuildProvider(cfg core.AIProviderConfig) (Provider, bool, error) {
	switch strings.TrimSpace(strings.ToLower(cfg.Type)) {
	case "", "openai":
		// cfg.BaseURL comes from repository config, so it is an untrusted source.
		if err := safehttp.ValidateProviderURL(cfg.BaseURL, false); err != nil {
			return nil, false, err
		}
		provider, ok := openAIProviderFromConfig(cfg)
		return provider, ok, nil
	case "anthropic":
		if err := safehttp.ValidateProviderURL(cfg.BaseURL, false); err != nil {
			return nil, false, err
		}
		provider, ok := anthropicProviderFromConfig(cfg)
		return provider, ok, nil
	case "command":
		if strings.TrimSpace(cfg.Command) == "" {
			return nil, false, nil
		}
		if err := trust.GuardConfigCommand("ai.provider", cfg.Command); err != nil {
			return nil, false, err
		}
		return commandProvider{command: cfg.Command, args: append([]string(nil), cfg.Args...)}, true, nil
	default:
		return nil, false, fmt.Errorf("unsupported ai provider %q", cfg.Type)
	}
}

type openAIProvider struct {
	baseURL string
	model   string
	apiKey  string
}

func openAIProviderFromConfig(cfg core.AIProviderConfig) (Provider, bool) {
	keyEnv := strings.TrimSpace(cfg.APIKeyEnv)
	if keyEnv == "" {
		keyEnv = "OPENAI_API_KEY"
	}
	apiKey := strings.TrimSpace(os.Getenv(keyEnv))
	if apiKey == "" {
		return nil, false
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "gpt-5"
	}
	return openAIProvider{baseURL: baseURL, model: model, apiKey: apiKey}, true
}

func (p openAIProvider) Name() string { return "openai" }

func (p openAIProvider) Evaluate(ctx context.Context, req Request) (Response, error) {
	body := map[string]any{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": strings.TrimSpace(req.System)},
			{"role": "user", "content": openAIUserPrompt(req)},
		},
	}
	respData, err := postProviderJSON(ctx, p.Name(), p.baseURL+"/chat/completions", map[string]string{
		"Authorization": "Bearer " + p.apiKey,
	}, body)
	if err != nil {
		return Response{}, err
	}
	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respData, &payload); err != nil {
		return Response{}, err
	}
	if len(payload.Choices) == 0 {
		return Response{}, fmt.Errorf("ai provider %s returned no choices", p.Name())
	}
	return Response{Raw: strings.TrimSpace(payload.Choices[0].Message.Content)}, nil
}

type commandProvider struct {
	command string
	args    []string
}

func (p commandProvider) Name() string { return "command" }

func (p commandProvider) Evaluate(ctx context.Context, req Request) (Response, error) {
	cmd := exec.CommandContext(ctx, p.command, p.args...)
	data, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil {
		return Response{}, err
	}
	return Response{Raw: strings.TrimSpace(string(out))}, nil
}

func providerHTTPClient() *http.Client {
	timeout := defaultProviderTimeout
	if raw := strings.TrimSpace(os.Getenv(providerTimeoutEnv)); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			timeout = parsed
		}
	}
	return safehttp.Client(timeout)
}

func openAIUserPrompt(req Request) string {
	if strings.TrimSpace(req.InputJSON) == "" {
		return strings.TrimSpace(req.Prompt)
	}
	return strings.TrimSpace(req.Prompt) + "\n\nInput JSON:\n" + req.InputJSON
}
