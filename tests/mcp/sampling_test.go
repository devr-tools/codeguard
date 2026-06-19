package mcp_test

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
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

	cmd := exec.Command(os.Args[0], "-test.run=TestMCPServeHelperProcess", "--", cfg)
	cmd.Env = append(os.Environ(), "GO_WANT_MCP_HELPER_PROCESS=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
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

	type outcome struct {
		sawSampling bool
		sawFix      bool
	}
	result := make(chan outcome, 1)
	go func() {
		var out outcome
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var msg struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
			}
			if json.Unmarshal([]byte(line), &msg) != nil {
				continue
			}
			if msg.Method == "sampling/createMessage" {
				out.sawSampling = true
				write(samplingResponse(t, msg.ID))
				continue
			}
			if strings.TrimSpace(string(msg.ID)) == `"fix"` {
				out.sawFix = true
				result <- out
				return
			}
		}
		result <- out
	}()

	select {
	case out := <-result:
		if !out.sawSampling {
			t.Fatalf("server did not issue sampling/createMessage")
		}
		if !out.sawFix {
			t.Fatalf("propose_fix never returned a result")
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("timed out waiting for sampling round trip")
	}
}

// TestMCPHTTPSampling drives propose_fix over HTTP and answers the server's
// sampling request over the GET SSE stream.
func TestMCPHTTPSampling(t *testing.T) {
	dir := t.TempDir()
	cfg := writeFixtureConfig(t, dir)
	base := startHTTPServerWithConfig(t, "", cfg)

	// initialize, advertising sampling, to obtain a session id.
	resp, _ := mcpPost(t, base, nil, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{"sampling":{}}}}`)
	session := resp.Header.Get("Mcp-Session-Id")
	if session == "" {
		t.Fatalf("no session id returned")
	}

	// Open the server→client SSE stream and wait for the readiness comment.
	streamReq, _ := http.NewRequest(http.MethodGet, base+"/mcp", nil)
	streamReq.Header.Set("Mcp-Session-Id", session)
	streamResp, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	t.Cleanup(func() { _ = streamResp.Body.Close() })
	streamReader := bufio.NewReader(streamResp.Body)
	if _, err := streamReader.ReadString('\n'); err != nil { // ": ready"
		t.Fatalf("read stream readiness: %v", err)
	}

	// Fire propose_fix; it blocks server-side until we answer the sampling request.
	fixDone := make(chan string, 1)
	go func() {
		_, body := mcpPost(t, base, map[string]string{"Mcp-Session-Id": session},
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"propose_fix","arguments":{"finding":{"rule_id":"demo.rule","message":"bad value","path":"main.go","line":1}}}}`)
		fixDone <- body
	}()

	// Read the sampling request off the stream and answer it on a POST.
	sawSampling := false
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) && !sawSampling {
		line, err := streamReader.ReadString('\n')
		if err != nil {
			t.Fatalf("read stream: %v", err)
		}
		line = strings.TrimSpace(line)
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(data)), &msg) != nil {
			continue
		}
		if msg.Method == "sampling/createMessage" {
			sawSampling = true
			mcpPost(t, base, map[string]string{"Mcp-Session-Id": session}, samplingResponse(t, msg.ID))
		}
	}
	if !sawSampling {
		t.Fatalf("server did not issue sampling/createMessage over the stream")
	}

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

	resp, _ := mcpPost(t, base, nil, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{"roots":{}}}}`)
	session := resp.Header.Get("Mcp-Session-Id")
	if session == "" {
		t.Fatalf("no session id returned")
	}

	streamReq, _ := http.NewRequest(http.MethodGet, base+"/mcp", nil)
	streamReq.Header.Set("Mcp-Session-Id", session)
	streamResp, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	t.Cleanup(func() { _ = streamResp.Body.Close() })
	streamReader := bufio.NewReader(streamResp.Body)
	if _, err := streamReader.ReadString('\n'); err != nil {
		t.Fatalf("read readiness: %v", err)
	}

	// validate_config with an out-of-tree config_path; the server must fetch
	// roots to decide whether it is permitted.
	validateDone := make(chan string, 1)
	go func() {
		req := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"validate_config","arguments":{"config_path":"` + cfg + `"}}}`
		_, body := mcpPost(t, base, map[string]string{"Mcp-Session-Id": session}, req)
		validateDone <- body
	}()

	// Answer the server's roots/list request, advertising the temp dir as a root.
	sawRoots := false
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && !sawRoots {
		line, err := streamReader.ReadString('\n')
		if err != nil {
			t.Fatalf("read stream: %v", err)
		}
		data, ok := strings.CutPrefix(strings.TrimSpace(line), "data:")
		if !ok {
			continue
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(data)), &msg) != nil {
			continue
		}
		if msg.Method == "roots/list" {
			sawRoots = true
			rootsResp, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg.ID,
				"result":  map[string]any{"roots": []map[string]any{{"uri": "file://" + dir, "name": "temp"}}},
			})
			mcpPost(t, base, map[string]string{"Mcp-Session-Id": session}, string(rootsResp))
		}
	}
	if !sawRoots {
		t.Fatalf("server did not issue roots/list")
	}

	select {
	case body := <-validateDone:
		// With the temp dir advertised as a root, confinement must permit the
		// path — so the response must not be the "not within an allowed root" error.
		if strings.Contains(body, "not within an allowed root") {
			t.Fatalf("config_path was rejected despite being an advertised root: %s", body)
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("validate_config did not complete after roots answer")
	}

	// A second config_path call must reuse the cached roots — no new roots/list.
	secondDone := make(chan string, 1)
	go func() {
		req := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"validate_config","arguments":{"config_path":"` + cfg + `"}}}`
		_, body := mcpPost(t, base, map[string]string{"Mcp-Session-Id": session}, req)
		secondDone <- body
	}()

	secondRoots := make(chan struct{}, 1)
	go func() {
		for {
			line, err := streamReader.ReadString('\n')
			if err != nil {
				return
			}
			data, ok := strings.CutPrefix(strings.TrimSpace(line), "data:")
			if !ok {
				continue
			}
			var msg struct {
				Method string `json:"method"`
			}
			if json.Unmarshal([]byte(strings.TrimSpace(data)), &msg) == nil && msg.Method == "roots/list" {
				secondRoots <- struct{}{}
				return
			}
		}
	}()

	select {
	case body := <-secondDone:
		if strings.Contains(body, "not within an allowed root") {
			t.Fatalf("cached-roots config_path was rejected: %s", body)
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("second validate_config did not complete")
	}
	select {
	case <-secondRoots:
		t.Fatalf("server issued a second roots/list instead of using the cache")
	case <-time.After(500 * time.Millisecond):
		// no second roots/list — cache hit, as expected
	}
}
