package cli

import (
	"net/http"
	"strings"
)

type mcpHTTPHandler struct {
	tools    *mcpToolService
	auth     mcpAuthConfig
	path     string
	sem      chan struct{}
	sessions *sessionRegistry
}

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

func (h *mcpHTTPHandler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
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

func callerForRequest(h *mcpHTTPHandler, r *http.Request) clientCaller {
	sess, ok := h.sessions.get(r.Header.Get(mcpSessionHeader))
	if !ok {
		return nil
	}
	return sess.caller()
}
