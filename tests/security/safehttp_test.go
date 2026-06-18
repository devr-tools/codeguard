package security_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/safehttp"
	"github.com/devr-tools/codeguard/internal/codeguard/trust"
)

func TestValidateProviderURLAllowlist(t *testing.T) {
	trust.Set(trust.Policy{})
	t.Cleanup(trust.ResetFromEnv)

	if err := safehttp.ValidateProviderURL("", false); err != nil {
		t.Fatalf("empty URL should be accepted (provider default), got %v", err)
	}
	if err := safehttp.ValidateProviderURL("https://api.openai.com/v1", false); err != nil {
		t.Fatalf("allowlisted host should be accepted, got %v", err)
	}
	if err := safehttp.ValidateProviderURL("https://evil.example.com/v1", false); err == nil {
		t.Fatal("non-allowlisted host from untrusted config must be rejected")
	}
	if err := safehttp.ValidateProviderURL("file:///etc/passwd", false); err == nil {
		t.Fatal("non-http scheme must be rejected")
	}
}

func TestValidateProviderURLTrustedSourceAndOptIn(t *testing.T) {
	trust.Set(trust.Policy{})
	t.Cleanup(trust.ResetFromEnv)

	// Trusted source (e.g. an environment variable) bypasses the allowlist.
	if err := safehttp.ValidateProviderURL("https://internal.host/v1", true); err != nil {
		t.Fatalf("trusted-source URL should be accepted, got %v", err)
	}

	// Operator opt-in bypasses the allowlist for config-sourced URLs too.
	trust.Set(trust.Policy{AllowConfigAIEndpoints: true})
	if err := safehttp.ValidateProviderURL("https://internal.host/v1", false); err != nil {
		t.Fatalf("opt-in should accept custom endpoint, got %v", err)
	}
}

func TestClientBlocksLoopbackByDefault(t *testing.T) {
	trust.Set(trust.Policy{})
	t.Cleanup(trust.ResetFromEnv)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := safehttp.Client(5 * time.Second)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected SSRF guard to block connection to loopback test server")
	}
	if !strings.Contains(err.Error(), "ssrf guard") {
		t.Fatalf("expected ssrf guard error, got %v", err)
	}
}

func TestClientAllowsLoopbackWhenOptedIn(t *testing.T) {
	trust.Set(trust.Policy{AllowConfigAIEndpoints: true})
	t.Cleanup(trust.ResetFromEnv)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := safehttp.Client(5 * time.Second)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("opt-in client should reach loopback endpoint, got %v", err)
	}
	resp.Body.Close()
}
