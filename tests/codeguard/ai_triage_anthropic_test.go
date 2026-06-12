package codeguard_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

const triageFixtureSource = `package sample

func buildClient() error {
	err := doThing()
	_ = err
	return nil
}

func doThing() error { return nil }
`

func triageFixtureConfig(t *testing.T, root string) codeguard.Config {
	t.Helper()
	writeArtifactFile(t, filepath.Join(root, "service.go"), triageFixtureSource)
	cacheEnabled := true
	return codeguard.Config{
		Name: "anthropic-triage",
		Targets: []codeguard.TargetConfig{{
			Name:     "go-target",
			Path:     root,
			Language: "go",
		}},
		Checks: codeguard.CheckConfig{Quality: true},
		Output: codeguard.OutputConfig{Format: "json"},
		Cache: codeguard.CacheConfig{
			Enabled: &cacheEnabled,
			Path:    filepath.Join(root, ".codeguard", "cache.json"),
		},
	}
}

// anthropicTriageHandler answers the Anthropic Messages API with a dismiss
// verdict for every candidate found in the request prompt.
func anthropicTriageHandler(t *testing.T, gotHeaders *http.Header) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if gotHeaders != nil {
			*gotHeaders = r.Header.Clone()
		}
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode triage request: %v", err)
		}
		var items []map[string]any
		if len(req.Messages) > 0 {
			if err := json.Unmarshal([]byte(req.Messages[len(req.Messages)-1].Content), &items); err != nil {
				t.Errorf("decode candidate payload: %v", err)
			}
		}
		verdicts := make([]map[string]string, 0, len(items))
		for _, item := range items {
			hash, _ := item["content_hash"].(string)
			verdicts = append(verdicts, map[string]string{
				"content_hash": hash,
				"decision":     "dismiss",
				"summary":      "intentional fixture suppression",
			})
		}
		text, _ := json.Marshal(map[string]any{"verdicts": verdicts})
		payload, _ := json.Marshal(map[string]any{
			"content": []map[string]string{{"type": "text", "text": string(text)}},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}
}

func TestHybridTriageAnthropicProviderDismissesFinding(t *testing.T) {
	root := t.TempDir()
	var gotHeaders http.Header
	server := httptest.NewServer(anthropicTriageHandler(t, &gotHeaders))
	defer server.Close()

	t.Setenv("CODEGUARD_AI_TRIAGE_PROVIDER", "anthropic")
	t.Setenv("CODEGUARD_AI_TRIAGE_BASE_URL", server.URL)
	t.Setenv("CODEGUARD_AI_TRIAGE_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "fallback-anthropic-key")

	report, err := codeguard.Run(context.Background(), triageFixtureConfig(t, root))
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if findings := findSection(t, report, "Code Quality").Findings; len(findings) != 0 {
		t.Fatalf("expected anthropic triage to dismiss findings, got %+v", findings)
	}
	artifact := findAIAnalysisArtifact(report)
	if artifact == nil || artifact.AIAnalysis == nil {
		t.Fatalf("expected ai_analysis artifact, got %#v", report.Artifacts)
	}
	if artifact.AIAnalysis.Provider != "anthropic:claude-sonnet-4-6" {
		t.Fatalf("provider = %q, want anthropic with default model", artifact.AIAnalysis.Provider)
	}
	if len(artifact.AIAnalysis.Verdicts) != 1 || artifact.AIAnalysis.Verdicts[0].Status != "dismissed" {
		t.Fatalf("expected one dismissed verdict, got %#v", artifact.AIAnalysis.Verdicts)
	}
	if gotHeaders.Get("x-api-key") != "fallback-anthropic-key" {
		t.Fatalf("x-api-key = %q, want ANTHROPIC_API_KEY fallback", gotHeaders.Get("x-api-key"))
	}
	if gotHeaders.Get("anthropic-version") != "2023-06-01" {
		t.Fatalf("anthropic-version = %q", gotHeaders.Get("anthropic-version"))
	}
}

func TestHybridTriageAnthropicProviderRetriesRateLimit(t *testing.T) {
	root := t.TempDir()
	var calls atomic.Int64
	handler := anthropicTriageHandler(t, nil)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		handler(w, r)
	}))
	defer server.Close()

	t.Setenv("CODEGUARD_AI_TRIAGE_PROVIDER", "anthropic")
	t.Setenv("CODEGUARD_AI_TRIAGE_MODEL", "claude-sonnet-4-6")
	t.Setenv("CODEGUARD_AI_TRIAGE_BASE_URL", server.URL)
	t.Setenv("CODEGUARD_AI_TRIAGE_API_KEY", "triage-key")
	t.Setenv("CODEGUARD_AI_RETRY_BASE_DELAY", "1ms")

	report, err := codeguard.Run(context.Background(), triageFixtureConfig(t, root))
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if findings := findSection(t, report, "Code Quality").Findings; len(findings) != 0 {
		t.Fatalf("expected dismissal after retry, got %+v", findings)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 provider requests (429 then 200), got %d", got)
	}
}

func TestHybridTriageProviderFailureKeepsFindings(t *testing.T) {
	root := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("CODEGUARD_AI_TRIAGE_PROVIDER", "anthropic")
	t.Setenv("CODEGUARD_AI_TRIAGE_MODEL", "claude-sonnet-4-6")
	t.Setenv("CODEGUARD_AI_TRIAGE_BASE_URL", server.URL)
	t.Setenv("CODEGUARD_AI_TRIAGE_API_KEY", "triage-key")
	t.Setenv("CODEGUARD_AI_RETRY_BASE_DELAY", "1ms")
	t.Setenv("CODEGUARD_AI_MAX_RETRIES", "1")

	report, err := codeguard.Run(context.Background(), triageFixtureConfig(t, root))
	if err != nil {
		t.Fatalf("expected scan to survive provider failure, got %v", err)
	}
	if findings := findSection(t, report, "Code Quality").Findings; len(findings) == 0 {
		t.Fatal("expected static findings to be kept when the provider fails")
	}
	artifact := findAIAnalysisArtifact(report)
	if artifact == nil || artifact.AIAnalysis == nil {
		t.Fatalf("expected ai_analysis artifact recording the failure, got %#v", report.Artifacts)
	}
	foundError := false
	for _, verdict := range artifact.AIAnalysis.Verdicts {
		if verdict.Status == "error" {
			foundError = true
		}
	}
	if !foundError {
		t.Fatalf("expected an error verdict, got %#v", artifact.AIAnalysis.Verdicts)
	}
}
