package codeguard_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// The report masks secret values (AKIA…UVWX); the triage snippet leaves the
// machine, so it must not carry them either: not on a credential finding's own
// line, not on a neighbouring line of another finding's window, and not in a
// PEM key body, which has no marker of its own.
func TestHybridTriageRedactsSecretsFromProviderSnippets(t *testing.T) {
	awsKey := "AKIA" + "Z7Q3XK2LMNPRSTUV"
	keyBody := "MIIEowIBAAKCAQEA" + "x7kQ2mZpLr9TvWcYbN4sHdJf"
	source := strings.Join([]string{
		"package main",
		"",
		"const awsKey = \"" + awsKey + "\"",
		"",
		"const pemKey = `-----BEGIN " + "RSA PRIVATE KEY-----", // codeguard:ignore security.private-key -- synthetic PEM header written to a temp dir to prove triage redacts key blocks
		keyBody,
		"-----END " + "RSA PRIVATE KEY-----`",
		"",
		"func main() {}",
	}, "\n") + "\n"

	var mu sync.Mutex
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		mu.Unlock()
		payload, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": `{"verdicts":[]}`}}},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	t.Setenv("CODEGUARD_AI_TRIAGE_PROVIDER", "openai")
	t.Setenv("CODEGUARD_AI_TRIAGE_MODEL", "test-model")
	t.Setenv("CODEGUARD_AI_TRIAGE_BASE_URL", server.URL)
	t.Setenv("CODEGUARD_AI_TRIAGE_API_KEY", "triage-key")

	root := t.TempDir()
	writeRepoFile(t, filepath.Join(root, "service.go"), source)
	cfg := codeguard.ExampleConfig()
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: root, Language: "go"}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	// A fresh cache: triage verdicts are cached by content, and this fixture's
	// content is fixed, so a shared cache would skip the provider request.
	cfg.Cache.Path = filepath.Join(t.TempDir(), "cache.json")

	if _, err := codeguard.Run(context.Background(), cfg); err != nil {
		t.Fatalf("run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) == 0 {
		t.Fatal("triage provider received no request; the fixture produced no candidates")
	}
	sent := strings.Join(bodies, "\n")
	if !strings.Contains(sent, "redacted: possible secret") {
		t.Errorf("expected redacted snippet lines in the triage request")
	}
	for _, secret := range []string{awsKey, keyBody} {
		if strings.Contains(sent, secret) {
			t.Errorf("triage request contains secret material %q", secret[:8]+"…")
		}
	}
}
