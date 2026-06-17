package checks_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func semanticOpenAIHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		assertSemanticOpenAIRequest(t, r)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{"verdicts":[{"rule_id":"quality.ai.semantic-test-adequacy","path":"app/users/page.tsx","line":4,"level":"warn","message":"openai-compatible backend verdict"}]}`,
					},
				},
			},
		})
	}
}

func assertSemanticOpenAIRequest(t *testing.T, r *http.Request) {
	t.Helper()
	if r.URL.Path != "/chat/completions" {
		t.Fatalf("path = %q, want /chat/completions", r.URL.Path)
	}
	if got := r.Header.Get("Authorization"); got != "Bearer test-openai-key" {
		t.Fatalf("authorization = %q", got)
	}
	var body struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if body.Model != "gpt-test-semantic" {
		t.Fatalf("model = %q", body.Model)
	}
	if len(body.Messages) < 2 {
		t.Fatalf("messages = %#v", body.Messages)
	}
	assertContainsAll(t, body.Messages[1].Content,
		"tests prove next() chaining",
		"changed params or searchParams handling changes the expected route input contract",
	)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Dir(filepath.Dir(wd))
}

func assertContainsAll(t *testing.T, value string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(value, want) {
			t.Fatalf("output missing %q\nfull output:\n%s", want, value)
		}
	}
}
