package mcp_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func assertCurrentDiscovery(t *testing.T, lines []string) {
	t.Helper()
	if len(lines) != 3 {
		t.Fatalf("expected 3 responses, got %d: %q", len(lines), lines)
	}
	initLine := findResponseLineByID(t, lines, `"init-1"`)
	toolsLine := findResponseLineByID(t, lines, `"tools-1"`)
	explainLine := findResponseLineByID(t, lines, `"explain-1"`)

	var initResp struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	decodeLine(t, initLine, &initResp)
	if initResp.Result.ProtocolVersion != "2025-11-25" {
		t.Fatalf("unexpected protocol version: %#v", initResp)
	}

	var listResp struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Annotations struct {
					ReadOnlyHint bool `json:"readOnlyHint"`
				} `json:"annotations"`
			} `json:"tools"`
		} `json:"result"`
	}
	decodeLine(t, toolsLine, &listResp)
	if !containsTool(toolNames(listResp.Result.Tools), "list_rules") || !containsTool(toolNames(listResp.Result.Tools), "validate_patch") {
		t.Fatalf("unexpected tool catalog: %#v", listResp.Result.Tools)
	}
	for _, tool := range listResp.Result.Tools {
		if tool.Name == "apply_fix" {
			continue // apply_fix is the one destructive tool
		}
		if !tool.Annotations.ReadOnlyHint {
			t.Fatalf("expected tool %q to carry readOnlyHint annotation: %#v", tool.Name, listResp.Result.Tools)
		}
	}

	var explainResp struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				ID string `json:"id"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	decodeLine(t, explainLine, &explainResp)
	if explainResp.Result.IsError || explainResp.Result.StructuredContent.ID != "security.hardcoded-secret" {
		t.Fatalf("unexpected explain response: %#v", explainResp)
	}
}

func assertCompatDiscovery(t *testing.T, lines []string) {
	t.Helper()
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d: %q", len(lines), lines)
	}
	initLine := findResponseLineByID(t, lines, "1")
	pingLine := findResponseLineByID(t, lines, "2")

	var initResp struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	decodeLine(t, initLine, &initResp)
	if initResp.Result.ProtocolVersion != "2025-06-18" {
		t.Fatalf("unexpected protocol version: %#v", initResp)
	}

	var pingResp struct {
		Result map[string]any `json:"result"`
	}
	decodeLine(t, pingLine, &pingResp)
}

func assertValidatePatchProfile(t *testing.T, lines []string) {
	t.Helper()
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d: %q", len(lines), lines)
	}
	patchLine := findResponseLineByID(t, lines, `"patch-1"`)
	var patchResp struct {
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
	decodeLine(t, patchLine, &patchResp)
	if patchResp.Result.IsError || patchResp.Result.StructuredContent.Summary.FailedSections == 0 || patchResp.Result.StructuredContent.Summary.TotalFindings == 0 {
		t.Fatalf("unexpected validate_patch response: %#v", patchResp)
	}
}

func assertScanProgressCancel(t *testing.T, lines []string) {
	t.Helper()
	if len(lines) < 3 {
		t.Fatalf("expected initialize and progress messages, got %d: %q", len(lines), lines)
	}
	progressSeen := 0
	for _, line := range lines {
		var envelope struct {
			ID     *json.RawMessage `json:"id"`
			Method string           `json:"method"`
			Params struct {
				ProgressToken string `json:"progressToken"`
			} `json:"params"`
		}
		decodeLine(t, line, &envelope)
		if envelope.Method == "notifications/progress" {
			progressSeen++
			if envelope.Params.ProgressToken != "scan-token" {
				t.Fatalf("unexpected progress token: %#v", envelope)
			}
		}
		if scanResponseMatchesID(envelope.ID, 2) {
			t.Fatalf("expected cancelled scan request to suppress final response: %q", lines)
		}
	}
	if progressSeen == 0 {
		t.Fatalf("expected progress notifications, got %q", lines)
	}
}

func assertResourcesDiscovery(t *testing.T, lines []string) {
	t.Helper()
	if len(lines) != 3 {
		t.Fatalf("expected 3 responses, got %d: %q", len(lines), lines)
	}
	listLine := findResponseLineByID(t, lines, `"res-list"`)
	readLine := findResponseLineByID(t, lines, `"res-read"`)

	var listResp struct {
		Result struct {
			Resources []struct {
				URI string `json:"uri"`
			} `json:"resources"`
		} `json:"result"`
	}
	decodeLine(t, listLine, &listResp)
	foundRules := false
	for _, res := range listResp.Result.Resources {
		if res.URI == "codeguard://rules" {
			foundRules = true
		}
	}
	if !foundRules {
		t.Fatalf("expected codeguard://rules in resources/list: %#v", listResp.Result.Resources)
	}

	var readResp struct {
		Result struct {
			Contents []struct {
				URI      string `json:"uri"`
				MIMEType string `json:"mimeType"`
				Text     string `json:"text"`
			} `json:"contents"`
		} `json:"result"`
	}
	decodeLine(t, readLine, &readResp)
	if len(readResp.Result.Contents) == 0 || readResp.Result.Contents[0].URI != "codeguard://rules" {
		t.Fatalf("unexpected resources/read response: %#v", readResp)
	}
	var payload struct {
		Rules []json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal([]byte(readResp.Result.Contents[0].Text), &payload); err != nil || len(payload.Rules) == 0 {
		t.Fatalf("expected rule catalog in resource text, got err=%v payload=%#v", err, payload)
	}
}

func assertPromptsDiscovery(t *testing.T, lines []string) {
	t.Helper()
	if len(lines) != 3 {
		t.Fatalf("expected 3 responses, got %d: %q", len(lines), lines)
	}
	listLine := findResponseLineByID(t, lines, `"prompts-list"`)
	getLine := findResponseLineByID(t, lines, `"prompts-get"`)

	var listResp struct {
		Result struct {
			Prompts []struct {
				Name string `json:"name"`
			} `json:"prompts"`
		} `json:"result"`
	}
	decodeLine(t, listLine, &listResp)
	if !containsTool(toolNames(listResp.Result.Prompts), "review-diff") {
		t.Fatalf("expected review-diff in prompts/list: %#v", listResp.Result.Prompts)
	}

	var getResp struct {
		Result struct {
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		} `json:"result"`
	}
	decodeLine(t, getLine, &getResp)
	if len(getResp.Result.Messages) == 0 {
		t.Fatalf("expected prompt messages, got %#v", getResp)
	}
}

func assertScanStreaming(t *testing.T, lines []string) {
	t.Helper()
	sectionProgress := 0
	sawResult := false
	for _, line := range lines {
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				ProgressToken string `json:"progressToken"`
				Message       string `json:"message"`
			} `json:"params"`
		}
		decodeLine(t, line, &envelope)
		if envelope.Method == "notifications/progress" && envelope.Params.ProgressToken == "stream-tok" {
			// Per-section messages look like "Code Quality: pass (0 findings)".
			if strings.Contains(envelope.Params.Message, ":") && strings.Contains(envelope.Params.Message, "findings") {
				sectionProgress++
			}
		}
		if strings.TrimSpace(string(envelope.ID)) == `"scan-1"` {
			sawResult = true
		}
	}
	if sectionProgress < 2 {
		t.Fatalf("expected at least 2 per-section progress notifications, got %d: %q", sectionProgress, lines)
	}
	if !sawResult {
		t.Fatalf("expected final scan result, got %q", lines)
	}
}

func assertVerifyFixFailsClosed(t *testing.T, lines []string) {
	t.Helper()
	line := findResponseLineByID(t, lines, `"vf-1"`)
	var resp struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Verified bool `json:"verified"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	decodeLine(t, line, &resp)
	if !resp.Result.IsError {
		t.Fatalf("expected verify_fix to fail closed on a bogus diff: %s", line)
	}
	if resp.Result.StructuredContent.Verified {
		t.Fatalf("expected structuredContent.verified=false on a failed fix: %s", line)
	}
}

func assertApplyFixFailsClosed(t *testing.T, lines []string) {
	t.Helper()
	line := findResponseLineByID(t, lines, `"af-1"`)
	var resp struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Applied  bool `json:"applied"`
				Verified bool `json:"verified"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	decodeLine(t, line, &resp)
	if !resp.Result.IsError {
		t.Fatalf("expected apply_fix to fail closed on a bogus diff: %s", line)
	}
	if resp.Result.StructuredContent.Applied || resp.Result.StructuredContent.Verified {
		t.Fatalf("expected apply_fix not applied / not verified on failure: %s", line)
	}
}

func scanResponseMatchesID(raw *json.RawMessage, want int) bool {
	if raw == nil {
		return false
	}
	var id int
	return json.Unmarshal(*raw, &id) == nil && id == want
}
