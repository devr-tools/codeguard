package mcp_test

import (
	"encoding/json"
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
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	decodeLine(t, toolsLine, &listResp)
	if !containsTool(listResp.Result.Tools, "list_rules") || !containsTool(listResp.Result.Tools, "validate_patch") {
		t.Fatalf("unexpected tool catalog: %#v", listResp.Result.Tools)
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

func scanResponseMatchesID(raw *json.RawMessage, want int) bool {
	if raw == nil {
		return false
	}
	var id int
	return json.Unmarshal(*raw, &id) == nil && id == want
}
