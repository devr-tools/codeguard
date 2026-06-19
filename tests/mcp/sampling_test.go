package mcp_test

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These tests exercise server→client requests end-to-end: the test acts as the
// MCP client, advertising sampling/roots and answering the server's
// server-initiated requests. propose_fix's verification is expected to fail on
// the throwaway diff — the point is to prove the bidirectional machinery and the
// sampling generator fire, not to land a verified patch.

const sampleDiff = "--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-bad\n+good\n"

func writeFixtureConfig(t *testing.T, dir string) string {
	t.Helper()
	configPath := filepath.Join(dir, "codeguard.json")
	body := `{
  "name": "mcp-sampling-test",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go"}],
  "checks": {"quality": true, "design": false, "security": false, "prompts": false, "ci": false},
  "output": {"format": "json"}
}`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func samplingResponse(t *testing.T, id json.RawMessage) string {
	t.Helper()
	resp, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"role":    "assistant",
			"content": map[string]any{"type": "text", "text": sampleDiff},
			"model":   "test-model",
		},
	})
	if err != nil {
		t.Fatalf("marshal sampling response: %v", err)
	}
	return string(resp)
}

// TestMCPStdioSampling drives propose_fix over stdio and answers the server's
// sampling/createMessage request.
func TestMCPStdioSampling(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFixtureConfig(t, dir)

	cmd, stdin, stdout := startStdioSamplingServer(t, cfg)
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	write := func(line string) {
		if _, err := io.WriteString(stdin, line+"\n"); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	write(`{"jsonrpc":"2.0","id":"init","method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{"sampling":{}}}}`)
	write(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	write(`{"jsonrpc":"2.0","id":"fix","method":"tools/call","params":{"name":"propose_fix","arguments":{"finding":{"rule_id":"demo.rule","message":"bad value","path":"main.go","line":1}}}}`)

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	assertStdioSamplingRoundTrip(t, scanner, write)
}

// TestMCPHTTPSampling drives propose_fix over HTTP and answers the server's
// sampling request over the GET SSE stream.
func TestMCPHTTPSampling(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFixtureConfig(t, dir)
	base := startHTTPServerWithConfig(t, "", cfg)

	session := initializeHTTPSession(t, base, `{"sampling":{}}`)
	streamReader := openSessionStream(t, base, session)
	fixDone := make(chan string, 1)
	go func() {
		_, body := mcpPost(t, base, map[string]string{"Mcp-Session-Id": session},
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"propose_fix","arguments":{"finding":{"rule_id":"demo.rule","message":"bad value","path":"main.go","line":1}}}}`)
		fixDone <- body
	}()
	waitForSamplingRequest(t, streamReader, base, session)

	select {
	case <-fixDone:
		// propose_fix returned (verification may fail; the round trip is what we assert).
	case <-time.After(30 * time.Second):
		t.Fatalf("propose_fix did not complete after sampling answer")
	}
}

// TestMCPHTTPRootsConfinement proves the server fetches client roots and uses
// them to permit a config_path that is otherwise outside the allowed roots.
func TestMCPHTTPRootsConfinement(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFixtureConfig(t, dir)
	base := startHTTPServer(t, "") // default config; cfg lives in an out-of-tree temp dir

	session := initializeHTTPSession(t, base, `{"roots":{}}`)
	streamReader := openSessionStream(t, base, session)
	validateDone := make(chan string, 1)
	go func() {
		req := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"validate_config","arguments":{"config_path":"` + cfg + `"}}}`
		_, body := mcpPost(t, base, map[string]string{"Mcp-Session-Id": session}, req)
		validateDone <- body
	}()
	waitForRootsRequest(t, streamReader, base, session, dir)

	select {
	case body := <-validateDone:
		if strings.Contains(body, "not within an allowed root") {
			t.Fatalf("config_path was rejected despite being an advertised root: %s", body)
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("validate_config did not complete after roots answer")
	}

	secondDone := make(chan string, 1)
	go func() {
		req := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"validate_config","arguments":{"config_path":"` + cfg + `"}}}`
		_, body := mcpPost(t, base, map[string]string{"Mcp-Session-Id": session}, req)
		secondDone <- body
	}()

	select {
	case body := <-secondDone:
		if strings.Contains(body, "not within an allowed root") {
			t.Fatalf("cached-roots config_path was rejected: %s", body)
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("second validate_config did not complete")
	}
	assertNoSecondRootsRequest(t, streamReader)
}
