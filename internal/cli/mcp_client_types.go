package cli

import (
	"context"
	"encoding/json"
)

// clientCaller is the server→client capability surface used by tools: it reports
// the client's advertised capabilities and issues server-initiated requests
// (sampling, roots). Implemented per transport by clientBridge.
type clientCaller interface {
	supports(capability string) bool
	sampleMessage(ctx context.Context, params map[string]any) (json.RawMessage, error)
	listRoots(ctx context.Context) ([]mcpRoot, error)
	elicit(ctx context.Context, message string, schema map[string]any) (elicitResult, error)
}

// elicitResult is the client's answer to an elicitation/create request.
type elicitResult struct {
	Action  string          `json:"action"` // "accept" | "decline" | "cancel"
	Content json.RawMessage `json:"content"`
}

func (e elicitResult) accepted() bool { return e.Action == "accept" }

type mcpRoot struct {
	URI  string `json:"uri"`
	Name string `json:"name,omitempty"`
}
