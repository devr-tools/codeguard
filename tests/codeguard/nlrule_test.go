package codeguard_test

import (
	"context"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/nlrule"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type stubNLRuntime struct {
	enabled  bool
	response nlrule.EvaluationResponse
}

func (runtime stubNLRuntime) Enabled() bool { return runtime.enabled }

func (runtime stubNLRuntime) Fingerprint() string { return "stub" }

func (runtime stubNLRuntime) Evaluate(context.Context, nlrule.EvaluationRequest) (nlrule.EvaluationResponse, error) {
	return runtime.response, nil
}

func TestNLRuleCompileIncludesNumberedSourceAndInstruction(t *testing.T) {
	rule := core.CustomRuleConfig{
		ID:              "custom.no-request-body-logs",
		Title:           "Never log request bodies",
		Message:         "request bodies must not be logged in handlers",
		NaturalLanguage: "never log request bodies in handlers",
	}
	request := nlrule.Compile(rule, "handlers/login.go", []byte("first line\nsecond line\n"))
	if request.Rule.Instruction != rule.NaturalLanguage {
		t.Fatalf("instruction = %q, want %q", request.Rule.Instruction, rule.NaturalLanguage)
	}
	if request.File.Path != "handlers/login.go" {
		t.Fatalf("path = %q", request.File.Path)
	}
	if request.File.Content != "1: first line\n2: second line\n3: " {
		t.Fatalf("unexpected numbered content: %q", request.File.Content)
	}
	if request.Prompt == "" {
		t.Fatal("expected prompt")
	}
}

func TestNLRuleEvaluateFileFallsBackToConfiguredMessageAndWhy(t *testing.T) {
	rule := core.CustomRuleConfig{
		ID:              "custom.no-request-body-logs",
		Message:         "request bodies must not be logged in handlers",
		NaturalLanguage: "never log request bodies in handlers",
	}
	findings, err := nlrule.EvaluateFile(context.Background(), stubNLRuntime{
		enabled: true,
		response: nlrule.EvaluationResponse{
			Matches: []nlrule.Match{{Line: 8}},
		},
	}, rule, "handlers/login.go", []byte("body"))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Message != rule.Message {
		t.Fatalf("message = %q, want %q", findings[0].Message, rule.Message)
	}
	if findings[0].Why != rule.Message {
		t.Fatalf("why = %q, want %q", findings[0].Why, rule.Message)
	}
}
