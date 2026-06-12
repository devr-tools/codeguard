package codeguard_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	airuntime "github.com/devr-tools/codeguard/internal/codeguard/ai/runtime"
	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestApplyDefaultsKeepsAnthropicProviderUnflavored(t *testing.T) {
	cfg := core.Config{}
	cfg.AI.Provider.Type = "anthropic"
	config.ApplyDefaults(&cfg)

	if cfg.AI.Provider.Model != "" {
		t.Fatalf("model = %q, want OpenAI default not applied to anthropic provider", cfg.AI.Provider.Model)
	}
	if cfg.AI.Provider.BaseURL != "" {
		t.Fatalf("base URL = %q, want OpenAI default not applied to anthropic provider", cfg.AI.Provider.BaseURL)
	}
	if cfg.AI.Provider.APIKeyEnv != "" {
		t.Fatalf("api key env = %q, want OpenAI default not applied to anthropic provider", cfg.AI.Provider.APIKeyEnv)
	}
}

func TestAnthropicRuntimeProviderSendsMessagesRequest(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")

	var gotPath, gotAPIKey, gotVersion string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"  hello from claude  "}]}`))
	}))
	defer server.Close()

	provider, ok, err := airuntime.BuildProvider(core.AIProviderConfig{
		Type:    "anthropic",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("BuildProvider: %v", err)
	}
	if !ok {
		t.Fatal("expected anthropic provider to be available with ANTHROPIC_API_KEY set")
	}
	if provider.Name() != "anthropic" {
		t.Fatalf("provider name = %q, want anthropic", provider.Name())
	}

	resp, err := provider.Evaluate(context.Background(), airuntime.Request{
		Kind:   "autofix",
		System: "system prompt",
		Prompt: "user prompt",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if resp.Raw != "hello from claude" {
		t.Fatalf("response raw = %q, want trimmed content[0].text", resp.Raw)
	}
	if gotPath != "/messages" {
		t.Fatalf("request path = %q, want /messages", gotPath)
	}
	if gotAPIKey != "test-anthropic-key" {
		t.Fatalf("x-api-key = %q", gotAPIKey)
	}
	if gotVersion != "2023-06-01" {
		t.Fatalf("anthropic-version = %q", gotVersion)
	}
	if gotBody["model"] != "claude-sonnet-4-6" {
		t.Fatalf("model = %v, want default claude-sonnet-4-6", gotBody["model"])
	}
	if tokens, ok := gotBody["max_tokens"].(float64); !ok || tokens <= 0 {
		t.Fatalf("max_tokens = %v, want a positive value", gotBody["max_tokens"])
	}
	if gotBody["system"] != "system prompt" {
		t.Fatalf("system = %v", gotBody["system"])
	}
}

func TestAnthropicRuntimeProviderUnavailableWithoutKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")

	_, ok, err := airuntime.BuildProvider(core.AIProviderConfig{Type: "anthropic"})
	if err != nil {
		t.Fatalf("BuildProvider: %v", err)
	}
	if ok {
		t.Fatal("expected anthropic provider to be unavailable without an API key")
	}
}

func TestAnthropicRuntimeProviderRetriesRateLimit(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	t.Setenv("CODEGUARD_AI_RETRY_BASE_DELAY", "1ms")

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"retried"}]}`))
	}))
	defer server.Close()

	provider, ok, err := airuntime.BuildProvider(core.AIProviderConfig{
		Type:    "anthropic",
		BaseURL: server.URL,
	})
	if err != nil || !ok {
		t.Fatalf("BuildProvider: ok=%v err=%v", ok, err)
	}

	resp, err := provider.Evaluate(context.Background(), airuntime.Request{Kind: "autofix", Prompt: "p"})
	if err != nil {
		t.Fatalf("Evaluate after 429: %v", err)
	}
	if resp.Raw != "retried" {
		t.Fatalf("response raw = %q", resp.Raw)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 requests (429 then 200), got %d", got)
	}
}

func TestOpenAIRuntimeProviderRetriesServerError(t *testing.T) {
	t.Setenv("TEST_OPENAI_KEY", "test-openai-key")
	t.Setenv("CODEGUARD_AI_RETRY_BASE_DELAY", "1ms")

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"retried"}}]}`))
	}))
	defer server.Close()

	provider, ok, err := airuntime.BuildProvider(core.AIProviderConfig{
		Type:      "openai",
		BaseURL:   server.URL,
		APIKeyEnv: "TEST_OPENAI_KEY",
	})
	if err != nil || !ok {
		t.Fatalf("BuildProvider: ok=%v err=%v", ok, err)
	}

	resp, err := provider.Evaluate(context.Background(), airuntime.Request{Kind: "triage", Prompt: "p"})
	if err != nil {
		t.Fatalf("Evaluate after 500: %v", err)
	}
	if resp.Raw != "retried" {
		t.Fatalf("response raw = %q", resp.Raw)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 requests (500 then 200), got %d", got)
	}
}

func TestRuntimeProviderRetriesExhaustGracefully(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	t.Setenv("CODEGUARD_AI_RETRY_BASE_DELAY", "1ms")
	t.Setenv("CODEGUARD_AI_MAX_RETRIES", "1")

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	provider, ok, err := airuntime.BuildProvider(core.AIProviderConfig{
		Type:    "anthropic",
		BaseURL: server.URL,
	})
	if err != nil || !ok {
		t.Fatalf("BuildProvider: ok=%v err=%v", ok, err)
	}

	_, err = provider.Evaluate(context.Background(), airuntime.Request{Kind: "autofix", Prompt: "p"})
	if err == nil {
		t.Fatal("expected error after retries are exhausted")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected initial attempt plus 1 retry, got %d requests", got)
	}
}
