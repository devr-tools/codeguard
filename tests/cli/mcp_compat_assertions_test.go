package cli_test

import (
	"strings"
	"testing"
)

func assertCurrentProtocolCompatibility(t *testing.T, lines []string) {
	t.Helper()
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d: %q", len(lines), lines)
	}
	assertInitializeLine(t, lines[0], "2025-11-25", "codeguard")

	var pingResp struct {
		ID string `json:"id"`
	}
	decodeMCPLine(t, lines[1], &pingResp)
	if pingResp.ID != "ping-1" {
		t.Fatalf("unexpected ping response: %#v", pingResp)
	}
}

func assertPreInitializeError(t *testing.T, lines []string) {
	t.Helper()
	if len(lines) != 1 {
		t.Fatalf("expected 1 response, got %d: %q", len(lines), lines)
	}
	var resp struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeMCPLine(t, lines[0], &resp)
	if resp.Error.Code != -32002 || !strings.Contains(resp.Error.Message, "initialized") {
		t.Fatalf("unexpected pre-init error: %#v", resp)
	}
}

func assertValidateConfigCompatibility(t *testing.T, lines []string) {
	t.Helper()
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d: %q", len(lines), lines)
	}
	line := findResponseLineByID(t, lines, "2")
	var resp struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				OK         bool   `json:"ok"`
				ConfigName string `json:"config_name"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	decodeMCPLine(t, line, &resp)
	if resp.Result.IsError || !resp.Result.StructuredContent.OK || resp.Result.StructuredContent.ConfigName != "mcp-compat-test" {
		t.Fatalf("unexpected validate_config response: %#v", resp)
	}
}

func assertListRulesCompatibility(t *testing.T, lines []string) {
	t.Helper()
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d: %q", len(lines), lines)
	}
	line := findResponseLineByID(t, lines, "2")
	var resp struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Rules []struct {
					ID string `json:"id"`
				} `json:"rules"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	decodeMCPLine(t, line, &resp)
	if resp.Result.IsError || len(resp.Result.StructuredContent.Rules) == 0 {
		t.Fatalf("unexpected list_rules response: %#v", resp)
	}
	for _, rule := range resp.Result.StructuredContent.Rules {
		if rule.ID == "security.hardcoded-secret" {
			return
		}
	}
	t.Fatalf("expected hardcoded secret rule in catalog: %#v", resp.Result.StructuredContent.Rules)
}

func assertFallbackProtocolCompatibility(t *testing.T, lines []string) {
	t.Helper()
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d: %q", len(lines), lines)
	}
	assertInitializeLine(t, lines[0], "2025-06-18", "codeguard")
}

func assertEmptyArgumentCompatibility(t *testing.T, lines []string) {
	t.Helper()
	if len(lines) != 3 {
		t.Fatalf("expected 3 responses, got %d: %q", len(lines), lines)
	}
	assertListRulesResponseByID(t, lines, "2")
	assertValidateConfigResponseByID(t, lines, "3")
}

func assertListRulesResponseByID(t *testing.T, lines []string, id string) {
	t.Helper()
	line := findResponseLineByID(t, lines, id)
	var resp struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Rules []struct {
					ID string `json:"id"`
				} `json:"rules"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	decodeMCPLine(t, line, &resp)
	if resp.Result.IsError || len(resp.Result.StructuredContent.Rules) == 0 {
		t.Fatalf("unexpected list_rules response: %#v", resp)
	}
}

func assertValidateConfigResponseByID(t *testing.T, lines []string, id string) {
	t.Helper()
	line := findResponseLineByID(t, lines, id)
	var resp struct {
		Result struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				OK bool `json:"ok"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	decodeMCPLine(t, line, &resp)
	if resp.Result.IsError || !resp.Result.StructuredContent.OK {
		t.Fatalf("unexpected validate_config response: %#v", resp)
	}
}

func assertUnknownToolCompatibility(t *testing.T, lines []string) {
	t.Helper()
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d: %q", len(lines), lines)
	}
	line := findResponseLineByID(t, lines, "2")
	var resp struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeMCPLine(t, line, &resp)
	if resp.Error.Code != -32602 || !strings.Contains(resp.Error.Message, "unknown tool") {
		t.Fatalf("unexpected unknown-tool error: %#v", resp)
	}
}

func assertNotificationCompatibility(t *testing.T, lines []string) {
	t.Helper()
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d: %q", len(lines), lines)
	}
	var pingResp struct {
		ID int `json:"id"`
	}
	decodeMCPLine(t, lines[1], &pingResp)
	if pingResp.ID != 2 {
		t.Fatalf("unexpected ping response: %#v", pingResp)
	}
}
