package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
)

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
	_, _ = io.WriteString(w, ": ready\n\n")
	flusher.Flush()
	<-r.Context().Done()
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
	if strings.HasPrefix(trimmed, "[") {
		h.handleBatch(w, r, []byte(trimmed))
		return
	}

	var req mcpRequest
	if err := json.Unmarshal([]byte(trimmed), &req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, buildErrorMessage(nil, -32700, "parse error"))
		return
	}
	status, handled := h.handleSpecialPost(w, r, req, trimmed)
	if handled {
		if status > 0 {
			w.WriteHeader(status)
		}
		return
	}

	msg, ok := h.tools.dispatchSyncMethod(req.Method, req.ID, req.Params)
	if !ok {
		writeJSONStatus(w, http.StatusOK, buildErrorMessage(req.idPtr(), -32601, "method not found"))
		return
	}
	writeJSONStatus(w, http.StatusOK, msg)
}

func (h *mcpHTTPHandler) handleSpecialPost(w http.ResponseWriter, r *http.Request, req mcpRequest, trimmed string) (int, bool) {
	if req.JSONRPC != "2.0" {
		writeJSONStatus(w, http.StatusBadRequest, buildErrorMessage(req.idPtr(), -32600, "invalid request"))
		return 0, true
	}
	if isResponseMessage(req) {
		if sess, ok := h.sessions.get(r.Header.Get(mcpSessionHeader)); ok {
			sess.requester.deliver(decodeIDKey(req.ID), json.RawMessage(trimmed))
		}
		return http.StatusAccepted, true
	}
	if req.Method == "notifications/roots/list_changed" {
		if sess, ok := h.sessions.get(r.Header.Get(mcpSessionHeader)); ok {
			sess.roots.invalidate()
		}
		return http.StatusAccepted, true
	}
	if len(req.ID) == 0 {
		return http.StatusAccepted, true
	}
	if req.Method == "initialize" {
		sess := h.sessions.create(parseClientCapabilities(req.Params))
		w.Header().Set(mcpSessionHeader, sess.id)
	}
	if req.Method == "tools/call" {
		h.streamToolCall(w, r, req)
		return 0, true
	}
	return 0, false
}

func (h *mcpHTTPHandler) handleBatch(w http.ResponseWriter, r *http.Request, body []byte) {
	var reqs []mcpRequest
	if err := json.Unmarshal(body, &reqs); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, buildErrorMessage(nil, -32700, "parse error"))
		return
	}
	ctx := withClientCaller(r.Context(), callerForRequest(h, r))
	responses := make([]map[string]any, 0, len(reqs))
	for _, req := range reqs {
		if req.JSONRPC != "2.0" {
			responses = append(responses, buildErrorMessage(req.idPtr(), -32600, "invalid request"))
			continue
		}
		if len(req.ID) == 0 {
			continue
		}
		responses = append(responses, h.produceMessage(ctx, req))
	}
	if len(responses) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeJSONStatus(w, http.StatusOK, responses)
}

func (h *mcpHTTPHandler) produceMessage(ctx context.Context, req mcpRequest) map[string]any {
	if req.Method == "tools/call" {
		if err := acquireRequestSlot(ctx, h.sem); err != nil {
			return buildErrorMessage(req.idPtr(), -32603, "server busy")
		}
		defer releaseRequestSlot(h.sem)
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

func (h *mcpHTTPHandler) streamToolCall(w http.ResponseWriter, r *http.Request, req mcpRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONStatus(w, http.StatusOK, h.produceMessage(r.Context(), req))
		return
	}
	if err := acquireRequestSlot(r.Context(), h.sem); err != nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, buildErrorMessage(req.idPtr(), -32603, "server busy"))
		return
	}
	defer releaseRequestSlot(h.sem)

	w.Header().Set("Content-Type", contentTypeEventStream)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	ctx := withClientCaller(r.Context(), callerForRequest(h, r))
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
	if err != nil {
		_ = writeSSE(w, buildErrorMessage(req.idPtr(), -32602, err.Error()))
		flusher.Flush()
		return
	}
	_ = writeSSE(w, buildResultMessage(req.ID, result))
	flusher.Flush()
}
