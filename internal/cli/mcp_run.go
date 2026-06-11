package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/devr-tools/codeguard/internal/version"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func runServe(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	mcpMode := fs.Bool("mcp", false, "serve an MCP server over stdio")
	configPath := fs.String("config", service.DefaultConfigPath(), "default config file or directory path")
	profile := fs.String("profile", "", "optional default policy profile override")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !*mcpMode {
		_, _ = fmt.Fprintln(stderr, "serve currently requires --mcp")
		return 1
	}

	server := mcpServer{
		defaultConfigPath: *configPath,
		defaultProfile:    *profile,
		active:            map[string]context.CancelFunc{},
		cancelled:         map[string]bool{},
		responder:         &mcpResponder{},
		tools: &mcpToolService{
			defaultConfigPath: *configPath,
			defaultProfile:    *profile,
		},
	}
	if err := server.serve(stdin, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "mcp server failed: %v\n", err)
		return 1
	}
	return 0
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
	return s.handleRequestMethod(req, stdout)
}

func (s *mcpServer) handleRequestMethod(req mcpRequest, stdout io.Writer) error {
	switch req.Method {
	case "initialize":
		return s.handleInitializeResponse(req, stdout)
	case "notifications/initialized":
		return nil
	case "notifications/cancelled":
		s.handleCancelledNotification(req.Params)
		return nil
	case "ping":
		return s.responder.writeResult(stdout, req.ID, map[string]any{})
	case "tools/list":
		return s.handleToolsList(req, stdout)
	case "tools/call":
		return s.handleToolsCallRequest(req, stdout)
	default:
		if len(req.ID) == 0 {
			return nil
		}
		return s.responder.writeError(stdout, req.idPtr(), -32601, "method not found")
	}
}

func (s *mcpServer) handleInitializeResponse(req mcpRequest, stdout io.Writer) error {
	s.mu.Lock()
	s.initializeSeen = true
	s.mu.Unlock()

	return s.responder.writeResult(stdout, req.ID, map[string]any{
		"protocolVersion": negotiateMCPProtocolVersion(req.Params),
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "codeguard",
			"title":   "CodeGuard MCP Server",
			"version": version.Number,
		},
		"instructions": "Use validate_patch before writing files to disk when you want policy feedback on a proposed diff.",
	})
}
