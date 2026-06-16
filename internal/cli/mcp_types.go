package cli

import (
	"context"
	"encoding/json"
	"sync"
)

const (
	mcpProtocolVersionCurrent = "2025-11-25"
	mcpProtocolVersionCompat  = "2025-06-18"
)

type mcpServer struct {
	defaultConfigPath string
	defaultProfile    string
	initializeSeen    bool
	mu                sync.Mutex
	active            map[string]context.CancelFunc
	cancelled         map[string]bool
	wg                sync.WaitGroup
	responder         *mcpResponder
	tools             *mcpToolService
}

type mcpResponder struct {
	writeMu sync.Mutex
}

type mcpToolService struct {
	defaultConfigPath string
	defaultProfile    string
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name         string         `json:"name"`
	Title        string         `json:"title,omitempty"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
}
