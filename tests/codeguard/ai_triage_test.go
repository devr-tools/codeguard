package codeguard_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func openAITriageHandler(t *testing.T, wrap func(string) string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
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
			"choices": []map[string]any{{"message": map[string]string{"content": wrap(string(text))}}},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}
}

func TestHybridTriageOpenAIWrappedVerdicts(t *testing.T) {
	tests := []struct {
		name        string
		wrap        func(string) string
		wantDismiss bool
	}{
		{name: "fenced JSON", wrap: func(text string) string { return "```json\n" + text + "\n```" }, wantDismiss: true},
		{name: "prose wrapped JSON", wrap: func(text string) string { return "Here are the verdicts:\n" + text + "\nI kept the response concise." }, wantDismiss: true},
		{name: "invalid body", wrap: func(string) string { return "not JSON" }},
		{name: "empty body", wrap: func(string) string { return "   " }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			server := httptest.NewServer(openAITriageHandler(t, tt.wrap))
			defer server.Close()

			t.Setenv("CODEGUARD_AI_TRIAGE_PROVIDER", "openai")
			t.Setenv("CODEGUARD_AI_TRIAGE_MODEL", "test-model")
			t.Setenv("CODEGUARD_AI_TRIAGE_BASE_URL", server.URL)
			t.Setenv("CODEGUARD_AI_TRIAGE_API_KEY", "triage-key")

			report, err := codeguard.Run(context.Background(), triageFixtureConfig(t, root))
			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}
			artifact := findAIAnalysisArtifact(report)
			if artifact == nil || artifact.AIAnalysis == nil {
				t.Fatalf("expected ai_analysis artifact, got %#v", report.Artifacts)
			}
			if tt.wantDismiss {
				if findings := findSection(t, report, "Code Quality").Findings; len(findings) != 0 {
					t.Fatalf("expected wrapped verdict to dismiss findings, got %+v", findings)
				}
				if verdicts := artifact.AIAnalysis.Verdicts; len(verdicts) != 1 || verdicts[0].Status != "dismissed" {
					t.Fatalf("expected one dismissed verdict, got %#v", verdicts)
				}
				return
			}
			if findings := findSection(t, report, "Code Quality").Findings; len(findings) == 0 {
				t.Fatal("expected invalid provider response to preserve findings")
			}
			if verdicts := artifact.AIAnalysis.Verdicts; len(verdicts) != 1 || verdicts[0].Status != "error" {
				t.Fatalf("expected one error verdict, got %#v", verdicts)
			} else if !strings.Contains(verdicts[0].Summary, "ai triage provider returned invalid JSON verdicts") {
				t.Fatalf("expected invalid JSON error summary, got %#v", verdicts[0])
			}
		})
	}
}

