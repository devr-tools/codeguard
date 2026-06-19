package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// serverRequestTimeout bounds how long the server waits for a client to answer
// a server-initiated request (sampling/createMessage, roots/list).
const serverRequestTimeout = 120 * time.Second

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

type clientCallerCtxKey struct{}

func withClientCaller(ctx context.Context, c clientCaller) context.Context {
	if c == nil {
		return ctx
	}
	return context.WithValue(ctx, clientCallerCtxKey{}, c)
}

func clientCallerFrom(ctx context.Context) clientCaller {
	c, _ := ctx.Value(clientCallerCtxKey{}).(clientCaller)
	return c
}

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

// deliver routes an inbound response to a waiting call. It returns false when no
// server-initiated request is awaiting that id (so the caller can dispatch the
// message normally).
func (r *serverRequester) deliver(id string, raw json.RawMessage) bool {
	r.mu.Lock()
	ch, ok := r.pending[id]
	r.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- raw:
	default:
	}
	return true
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

// rootsCache memoizes the client's roots per connection so a config-path check
// does not issue a fresh roots/list round trip every time. It is invalidated on
// notifications/roots/list_changed.
type rootsCache struct {
	mu     sync.Mutex
	loaded bool
	roots  []mcpRoot
}

func (c *rootsCache) invalidate() {
	c.mu.Lock()
	c.loaded = false
	c.roots = nil
	c.mu.Unlock()
}

// load returns the cached roots, fetching once via fetch on a miss. The lock is
// held across fetch so concurrent callers coalesce into a single round trip.
func (c *rootsCache) load(fetch func() ([]mcpRoot, error)) ([]mcpRoot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded {
		return c.roots, nil
	}
	roots, err := fetch()
	if err != nil {
		return nil, err
	}
	c.roots = roots
	c.loaded = true
	return roots, nil
}

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

// mcp_client.go holds the per-call transport callbacks that the tool layer
// reaches through context: a progress emitter (for streaming partial findings)
// and, added in a later phase, a client caller (for sampling/roots). Threading
// them via context keeps the many tool method signatures unchanged, and both
// are nil-safe so tools work when a transport does not supply them.

type progressFunc func(progress float64, total float64, message string)

type progressCtxKey struct{}

func withProgress(ctx context.Context, fn progressFunc) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, progressCtxKey{}, fn)
}

func progressFrom(ctx context.Context) progressFunc {
	fn, _ := ctx.Value(progressCtxKey{}).(progressFunc)
	return fn
}

type clientRootsCtxKey struct{}

// withClientRoots attaches the filesystem roots the connected client advertised
// (via the roots capability) so config-path confinement can permit them.
func withClientRoots(ctx context.Context, roots []string) context.Context {
	if len(roots) == 0 {
		return ctx
	}
	return context.WithValue(ctx, clientRootsCtxKey{}, roots)
}

func clientRootsFrom(ctx context.Context) []string {
	roots, _ := ctx.Value(clientRootsCtxKey{}).([]string)
	return roots
}

// countEnabledSections returns a best-effort count of the scan sections that
// will run for a config, used as the `total` for per-section progress. It is a
// hint, not a guarantee — if it drifts from the runner the progress bar simply
// may not land exactly on 100%.
func countEnabledSections(cfg service.Config, mode service.ScanMode) float64 {
	count := 0
	if cfg.Checks.Quality {
		count++
	}
	if cfg.Checks.Design {
		count++
	}
	if cfg.Checks.Security {
		count++
	}
	if cfg.Checks.Prompts {
		count++
	}
	if cfg.Checks.CI {
		count++
	}
	if cfg.Checks.SupplyChain {
		count++
	}
	if cfg.Checks.Contracts != nil {
		if *cfg.Checks.Contracts {
			count++
		}
	} else if mode == service.ScanModeDiff {
		count++
	}
	for _, pack := range cfg.RulePacks {
		if len(pack.Rules) > 0 {
			count++
			break
		}
	}
	if count == 0 {
		count = 1
	}
	return float64(count)
}
