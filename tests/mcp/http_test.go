package mcp_test

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/codeguard/internal/cli"
)

// The HTTP transport is exercised end-to-end by launching the real `serve --mcp
// --http` binary in a subprocess (mirroring the stdio smoke harness) and
// driving it over HTTP, keeping all tests in the external mcp_test package.

const httpTestToken = "http-smoke-token"

func TestMCPServeHTTPHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MCP_HTTP_HELPER_PROCESS") != "1" {
		return
	}
	args := []string{"serve", "--mcp", "--http", "--addr", os.Getenv("CODEGUARD_TEST_HTTP_ADDR")}
	if token := os.Getenv("CODEGUARD_TEST_HTTP_TOKEN"); token != "" {
		args = append(args, "--auth-token", token)
	}
	if cfg := os.Getenv("CODEGUARD_TEST_HTTP_CONFIG"); cfg != "" {
		args = append(args, "-config", cfg)
	}
	os.Exit(cli.Run(args, os.Stdin, os.Stdout, os.Stderr))
}

func TestMCPServeHTTP(t *testing.T) {
	openBase := startHTTPServer(t, "")
	authBase := startHTTPServer(t, httpTestToken)

	t.Run("initialize-capabilities", func(t *testing.T) {
		resp, body := mcpPost(t, openBase, nil, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
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
	})

	t.Run("tools-list-annotations", func(t *testing.T) {
		_, body := mcpPost(t, openBase, nil, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
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
				continue // apply_fix is the one destructive tool
			}
			if !tool.Annotations.ReadOnlyHint {
				t.Fatalf("tool %q missing readOnlyHint", tool.Name)
			}
		}
	})

	t.Run("tool-call-streams-sse", func(t *testing.T) {
		resp, body := mcpPost(t, openBase, nil, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"list_rules","arguments":{},"_meta":{"progressToken":"p1"}}}`)
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
	})

	t.Run("resource-read", func(t *testing.T) {
		_, body := mcpPost(t, openBase, nil, `{"jsonrpc":"2.0","id":9,"method":"resources/read","params":{"uri":"codeguard://rules"}}`)
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
	})

	t.Run("prompt-get", func(t *testing.T) {
		_, body := mcpPost(t, openBase, nil, `{"jsonrpc":"2.0","id":5,"method":"prompts/get","params":{"name":"review-diff","arguments":{"diff":"--- a\n+++ b\n"}}}`)
		if !strings.Contains(body, "validate_patch") {
			t.Fatalf("expected review-diff prompt to mention validate_patch: %s", body)
		}
	})

	t.Run("health-and-get-stream-needs-session", func(t *testing.T) {
		resp, body := httpGet(t, openBase+"/healthz")
		if resp.StatusCode != http.StatusOK || !strings.Contains(body, "ok") {
			t.Fatalf("unexpected health response: %d %s", resp.StatusCode, body)
		}
		// GET /mcp is now the server→client SSE stream; without a session id it
		// has nothing to attach to and returns 404.
		resp, _ = httpGet(t, openBase+"/mcp")
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 for GET /mcp without session, got %d", resp.StatusCode)
		}
	})

	t.Run("auth-enforcement", func(t *testing.T) {
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
			resp, body := mcpPost(t, authBase, tc.header, initBody)
			if resp.StatusCode != tc.want {
				t.Fatalf("%s: expected %d, got %d body=%s", tc.name, tc.want, resp.StatusCode, body)
			}
		}
	})
}

// startHTTPServer launches the serve --mcp --http binary on a free port and
// waits for /healthz, returning the base URL. The subprocess is killed on test
// cleanup.
func startHTTPServer(t *testing.T, token string) string {
	t.Helper()
	return startHTTPServerWithConfig(t, token, "")
}

func startHTTPServerWithConfig(t *testing.T, token string, configPath string) string {
	t.Helper()
	addr := freeTCPAddr(t)
	cmd := exec.Command(os.Args[0], "-test.run=TestMCPServeHTTPHelperProcess")
	cmd.Env = append(os.Environ(),
		"GO_WANT_MCP_HTTP_HELPER_PROCESS=1",
		"CODEGUARD_TEST_HTTP_ADDR="+addr,
		"CODEGUARD_TEST_HTTP_TOKEN="+token,
		"CODEGUARD_TEST_HTTP_CONFIG="+configPath,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start http helper: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	base := "http://" + addr
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return base
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("http server at %s did not become ready", base)
	return ""
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func mcpPost(t *testing.T, base string, header map[string]string, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	data, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, string(data)
}

func httpGet(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	data, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, string(data)
}