func TestHybridTriageStaysOfflineWithoutProvider(t *testing.T) {
	root := t.TempDir()
	writeArtifactFile(t, filepath.Join(root, "service.go"), `package sample

func BuildClient() error {
	err := doThing()
	_ = err
	return nil
}

func doThing() error { return nil }
`)

	cacheEnabled := true
	report, err := codeguard.Run(context.Background(), codeguard.Config{
		Name: "offline-triage",
		Targets: []codeguard.TargetConfig{{
			Name:     "go-target",
			Path:     root,
			Language: "go",
		}},
		Checks: codeguard.CheckConfig{
			Quality:      true,
			Context:      contextOff(),
			QualityRules: codeguard.QualityRulesConfig{LocalPrecision: localPrecisionOff()},
		},
		Output: codeguard.OutputConfig{Format: "json"},
		Cache: codeguard.CacheConfig{
			Enabled: &cacheEnabled,
			Path:    filepath.Join(root, ".codeguard", "cache.json"),
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(findSection(t, report, "Code Quality").Findings) == 0 {
		t.Fatal("expected static findings without provider")
	}
	if findAIAnalysisArtifact(report) != nil {
		t.Fatalf("expected no ai_analysis artifact when provider is absent, got %#v", report.Artifacts)
	}
}

func TestHybridTriageDismissesStaticFinding(t *testing.T) {
	root := t.TempDir()
	writeArtifactFile(t, filepath.Join(root, "service.go"), `package sample

func BuildClient() error {
	err := doThing()
	_ = err
	return nil
}

func doThing() error { return nil }
`)

	t.Setenv("CODEGUARD_AI_TRIAGE_PROVIDER", "mock")
	t.Setenv("CODEGUARD_AI_TRIAGE_MODEL", "test-model")
	t.Setenv("CODEGUARD_AI_TRIAGE_DECISION", "dismiss")
	t.Setenv("CODEGUARD_AI_TRIAGE_SUMMARY", "blank identifier use is intentional in this fixture")

	cacheEnabled := true
	report, err := codeguard.Run(context.Background(), codeguard.Config{
		Name: "provider-triage",
		Targets: []codeguard.TargetConfig{{
			Name:     "go-target",
			Path:     root,
			Language: "go",
		}},
		Checks: codeguard.CheckConfig{
			Quality:      true,
			Context:      contextOff(),
			QualityRules: codeguard.QualityRulesConfig{LocalPrecision: localPrecisionOff()},
		},
		Output: codeguard.OutputConfig{Format: "json"},
		Cache: codeguard.CacheConfig{
			Enabled: &cacheEnabled,
			Path:    filepath.Join(root, ".codeguard", "cache.json"),
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	section := findSection(t, report, "Code Quality")
	if len(section.Findings) != 0 {
		t.Fatalf("expected triage dismissal to remove static finding, got %+v", section.Findings)
	}
	artifact := findAIAnalysisArtifact(report)
	if artifact == nil || artifact.AIAnalysis == nil {
		t.Fatalf("expected ai_analysis artifact, got %#v", report.Artifacts)
	}
	if len(artifact.AIAnalysis.Verdicts) != 1 {
		t.Fatalf("expected 1 triage verdict, got %#v", artifact.AIAnalysis)
	}
	if artifact.AIAnalysis.Verdicts[0].Status != "dismissed" {
		t.Fatalf("expected dismissed verdict, got %#v", artifact.AIAnalysis.Verdicts[0])
	}
}

func TestHybridTriageCachesVerdictsByContentHash(t *testing.T) {
	root := t.TempDir()
	writeArtifactFile(t, filepath.Join(root, "service.go"), `package sample

func BuildClient() error {
	err := doThing()
	_ = err
	return nil
}

func doThing() error { return nil }
`)

	counterPath := filepath.Join(root, "triage-count.txt")
	t.Setenv("CODEGUARD_AI_TRIAGE_PROVIDER", "mock")
	t.Setenv("CODEGUARD_AI_TRIAGE_MODEL", "test-model")
	t.Setenv("CODEGUARD_AI_TRIAGE_DECISION", "dismiss")
	t.Setenv("CODEGUARD_AI_TRIAGE_SUMMARY", "cached dismissal")
	t.Setenv("CODEGUARD_AI_TRIAGE_COUNT_FILE", counterPath)

	cachePath := filepath.Join(root, ".codeguard", "cache.json")
	cacheEnabled := true
	cfg := codeguard.Config{
		Name: "cache-triage",
		Targets: []codeguard.TargetConfig{{
			Name:     "go-target",
			Path:     root,
			Language: "go",
		}},
		Checks: codeguard.CheckConfig{
			Quality:      true,
			Context:      contextOff(),
			QualityRules: codeguard.QualityRulesConfig{LocalPrecision: localPrecisionOff()},
		},
		Output: codeguard.OutputConfig{Format: "json"},
		Cache: codeguard.CacheConfig{
			Enabled: &cacheEnabled,
			Path:    cachePath,
		},
	}

	first, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	second, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}
	if data, readErr := os.ReadFile(counterPath); readErr != nil {
		t.Fatalf("read count file: %v", readErr)
	} else if strings.TrimSpace(string(data)) != "1" {
		t.Fatalf("expected 1 provider call across cached rerun, got %q", strings.TrimSpace(string(data)))
	}
	if verdicts := findAIAnalysisArtifact(second).AIAnalysis.Verdicts; len(verdicts) != 1 || verdicts[0].Status != "cached-dismissed" {
		t.Fatalf("expected cached-dismissed verdict on second run, got %#v", verdicts)
	}
	if len(findSection(t, first, "Code Quality").Findings) != 0 || len(findSection(t, second, "Code Quality").Findings) != 0 {
		t.Fatal("expected dismissal to persist across cached rerun")
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("expected cache file: %v", err)
	}
	if !strings.Contains(string(data), "\"triage_verdicts\"") {
		t.Fatalf("expected triage verdicts in cache file, got %s", string(data))
	}
}

//nolint:unparam // general-purpose test helper; name is part of its API shape
func findSection(t *testing.T, report codeguard.Report, name string) codeguard.SectionResult {
	t.Helper()
	for _, section := range report.Sections {
		if section.Name == name {
			return section
		}
	}
	t.Fatalf("section %q not found", name)
	return codeguard.SectionResult{}
}

func findAIAnalysisArtifact(report codeguard.Report) *codeguard.Artifact {
	for idx := range report.Artifacts {
		if report.Artifacts[idx].Kind == "ai_analysis" {
			return &report.Artifacts[idx]
		}
	}
	return nil
}
