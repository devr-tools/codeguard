package cli

import (
	"context"
	"encoding/json"
	"fmt"
)

// clientBridge is the transport-agnostic clientCaller. Each transport supplies
// the client capabilities, a serverRequester, a send closure that writes a
// server→client message over that transport, and a per-connection roots cache.
type clientBridge struct {
	caps      map[string]any
	requester *serverRequester
	send      func(payload any) error
	roots     *rootsCache
}

func (b *clientBridge) supports(capability string) bool {
	if b == nil || b.caps == nil {
		return false
	}
	_, ok := b.caps[capability]
	return ok
}

func (b *clientBridge) sampleMessage(ctx context.Context, params map[string]any) (json.RawMessage, error) {
	if !b.supports("sampling") {
		return nil, fmt.Errorf("client does not support sampling")
	}
	return b.requester.call(ctx, func(id string) error {
		return b.send(map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  "sampling/createMessage",
			"params":  params,
		})
	})
}

func (b *clientBridge) listRoots(ctx context.Context) ([]mcpRoot, error) {
	if !b.supports("roots") {
		return nil, fmt.Errorf("client does not support roots")
	}
	if b.roots != nil {
		return b.roots.load(func() ([]mcpRoot, error) { return b.fetchRoots(ctx) })
	}
	return b.fetchRoots(ctx)
}

func (b *clientBridge) elicit(ctx context.Context, message string, schema map[string]any) (elicitResult, error) {
	if !b.supports("elicitation") {
		return elicitResult{}, fmt.Errorf("client does not support elicitation")
	}
	raw, err := b.requester.call(ctx, func(id string) error {
		return b.send(map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  "elicitation/create",
			"params": map[string]any{
				"message":         message,
				"requestedSchema": schema,
			},
		})
	})
	if err != nil {
		return elicitResult{}, err
	}
	var result elicitResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return elicitResult{}, err
	}
	return result, nil
}

func (b *clientBridge) fetchRoots(ctx context.Context) ([]mcpRoot, error) {
	raw, err := b.requester.call(ctx, func(id string) error {
		return b.send(map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  "roots/list",
			"params":  map[string]any{},
		})
	})
	if err != nil {
		return nil, err
	}
	var result struct {
		Roots []mcpRoot `json:"roots"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result.Roots, nil
}
