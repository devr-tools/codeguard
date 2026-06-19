package cli

import (
	"encoding/json"

	"github.com/devr-tools/codeguard/internal/version"
)

// This file holds the transport-neutral core shared by the stdio (mcp_run.go)
// and HTTP (mcp_http.go) transports: JSON-RPC message builders, the advertised
// capability set, and a synchronous method router for everything except
// tools/call (which each transport drives with its own concurrency model).

const mcpServerInstructions = "Use validate_patch before writing files to disk when you want policy feedback on a proposed diff. Read codeguard://rules for the rule catalog and prompts/get for review workflows."

// buildResultMessage constructs a JSON-RPC result envelope. Callers that handle
// notifications (empty id) must skip writing the message themselves.
func buildResultMessage(id json.RawMessage, result any) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"result":  result,
	}
}

// buildErrorMessage constructs a JSON-RPC error envelope. A nil id serializes to
// a null id, matching the JSON-RPC spec for unparseable requests.
func buildErrorMessage(id *json.RawMessage, code int, message string) map[string]any {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"error":   mcpError{Code: code, Message: message},
	}
	if id != nil {
		payload["id"] = json.RawMessage(*id)
	} else {
		payload["id"] = nil
	}
	return payload
}

// buildProgressMessage constructs a notifications/progress envelope.
func buildProgressMessage(token json.RawMessage, progress float64, total float64, message string) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/progress",
		"params": map[string]any{
			"progressToken": json.RawMessage(token),
			"progress":      progress,
			"total":         total,
			"message":       message,
		},
	}
}

// serverCapabilities is the capability set advertised during initialize. It is
// shared so both transports report identical capabilities.
func serverCapabilities() map[string]any {
	return map[string]any{
		"tools":     map[string]any{},
		"resources": map[string]any{},
		"prompts":   map[string]any{},
		"logging":   map[string]any{},
	}
}

// buildInitializeResult builds the initialize response payload, negotiating the
// protocol version from the client's requested version.
func buildInitializeResult(params json.RawMessage) map[string]any {
	return map[string]any{
		"protocolVersion": negotiateMCPProtocolVersion(params),
		"capabilities":    serverCapabilities(),
		"serverInfo": map[string]any{
			"name":    "codeguard",
			"title":   "CodeGuard MCP Server",
			"version": version.Number,
		},
		"instructions": mcpServerInstructions,
	}
}

// dispatchSyncMethod handles every request method that produces a single,
// synchronous response (i.e. everything except tools/call, which streams
// progress). It returns the JSON-RPC result envelope and true when it handled
// the method; handled is false for unknown methods so the caller can emit a
// method-not-found error. tools/call must be routed by the transport before
// calling this.
//
// Notifications (notifications/initialized, notifications/cancelled) are not
// handled here — they carry no id and are transport-specific.
func (s *mcpToolService) dispatchSyncMethod(method string, id json.RawMessage, params json.RawMessage) (map[string]any, bool) {
	if msg, ok := staticSyncMethod(method, id, params); ok {
		return msg, true
	}
	if msg, ok := s.dispatchResourceMethod(method, id, params); ok {
		return msg, true
	}
	if msg, ok := dispatchPromptMethod(method, id, params); ok {
		return msg, true
	}
	if method == "logging/setLevel" {
		return buildResultMessage(id, map[string]any{}), true
	}
	return nil, false
}

func staticSyncMethod(method string, id json.RawMessage, params json.RawMessage) (map[string]any, bool) {
	handlers := map[string]func() map[string]any{
		"initialize": func() map[string]any { return buildResultMessage(id, buildInitializeResult(params)) },
		"ping":       func() map[string]any { return buildResultMessage(id, map[string]any{}) },
		"tools/list": func() map[string]any { return buildResultMessage(id, map[string]any{"tools": mcpTools()}) },
	}
	handler, ok := handlers[method]
	if !ok {
		return nil, false
	}
	return handler(), true
}

// ptrID returns a pointer to a copy of id, or nil when id is empty, for use with
// buildErrorMessage.
func ptrID(id json.RawMessage) *json.RawMessage {
	if len(id) == 0 {
		return nil
	}
	cp := id
	return &cp
}
