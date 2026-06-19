package cli

import (
	"errors"
	"net/http"
	"strings"
	"sync"
)

// mcp_http_session.go gives the Streamable-HTTP transport the state needed for
// server→client requests. A session is created on initialize (its id returned
// via Mcp-Session-Id); the client opens a GET SSE stream for that session over
// which the server writes sampling/roots requests, and answers them on later
// POSTs which are routed back through the session's serverRequester.

var errNoClientStream = errors.New("client SSE stream is not connected")

// maxHTTPSessions bounds the session map so streamless clients that never send
// DELETE cannot grow it without limit; the oldest session is evicted on create.
const maxHTTPSessions = 512

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

type sessionRegistry struct {
	mu       sync.Mutex
	sessions map[string]*httpSession
	order    []string
	max      int
}

func newSessionRegistry() *sessionRegistry {
	return &sessionRegistry{sessions: map[string]*httpSession{}, max: maxHTTPSessions}
}

func (r *sessionRegistry) create(caps map[string]any) *httpSession {
	sess := &httpSession{id: newSessionID(), caps: caps, requester: newServerRequester(), roots: &rootsCache{}}
	r.mu.Lock()
	if r.max > 0 && len(r.order) >= r.max {
		oldest := r.order[0]
		r.order = r.order[1:]
		delete(r.sessions, oldest)
	}
	r.sessions[sess.id] = sess
	r.order = append(r.order, sess.id)
	r.mu.Unlock()
	return sess
}

func (r *sessionRegistry) get(id string) (*httpSession, bool) {
	if strings.TrimSpace(id) == "" {
		return nil, false
	}
	r.mu.Lock()
	sess, ok := r.sessions[id]
	r.mu.Unlock()
	return sess, ok
}

func (r *sessionRegistry) remove(id string) {
	if strings.TrimSpace(id) == "" {
		return
	}
	r.mu.Lock()
	if _, ok := r.sessions[id]; ok {
		delete(r.sessions, id)
		for i, existing := range r.order {
			if existing == id {
				r.order = append(r.order[:i], r.order[i+1:]...)
				break
			}
		}
	}
	r.mu.Unlock()
}
