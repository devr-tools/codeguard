package cli_test

import (
	"encoding/json"
	"testing"
)

type explainAgentPayload struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Section          string `json:"section"`
	Level            string `json:"level"`
	ExecutionModel   string `json:"execution_model"`
	Description      string `json:"description"`
	Why              string `json:"why"`
	HowToFix         string `json:"how_to_fix"`
	FixTemplate      string `json:"fix_template"`
	FixTemplateKind  string `json:"fix_template_kind"`
	LanguageCoverage struct {
		Mode      string   `json:"mode"`
		Languages []string `json:"languages"`
	} `json:"language_coverage"`
}

func decodeExplainAgentPayload(t *testing.T, body []byte, raw string) explainAgentPayload {
	t.Helper()
	var payload explainAgentPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("expected valid json, got err=%v body=%s", err, raw)
	}
	return payload
}

func assertExplainAgentPayload(t *testing.T, payload explainAgentPayload, ruleID string) {
	t.Helper()
	if payload.ID != ruleID {
		t.Fatalf("expected rule id, got %#v", payload)
	}
	if payload.ExecutionModel != "language-agnostic" {
		t.Fatalf("expected execution model, got %#v", payload)
	}
	if payload.LanguageCoverage.Mode != "repository-wide" {
		t.Fatalf("expected repository-wide coverage, got %#v", payload.LanguageCoverage)
	}
	if len(payload.LanguageCoverage.Languages) != 0 {
		t.Fatalf("expected empty languages for repository-wide coverage, got %#v", payload.LanguageCoverage.Languages)
	}
	if payload.Description == "" || payload.Why == "" {
		t.Fatalf("expected description and why, got %#v", payload)
	}
	if payload.HowToFix == "" {
		t.Fatalf("expected how_to_fix, got %#v", payload)
	}
	if payload.FixTemplate == "" {
		t.Fatalf("expected populated fix_template for catalog rule, got %#v", payload)
	}
	if payload.FixTemplateKind != "deterministic" && payload.FixTemplateKind != "guided" {
		t.Fatalf("expected valid fix_template_kind, got %#v", payload)
	}
}
