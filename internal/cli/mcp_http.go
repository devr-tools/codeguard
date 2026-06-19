package cli

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// mcp_http.go implements a Streamable-HTTP transport for the MCP server so
// remote, cloud-hosted hosts (e.g. Devin) can reach codeguard over a URL. It
// reuses the transport-neutral core in mcp_dispatch.go: synchronous methods go
// through dispatchSyncMethod and tools/call streams progress over SSE.

const (
	mcpHTTPMaxBodyBytes       = 4 << 20 // 4 MiB request cap
	mcpHTTPMaxConcurrentTools = 8       // concurrent tools/call executions
	mcpSessionHeader          = "Mcp-Session-Id"
	mcpDefaultAuthHeader      = "Authorization"
	mcpDefaultHTTPPath        = "/mcp"
	mcpHealthPath             = "/healthz"
	contentTypeJSON           = "application/json"
	contentTypeEventStream    = "text/event-stream"
)

// mcpAuthConfig configures optional static-bearer auth for the HTTP transport.
// A blank token disables auth (suitable only behind a private network).
type mcpAuthConfig struct {
	token  string
	header string
}

func (a mcpAuthConfig) enabled() bool { return strings.TrimSpace(a.token) != "" }

func (a mcpAuthConfig) headerName() string {
	if strings.TrimSpace(a.header) == "" {
		return mcpDefaultAuthHeader
	}
	return a.header
}

