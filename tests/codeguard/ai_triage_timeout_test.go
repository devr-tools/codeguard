package codeguard_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/triage"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestAITriageEffectiveTimeoutScalesWithCandidates(t *testing.T) {
	tests := []struct {
		name       string
		candidates int
		want       time.Duration
	}{
		{name: "one candidate uses base", candidates: 1, want: 60 * time.Second},
		{name: "additional candidates add time", candidates: 9, want: 84 * time.Second},
		{name: "large workload is capped", candidates: 100, want: 180 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := triage.EffectiveTimeout(tt.candidates, 0); got != tt.want {
				t.Fatalf("EffectiveTimeout(%d, 0) = %v, want %v", tt.candidates, got, tt.want)
			}
		})
	}
}

func TestAITriageEffectiveTimeoutHonorsExplicitOverride(t *testing.T) {
	for _, want := range []time.Duration{5 * time.Second, 120 * time.Second, 4 * time.Minute} {
		if got := triage.EffectiveTimeout(100, want); got != want {
			t.Fatalf("EffectiveTimeout(100, %v) = %v, want explicit override", want, got)
		}
	}
}

func TestHybridTriageTimeoutIsActionableAndNotRetried(t *testing.T) {
	root := t.TempDir()
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("CODEGUARD_AI_TRIAGE_PROVIDER", "anthropic")
	t.Setenv("CODEGUARD_AI_TRIAGE_MODEL", "claude-sonnet-4-6")
	t.Setenv("CODEGUARD_AI_TRIAGE_BASE_URL", server.URL)
	t.Setenv("CODEGUARD_AI_TRIAGE_API_KEY", "triage-key")
	t.Setenv("CODEGUARD_AI_TRIAGE_TIMEOUT", "20ms")
	t.Setenv("CODEGUARD_AI_RETRY_BASE_DELAY", "1ms")
	t.Setenv("CODEGUARD_AI_MAX_RETRIES", "3")

	report, err := codeguard.Run(context.Background(), triageFixtureConfig(t, root))
	if err != nil {
		t.Fatalf("triage timeout must not fail the scan: %v", err)
	}
	if findings := findSection(t, report, "Code Quality").Findings; len(findings) == 0 {
		t.Fatal("triage timeout removed static findings")
	}
	artifact := findAIAnalysisArtifact(report)
	if artifact == nil || artifact.AIAnalysis == nil || len(artifact.AIAnalysis.Verdicts) != 1 {
		t.Fatalf("expected one provider error verdict, got %#v", artifact)
	}
	summary := artifact.AIAnalysis.Verdicts[0].Summary
	if !strings.Contains(summary, "timed out after 20ms") || !strings.Contains(summary, "CODEGUARD_AI_TRIAGE_TIMEOUT") {
		t.Fatalf("timeout summary is not actionable: %q", summary)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("timed-out request made %d attempts, want 1", got)
	}
}
