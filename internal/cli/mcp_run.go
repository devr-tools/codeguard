package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func runServe(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	mcpMode := fs.Bool("mcp", false, "serve an MCP server")
	httpMode := fs.Bool("http", false, "serve MCP over Streamable HTTP instead of stdio")
	addr := fs.String("addr", "127.0.0.1:8080", "HTTP listen address (with --http)")
	mcpPath := fs.String("mcp-path", mcpDefaultHTTPPath, "HTTP path for the MCP endpoint (with --http)")
	authToken := fs.String("auth-token", "", "optional static bearer token required on HTTP requests; falls back to $CODEGUARD_MCP_AUTH_TOKEN")
	authHeader := fs.String("auth-header", mcpDefaultAuthHeader, "HTTP header carrying the auth token (with --http)")
	configPath := fs.String("config", service.DefaultConfigPath(), "default config file or directory path")
	profile := fs.String("profile", "", "optional default policy profile override")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !*mcpMode {
		_, _ = fmt.Fprintln(stderr, "serve currently requires --mcp")
		return 1
	}

	tools := &mcpToolService{
		defaultConfigPath: *configPath,
		defaultProfile:    *profile,
	}

	if *httpMode {
		token := *authToken
		if strings.TrimSpace(token) == "" {
			token = strings.TrimSpace(os.Getenv("CODEGUARD_MCP_AUTH_TOKEN"))
		}
		auth := mcpAuthConfig{token: token, header: *authHeader}
		if err := serveMCPHTTP(*addr, *mcpPath, tools, auth, stderr); err != nil {
			_, _ = fmt.Fprintf(stderr, "mcp http server failed: %v\n", err)
			return 1
		}
		return 0
	}

	server := mcpServer{
		defaultConfigPath: *configPath,
		defaultProfile:    *profile,
		active:            map[string]context.CancelFunc{},
		cancelled:         map[string]bool{},
		responder:         &mcpResponder{},
		tools:             tools,
		requester:         newServerRequester(),
		rootsCache:        &rootsCache{},
	}
	if err := server.serve(stdin, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "mcp server failed: %v\n", err)
		return 1
	}
	return 0
}

// serveMCPHTTP runs the Streamable-HTTP transport until interrupted, then drains
// in-flight requests gracefully.
func serveMCPHTTP(addr string, path string, tools *mcpToolService, auth mcpAuthConfig, stderr io.Writer) error {
	handler := newMCPHTTPHandler(tools, auth, path)
	// Set server timeouts to defend against Slowloris-style attacks, where a
	// slow client trickles request bytes to exhaust connection slots.
	// ReadHeaderTimeout is the key defense; the others bound overall request,
	// response, and idle-connection lifetimes. WriteTimeout is generous because
	// MCP tool responses can be large and stream over a longer scan.
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		authNote := "auth disabled"
		if auth.enabled() {
			authNote = "auth required via " + auth.headerName()
		}
		_, _ = fmt.Fprintf(stderr, "codeguard MCP HTTP server listening on %s%s (%s)\n", addr, path, authNote)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func (s *mcpServer) serve(stdin io.Reader, stdout io.Writer) error {
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := s.handleLine(line, stdout); err != nil {
			return err
		}
	}
	s.wg.Wait()
	return scanner.Err()
}

func (s *mcpServer) handleLine(line string, stdout io.Writer) error {
	var req mcpRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		return s.responder.writeError(stdout, nil, -32700, "parse error")
	}
	if req.JSONRPC != "2.0" {
		return s.responder.writeError(stdout, req.idPtr(), -32600, "invalid request")
	}
	// Responses to server-initiated requests (sampling/createMessage,
	// roots/list) carry an id and no method; route them to the waiting caller.
	if isResponseMessage(req) {
		s.requester.deliver(decodeIDKey(req.ID), json.RawMessage(line))
		return nil
	}
	return s.handleRequestMethod(req, stdout)
}

func (s *mcpServer) handleRequestMethod(req mcpRequest, stdout io.Writer) error {
	if handled, err := s.handleAsyncRequest(req, stdout); handled {
		return err
	}
	switch req.Method {
	case "ping":
		return s.responder.writeResult(stdout, req.ID, map[string]any{})
	case "tools/list":
		return s.handleToolsList(req, stdout)
	case "tools/call":
		return s.handleToolsCallRequest(req, stdout)
	case "resources/list", "resources/templates/list", "resources/read",
		"prompts/list", "prompts/get", "logging/setLevel":
		return s.handleSyncMethod(req, stdout)
	default:
		if len(req.ID) == 0 {
			return nil
		}
		return s.responder.writeError(stdout, req.idPtr(), -32601, "method not found")
	}
}

func (s *mcpServer) handleAsyncRequest(req mcpRequest, stdout io.Writer) (bool, error) {
	switch req.Method {
	case "initialize":
		return true, s.handleInitializeResponse(req, stdout)
	case "notifications/initialized":
		return true, nil
	case "notifications/cancelled":
		s.handleCancelledNotification(req.Params)
		return true, nil
	case "notifications/roots/list_changed":
		s.rootsCache.invalidate()
		return true, nil
	default:
		return false, nil
	}
}

// handleSyncMethod serves the request methods that produce a single synchronous
// response (resources/*, prompts/*, logging/*) by delegating to the shared
// transport-neutral router and writing its envelope to stdout.
func (s *mcpServer) handleSyncMethod(req mcpRequest, stdout io.Writer) error {
	if !s.isInitialized() {
		return s.responder.writeError(stdout, req.idPtr(), -32002, "server not initialized")
	}
	msg, handled := s.tools.dispatchSyncMethod(req.Method, req.ID, req.Params)
	if !handled {
		return s.responder.writeError(stdout, req.idPtr(), -32601, "method not found")
	}
	return s.responder.writeMessage(stdout, msg)
}

func (s *mcpServer) handleInitializeResponse(req mcpRequest, stdout io.Writer) error {
	s.mu.Lock()
	s.initializeSeen = true
	s.clientCaps = parseClientCapabilities(req.Params)
	s.mu.Unlock()

	return s.responder.writeResult(stdout, req.ID, buildInitializeResult(req.Params))
}
