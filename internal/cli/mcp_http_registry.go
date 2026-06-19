package cli

import (
	"strings"
	"sync"
)

const maxHTTPSessions = 512

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
	defer r.mu.Unlock()
	if r.max > 0 && len(r.order) >= r.max {
		oldest := r.order[0]
		r.order = r.order[1:]
		delete(r.sessions, oldest)
	}
	r.sessions[sess.id] = sess
	r.order = append(r.order, sess.id)
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
	defer r.mu.Unlock()
	if _, ok := r.sessions[id]; !ok {
		return
	}
	delete(r.sessions, id)
	for i, existing := range r.order {
		if existing == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			return
		}
	}
}
