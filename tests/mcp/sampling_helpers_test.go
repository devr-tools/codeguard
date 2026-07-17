package mcp_test

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func initializeHTTPSession(t *testing.T, base string, capabilities string) string {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":` + capabilities + `}}`
	resp, _ := mcpPost(t, base, nil, body)
	session := resp.Header.Get("Mcp-Session-Id")
	if session == "" {
		t.Fatalf("no session id returned")
	}
	return session
}

func openSessionStream(t *testing.T, base string, session string) *bufio.Reader {
	t.Helper()
	streamReader, _ := openClosableSessionStream(t, base, session)
	return streamReader
}

func openClosableSessionStream(t *testing.T, base string, session string) (*bufio.Reader, io.ReadCloser) {
	t.Helper()
	streamReq, _ := http.NewRequest(http.MethodGet, base+"/mcp", nil)
	streamReq.Header.Set("Mcp-Session-Id", session)
	streamResp, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	t.Cleanup(func() { _ = streamResp.Body.Close() })
	streamReader := bufio.NewReader(streamResp.Body)
	if _, err := streamReader.ReadString('\n'); err != nil {
		t.Fatalf("read stream readiness: %v", err)
	}
	return streamReader, streamResp.Body
}

func waitForSamplingRequest(t *testing.T, streamReader *bufio.Reader, base string, session string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
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
		if json.Unmarshal([]byte(strings.TrimSpace(data)), &msg) == nil && msg.Method == "sampling/createMessage" {
			mcpPost(t, base, map[string]string{"Mcp-Session-Id": session}, samplingResponse(t, msg.ID))
			return
		}
	}
	t.Fatalf("server did not issue sampling/createMessage over the stream")
}

func waitForRootsRequest(t *testing.T, streamReader *bufio.Reader, base string, session string, dir string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
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
		if json.Unmarshal([]byte(strings.TrimSpace(data)), &msg) == nil && msg.Method == "roots/list" {
			rootsResp, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg.ID,
				"result":  map[string]any{"roots": []map[string]any{{"uri": "file://" + dir, "name": "temp"}}},
			})
			mcpPost(t, base, map[string]string{"Mcp-Session-Id": session}, string(rootsResp))
			return
		}
	}
	t.Fatalf("server did not issue roots/list")
}

func assertNoSecondRootsRequest(t *testing.T, streamReader *bufio.Reader) {
	t.Helper()
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
	case <-secondRoots:
		t.Fatalf("server issued a second roots/list instead of using the cache")
	case <-time.After(500 * time.Millisecond):
	}
}
