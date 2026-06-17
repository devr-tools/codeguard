package checks_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type semanticFrameworkEntry struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Signals []string `json:"signals"`
	Hints   []string `json:"hints"`
}

type semanticPromptTemplate struct {
	Overview              string                    `json:"overview"`
	ResponseRequirements  []string                  `json:"response_requirements"`
	RuleInstructions      []semanticRulePrompt      `json:"rule_instructions"`
	FrameworkInstructions []semanticFrameworkPrompt `json:"framework_instructions"`
}

type semanticRulePrompt struct {
	RuleID    string   `json:"rule_id"`
	Focus     string   `json:"focus"`
	Consider  []string `json:"consider"`
	Avoid     []string `json:"avoid"`
	Threshold string   `json:"threshold"`
}

type semanticFrameworkPrompt struct {
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Hints  []string `json:"hints"`
	Advice []string `json:"advice"`
}

func readSemanticFrameworks(t *testing.T, requestPath string) []semanticFrameworkEntry {
	t.Helper()
	var req struct {
		Frameworks []semanticFrameworkEntry `json:"frameworks"`
	}
	readSemanticRequest(t, requestPath, &req)
	return req.Frameworks
}

func readSemanticPrompt(t *testing.T, requestPath string) semanticPromptTemplate {
	t.Helper()
	var req struct {
		Prompt semanticPromptTemplate `json:"prompt"`
	}
	readSemanticRequest(t, requestPath, &req)
	return req.Prompt
}

func readSemanticRequest(t *testing.T, requestPath string, out any) {
	t.Helper()
	data, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
}

func requireFrameworkEntry(t *testing.T, frameworks []semanticFrameworkEntry, name string, path string) semanticFrameworkEntry {
	t.Helper()
	for _, framework := range frameworks {
		if framework.Name == name && framework.Path == path {
			return framework
		}
	}
	t.Fatalf("framework entry %s %s not found in %#v", name, path, frameworks)
	return semanticFrameworkEntry{}
}

func assertStringSliceContainsAll(t *testing.T, got []string, want ...string) {
	t.Helper()
	seen := map[string]struct{}{}
	for _, value := range got {
		seen[value] = struct{}{}
	}
	for _, value := range want {
		if _, ok := seen[value]; !ok {
			t.Fatalf("slice %v missing %q", got, value)
		}
	}
}

func assertRulePromptContainsAll(t *testing.T, prompt semanticPromptTemplate, ruleID string, want ...string) {
	t.Helper()
	for _, instruction := range prompt.RuleInstructions {
		if instruction.RuleID != ruleID {
			continue
		}
		all := append([]string{instruction.Focus, instruction.Threshold}, instruction.Consider...)
		for _, value := range want {
			if !containsSubstring(all, value) {
				t.Fatalf("rule prompt %s missing %q in %#v", ruleID, value, instruction)
			}
		}
		return
	}
	t.Fatalf("rule prompt %s not found in %#v", ruleID, prompt.RuleInstructions)
}

func containsSubstring(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}
