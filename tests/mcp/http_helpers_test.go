package mcp_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func assertInitializeCapabilities(t *testing.T, base string) {
	t.Helper()
	resp, body := mcpPost(t, base, nil, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, body)
	}
	if resp.Header.Get("Mcp-Session-Id") == "" {
		t.Fatalf("expected Mcp-Session-Id header on initialize")
	}
	var out struct {
		Result struct {
			ProtocolVersion string         `json:"protocolVersion"`
			Capabilities    map[string]any `json:"capabilities"`
		} `json:"result"`
	}
	decodeLine(t, body, &out)
	if out.Result.ProtocolVersion != "2025-06-18" {
		t.Fatalf("unexpected protocol version: %s", out.Result.ProtocolVersion)
	}
	for _, capability := range []string{"tools", "resources", "prompts", "logging"} {
		if _, ok := out.Result.Capabilities[capability]; !ok {
			t.Fatalf("missing capability %q: %#v", capability, out.Result.Capabilities)
		}
	}
}

func assertToolsListAnnotations(t *testing.T, base string) {
	t.Helper()
	_, body := mcpPost(t, base, nil, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	var out struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Annotations struct {
					ReadOnlyHint bool `json:"readOnlyHint"`
				} `json:"annotations"`
			} `json:"tools"`
		} `json:"result"`
	}
	decodeLine(t, body, &out)
	if len(out.Result.Tools) != 8 {
		t.Fatalf("expected 8 tools, got %d", len(out.Result.Tools))
	}
	for _, tool := range out.Result.Tools {
		if tool.Name == "apply_fix" {
			continue
		}
		if !tool.Annotations.ReadOnlyHint {
			t.Fatalf("tool %q missing readOnlyHint", tool.Name)
		}
	}
}

func assertToolCallStreamsSSE(t *testing.T, base string) {
	t.Helper()
	resp, body := mcpPost(t, base, nil, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"list_rules","arguments":{},"_meta":{"progressToken":"p1"}}}`)
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", ct)
	}
	var sawProgress, sawResult bool
	for _, frame := range strings.Split(body, "\n\n") {
		frame = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(frame), "data:"))
		if frame == "" {
			continue
		}
		var msg struct {
			Method string `json:"method"`
			Result struct {
				IsError bool `json:"isError"`
			} `json:"result"`
		}
		decodeLine(t, frame, &msg)
		if msg.Method == "notifications/progress" {
			sawProgress = true
		}
		if msg.Method == "" && !msg.Result.IsError {
			sawResult = true
		}
	}
	if !sawProgress || !sawResult {
		t.Fatalf("expected progress + result over SSE (progress=%v result=%v): %s", sawProgress, sawResult, body)
	}
}

func assertResourceRead(t *testing.T, base string) {
	t.Helper()
	_, body := mcpPost(t, base, nil, `{"jsonrpc":"2.0","id":9,"method":"resources/read","params":{"uri":"codeguard://rules"}}`)
	var out struct {
		Result struct {
			Contents []struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			} `json:"contents"`
		} `json:"result"`
	}
	decodeLine(t, body, &out)
	if len(out.Result.Contents) == 0 || out.Result.Contents[0].URI != "codeguard://rules" {
		t.Fatalf("unexpected resources/read response: %s", body)
	}
	var payload struct {
		Rules []json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal([]byte(out.Result.Contents[0].Text), &payload); err != nil || len(payload.Rules) == 0 {
		t.Fatalf("expected rule catalog in resource text, got err=%v len=%d", err, len(payload.Rules))
	}
}

func assertPromptGet(t *testing.T, base string) {
	t.Helper()
	_, body := mcpPost(t, base, nil, `{"jsonrpc":"2.0","id":5,"method":"prompts/get","params":{"name":"review-diff","arguments":{"diff":"--- a\n+++ b\n"}}}`)
	if !strings.Contains(body, "validate_patch") {
		t.Fatalf("expected review-diff prompt to mention validate_patch: %s", body)
	}
}

func assertHealthAndMissingSession(t *testing.T, base string) {
	t.Helper()
	resp, body := httpGet(t, base+"/healthz")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "ok") {
		t.Fatalf("unexpected health response: %d %s", resp.StatusCode, body)
	}
	resp, _ = httpGet(t, base+"/mcp")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for GET /mcp without session, got %d", resp.StatusCode)
	}
}

func assertAuthEnforcement(t *testing.T, base string) {
	t.Helper()
	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`
	cases := []struct {
		name   string
		header map[string]string
		want   int
	}{
		{"missing", nil, http.StatusUnauthorized},
		{"wrong", map[string]string{"Authorization": "Bearer nope"}, http.StatusUnauthorized},
		{"correct", map[string]string{"Authorization": "Bearer " + httpTestToken}, http.StatusOK},
		{"correct-no-scheme", map[string]string{"Authorization": httpTestToken}, http.StatusOK},
	}
	for _, tc := range cases {
		resp, body := mcpPost(t, base, tc.header, initBody)
		if resp.StatusCode != tc.want {
			t.Fatalf("%s: expected %d, got %d body=%s", tc.name, tc.want, resp.StatusCode, body)
		}
	}
}
