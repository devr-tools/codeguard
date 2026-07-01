package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// serverRequestTimeout bounds how long the server waits for a client to answer
// a server-initiated request (sampling/createMessage, roots/list).
const serverRequestTimeout = 120 * time.Second

// serverRequester correlates server-initiated requests with the client's
// responses. The transport writes the outbound request (via the send closure
// passed to call) and feeds inbound responses back through deliver.
type serverRequester struct {
	mu      sync.Mutex
	pending map[string]chan json.RawMessage
	counter int
}

func newServerRequester() *serverRequester {
	return &serverRequester{pending: map[string]chan json.RawMessage{}}
}

// call issues a server-initiated request: it allocates an id, lets send write
// the request to the transport, and waits for the matching response (or a
// timeout / context cancellation).
func (r *serverRequester) call(ctx context.Context, send func(id string) error) (json.RawMessage, error) {
	r.mu.Lock()
	r.counter++
	id := fmt.Sprintf("srv-%d", r.counter)
	ch := make(chan json.RawMessage, 1)
	r.pending[id] = ch
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.pending, id)
		r.mu.Unlock()
	}()

	if err := send(id); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, serverRequestTimeout)
	defer cancel()
	select {
	case resp := <-ch:
		return parseServerResponse(resp)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// deliver routes an inbound response to the call awaiting that id. It is a no-op
// when no server-initiated request is pending for the id.
func (r *serverRequester) deliver(id string, raw json.RawMessage) {
	r.mu.Lock()
	ch, ok := r.pending[id]
	r.mu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- raw:
	default:
	}
}

func parseServerResponse(raw json.RawMessage) (json.RawMessage, error) {
	var env struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Error != nil {
		return nil, fmt.Errorf("client error %d: %s", env.Error.Code, env.Error.Message)
	}
	return env.Result, nil
}

// isResponseMessage reports whether an inbound JSON-RPC message is a response to
// a server-initiated request: it carries an id and no method.
func isResponseMessage(req mcpRequest) bool {
	return len(req.ID) > 0 && strings.TrimSpace(req.Method) == ""
}

// decodeIDKey canonicalizes a JSON-RPC id into the string key used by
// serverRequester (which issues string ids like "srv-1").
func decodeIDKey(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

// parseClientCapabilities extracts the capabilities object the client advertised
// during initialize, so the server knows whether sampling/roots are available.
func parseClientCapabilities(raw json.RawMessage) map[string]any {
	var params struct {
		Capabilities map[string]any `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil
	}
	return params.Capabilities
}