// authorize reports whether the request carries the expected credential. For
// the Authorization header it accepts an optional "Bearer " scheme prefix.
func (a mcpAuthConfig) authorize(r *http.Request) bool {
	if !a.enabled() {
		return true
	}
	got := strings.TrimSpace(r.Header.Get(a.headerName()))
	if strings.EqualFold(a.headerName(), mcpDefaultAuthHeader) {
		if rest := strings.TrimSpace(strings.TrimPrefix(got, "Bearer ")); len(rest) != len(got) {
			got = rest
		} else if rest := strings.TrimSpace(strings.TrimPrefix(got, "bearer ")); len(rest) != len(got) {
			got = rest
		}
	}
	want := strings.TrimSpace(a.token)
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

type mcpHTTPHandler struct {
	tools    *mcpToolService
	auth     mcpAuthConfig
	path     string
	sem      chan struct{}
	sessions *sessionRegistry
}

// newMCPHTTPHandler builds the HTTP handler for the MCP server. It is split out
// from the listener so tests can mount it on httptest.NewServer.
func newMCPHTTPHandler(tools *mcpToolService, auth mcpAuthConfig, path string) http.Handler {
	if strings.TrimSpace(path) == "" {
		path = mcpDefaultHTTPPath
	}
	h := &mcpHTTPHandler{
		tools:    tools,
		auth:     auth,
		path:     path,
		sem:      make(chan struct{}, mcpHTTPMaxConcurrentTools),
		sessions: newSessionRegistry(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(mcpHealthPath, h.handleHealth)
	mux.HandleFunc(path, h.handleMCP)
	return mux
}

// callerForRequest returns the client caller bound to the request's session, or
// nil when no session/header is present.
func (h *mcpHTTPHandler) callerForRequest(r *http.Request) clientCaller {
	sess, ok := h.sessions.get(r.Header.Get(mcpSessionHeader))
	if !ok {
		return nil
	}
	return sess.caller()
}

func (h *mcpHTTPHandler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok\n")
}

func (h *mcpHTTPHandler) handleMCP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.handlePost(w, r)
	case http.MethodGet:
		h.handleGetStream(w, r)
	case http.MethodDelete:
		if !h.auth.authorize(r) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		h.sessions.remove(r.Header.Get(mcpSessionHeader))
		w.WriteHeader(http.StatusOK)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleGetStream opens the server→client SSE stream for a session. The server
// writes sampling/createMessage and roots/list requests over this stream; the
// client answers them on subsequent POSTs.
func (h *mcpHTTPHandler) handleGetStream(w http.ResponseWriter, r *http.Request) {
	if !h.auth.authorize(r) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	sess, ok := h.sessions.get(r.Header.Get(mcpSessionHeader))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentTypeEventStream)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	sess.attachStream(w, flusher)
	defer sess.detachStream()
	// Emit an SSE comment so the client knows the stream is attached and the
	// server can now deliver server-initiated requests over it.
	_, _ = io.WriteString(w, ": ready\n\n")
	flusher.Flush()
	<-r.Context().Done()
	// The client disconnected; drop the session so the map does not retain dead
	// sessions (the common client keeps one stream open for the session's life).
	h.sessions.remove(sess.id)
}

func (h *mcpHTTPHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	if !h.auth.authorize(r) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeJSONStatus(w, http.StatusUnauthorized, buildErrorMessage(nil, -32001, "unauthorized"))
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, mcpHTTPMaxBodyBytes))
	if err != nil {
		writeJSONStatus(w, http.StatusRequestEntityTooLarge, buildErrorMessage(nil, -32600, "request too large"))
		return
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		writeJSONStatus(w, http.StatusBadRequest, buildErrorMessage(nil, -32700, "parse error"))
		return
	}

	// JSON-RPC batch (array) — process each message and return an array of the
	// non-notification responses. Batched requests do not stream progress.
	if strings.HasPrefix(trimmed, "[") {
		h.handleBatch(w, r, []byte(trimmed))
		return
	}

	var req mcpRequest
	if err := json.Unmarshal([]byte(trimmed), &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, buildErrorMessage(nil, -32700, "parse error"))
		return
	}
	if req.JSONRPC != "2.0" {
		writeJSONStatus(w, http.StatusBadRequest, buildErrorMessage(req.idPtr(), -32600, "invalid request"))
		return
	}

	// A response (id, no method) is the client answering a server-initiated
	// request; route it to the session's pending caller.
	if isResponseMessage(req) {
		if sess, ok := h.sessions.get(r.Header.Get(mcpSessionHeader)); ok {
			sess.requester.deliver(decodeIDKey(req.ID), json.RawMessage(trimmed))
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if req.Method == "notifications/roots/list_changed" {
		if sess, ok := h.sessions.get(r.Header.Get(mcpSessionHeader)); ok {
			sess.roots.invalidate()
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Notifications carry no id and expect no response body.
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if req.Method == "initialize" {
		sess := h.sessions.create(parseClientCapabilities(req.Params))
		w.Header().Set(mcpSessionHeader, sess.id)
	}

	if req.Method == "tools/call" {
		h.streamToolCall(w, r, req)
		return
	}

	msg, handled := h.tools.dispatchSyncMethod(req.Method, req.ID, req.Params)
	if !handled {
		writeJSONStatus(w, http.StatusOK, buildErrorMessage(req.idPtr(), -32601, "method not found"))
		return
	}
	writeJSONStatus(w, http.StatusOK, msg)
}

func (h *mcpHTTPHandler) handleBatch(w http.ResponseWriter, r *http.Request, body []byte) {
	var reqs []mcpRequest
	if err := json.Unmarshal(body, &reqs); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, buildErrorMessage(nil, -32700, "parse error"))
		return
	}
	ctx := withClientCaller(r.Context(), h.callerForRequest(r))
	responses := make([]map[string]any, 0, len(reqs))
	for _, req := range reqs {
		if req.JSONRPC != "2.0" {
			responses = append(responses, buildErrorMessage(req.idPtr(), -32600, "invalid request"))
			continue
		}
		if len(req.ID) == 0 {
			continue // notification — no response
		}
		responses = append(responses, h.produceMessage(ctx, req))
	}
	if len(responses) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeJSONStatus(w, http.StatusOK, responses)
}

// produceMessage resolves a single request to its response envelope, running
// tools/call synchronously (no progress streaming). Used for batched requests.
func (h *mcpHTTPHandler) produceMessage(ctx context.Context, req mcpRequest) map[string]any {
	if req.Method == "tools/call" {
		if err := h.acquire(ctx); err != nil {
			return buildErrorMessage(req.idPtr(), -32603, "server busy")
		}
		defer h.release()
		result, err := h.tools.callToolWithContext(ctx, req.Params)
		if err != nil {
			return buildErrorMessage(req.idPtr(), -32602, err.Error())
		}
		return buildResultMessage(req.ID, result)
	}
	msg, handled := h.tools.dispatchSyncMethod(req.Method, req.ID, req.Params)
	if !handled {
		return buildErrorMessage(req.idPtr(), -32601, "method not found")
	}
	return msg
}

// streamToolCall runs a tools/call and streams progress + result as SSE so the
// HTTP transport matches the stdio transport's progress behavior. Cancellation
// is driven by the request context (client disconnect).
func (h *mcpHTTPHandler) streamToolCall(w http.ResponseWriter, r *http.Request, req mcpRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		// No streaming support — fall back to a single JSON response.
		writeJSONStatus(w, http.StatusOK, h.produceMessage(r.Context(), req))
		return
	}

	if err := h.acquire(r.Context()); err != nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, buildErrorMessage(req.idPtr(), -32603, "server busy"))
		return
	}
	defer h.release()

	w.Header().Set("Content-Type", contentTypeEventStream)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	ctx := withClientCaller(r.Context(), h.callerForRequest(r))
	progressToken := progressTokenFromParams(req.Params)
	if progressToken != nil {
		_ = writeSSE(w, buildProgressMessage(*progressToken, 0, 1, "Started"))
		flusher.Flush()
		var progressMu sync.Mutex
		ctx = withProgress(ctx, func(progress float64, total float64, message string) {
			progressMu.Lock()
			defer progressMu.Unlock()
			_ = writeSSE(w, buildProgressMessage(*progressToken, progress, total, message))
			flusher.Flush()
		})
	}

	result, err := h.tools.callToolWithContext(ctx, req.Params)

	if progressToken != nil {
		message := "Completed"
		if err != nil {
			message = "Stopped"
		}
		_ = writeSSE(w, buildProgressMessage(*progressToken, 1, 1, message))
		flusher.Flush()
	}

	var msg map[string]any
	if err != nil {
		msg = buildErrorMessage(req.idPtr(), -32602, err.Error())
	} else {
		msg = buildResultMessage(req.ID, result)
	}
	_ = writeSSE(w, msg)
	flusher.Flush()
}

func (h *mcpHTTPHandler) acquire(ctx context.Context) error {
	select {
	case h.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *mcpHTTPHandler) release() { <-h.sem }

func writeJSONStatus(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = w.Write(data)
	_, _ = w.Write([]byte("\n"))
}

func writeSSE(w io.Writer, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	return err
}

// newSessionID returns a random hex session identifier.
func newSessionID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "codeguard-session"
	}
	return hex.EncodeToString(buf)
}
