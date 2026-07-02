package cli_test

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

func assertInitializeLine(t *testing.T, line string, version string, serverName string) {
	t.Helper()
	var resp struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
			ServerInfo      struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	decodeMCPLine(t, line, &resp)
	if resp.Result.ProtocolVersion != version || resp.Result.ServerInfo.Name != serverName {
		t.Fatalf("unexpected initialize response: %#v", resp)
	}
}

func assertToolCatalogLine(t *testing.T, line string, expected ...string) {
	t.Helper()
	var resp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	decodeMCPLine(t, line, &resp)
	for _, name := range expected {
		if !containsTool(resp.Result.Tools, name) {
			t.Fatalf("missing tool %s in %#v", name, resp.Result.Tools)
		}
	}
}

func assertExplainLine(t *testing.T, line string, ruleID string, executionModel string) {
	t.Helper()
	var resp struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				ID             string `json:"id"`
				ExecutionModel string `json:"execution_model"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	decodeMCPLine(t, line, &resp)
	if resp.Result.IsError || resp.Result.StructuredContent.ID != ruleID || resp.Result.StructuredContent.ExecutionModel != executionModel {
		t.Fatalf("unexpected explain payload: %#v", resp)
	}
}

func assertExplainFixTemplateLine(t *testing.T, line string, ruleID string) {
	t.Helper()
	var resp struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				ID              string `json:"id"`
				FixTemplate     string `json:"fix_template"`
				FixTemplateKind string `json:"fix_template_kind"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	decodeMCPLine(t, line, &resp)
	if resp.Result.IsError || resp.Result.StructuredContent.ID != ruleID {
		t.Fatalf("unexpected explain payload: %#v", resp)
	}
	if strings.TrimSpace(resp.Result.StructuredContent.FixTemplate) == "" {
		t.Fatalf("expected populated fix_template for %s, got %#v", ruleID, resp)
	}
	if kind := resp.Result.StructuredContent.FixTemplateKind; kind != "deterministic" && kind != "guided" {
		t.Fatalf("expected valid fix_template_kind for %s, got %#v", ruleID, resp)
	}
}

func assertValidatePatchLine(t *testing.T, line string) {
	t.Helper()
	var resp struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Summary struct {
					FailedSections int `json:"failed_sections"`
					TotalFindings  int `json:"total_findings"`
				} `json:"summary"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	decodeMCPLine(t, line, &resp)
	if resp.Result.IsError || resp.Result.StructuredContent.Summary.FailedSections == 0 || resp.Result.StructuredContent.Summary.TotalFindings == 0 {
		t.Fatalf("unexpected validate_patch payload: %#v", resp)
	}
}

func assertProgressValues(t *testing.T, lines []string, token string, expected []float64) {
	t.Helper()
	var got []float64
	for _, line := range lines {
		var envelope struct {
			Method string `json:"method"`
			Params struct {
				ProgressToken string  `json:"progressToken"`
				Progress      float64 `json:"progress"`
			} `json:"params"`
		}
		decodeMCPLine(t, line, &envelope)
		if envelope.Method != "notifications/progress" {
			continue
		}
		if envelope.Params.ProgressToken != token {
			t.Fatalf("unexpected progress token: %#v", envelope)
		}
		got = append(got, envelope.Params.Progress)
	}
	sort.Float64s(got)
	if len(got) != len(expected) {
		t.Fatalf("unexpected progress notifications: %#v", got)
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("unexpected progress notifications: %#v", got)
		}
	}
}

func assertCancellationBehavior(t *testing.T, lines []string, requestID int, token string) {
	t.Helper()
	progressSeen := 0
	resultResponses := 0
	for _, line := range lines {
		var envelope struct {
			ID     *json.RawMessage `json:"id"`
			Method string           `json:"method"`
			Params struct {
				ProgressToken string `json:"progressToken"`
			} `json:"params"`
		}
		decodeMCPLine(t, line, &envelope)
		if envelope.Method == "notifications/progress" {
			progressSeen++
			if envelope.Params.ProgressToken != token {
				t.Fatalf("unexpected progress token: %#v", envelope)
			}
		}
		if responseMatchesID(t, envelope.ID, requestID) {
			resultResponses++
		}
	}
	if progressSeen == 0 {
		t.Fatalf("expected progress notifications during cancelled request: %q", lines)
	}
	if resultResponses != 0 {
		t.Fatalf("expected cancelled request to suppress final response: %q", lines)
	}
}

func responseMatchesID(t *testing.T, raw *json.RawMessage, want int) bool {
	t.Helper()
	if raw == nil {
		return false
	}
	var id int
	return json.Unmarshal(*raw, &id) == nil && id == want
}

func assertMCPPromptFileUnchanged(t *testing.T, promptPath string) {
	t.Helper()
	data, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	if strings.Contains(string(data), "OPENAI_API_KEY") {
		t.Fatalf("working tree file was modified: %s", string(data))
	}
}
