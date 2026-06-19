package mcp_test

import (
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
		assertInitializeCapabilities(t, openBase)
	})

	t.Run("tools-list-annotations", func(t *testing.T) {
		assertToolsListAnnotations(t, openBase)
	})

	t.Run("tool-call-streams-sse", func(t *testing.T) {
		assertToolCallStreamsSSE(t, openBase)
	})

	t.Run("resource-read", func(t *testing.T) {
		assertResourceRead(t, openBase)
	})

	t.Run("prompt-get", func(t *testing.T) {
		assertPromptGet(t, openBase)
	})

	t.Run("health-and-get-stream-needs-session", func(t *testing.T) {
		assertHealthAndMissingSession(t, openBase)
	})

	t.Run("auth-enforcement", func(t *testing.T) {
		assertAuthEnforcement(t, authBase)
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
