package cli

import (
	"errors"
	"net/http"
	"sync"
)

// mcp_http_session.go gives the Streamable-HTTP transport the state needed for
// server→client requests. A session is created on initialize (its id returned
// via Mcp-Session-Id); the client opens a GET SSE stream for that session over
// which the server writes sampling/roots requests, and answers them on later
// POSTs which are routed back through the session's serverRequester.

var errNoClientStream = errors.New("client SSE stream is not connected")

type httpSession struct {
	id        string
	caps      map[string]any
	requester *serverRequester
	roots     *rootsCache

	streamMu sync.Mutex
	stream   http.ResponseWriter
	flusher  http.Flusher
	attached bool
}

func (sess *httpSession) attachStream(w http.ResponseWriter, f http.Flusher) {
	sess.streamMu.Lock()
	sess.stream = w
	sess.flusher = f
	sess.attached = true
	sess.streamMu.Unlock()
}

func (sess *httpSession) detachStream() {
	sess.streamMu.Lock()
	sess.stream = nil
	sess.flusher = nil
	sess.attached = false
	sess.streamMu.Unlock()
}

// sendToClient writes a server→client message over the session's SSE stream.
func (sess *httpSession) sendToClient(payload any) error {
	sess.streamMu.Lock()
	defer sess.streamMu.Unlock()
	if !sess.attached || sess.stream == nil {
		return errNoClientStream
	}
	if err := writeSSE(sess.stream, payload); err != nil {
		return err
	}
	sess.flusher.Flush()
	return nil
}

func (sess *httpSession) caller() clientCaller {
	return &clientBridge{caps: sess.caps, requester: sess.requester, send: sess.sendToClient, roots: sess.roots}
}
