package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPStreamDisconnectPreservesSessionState(t *testing.T) {
	h := &mcpHTTPHandler{sessions: newSessionRegistry()}
	sess := h.sessions.create(map[string]any{"roots": map[string]any{}})
	wantRoots := []mcpRoot{{URI: "file:///tmp/codeguard-root", Name: "test"}}
	if _, err := sess.roots.load(func() ([]mcpRoot, error) { return wantRoots, nil }); err != nil {
		t.Fatalf("prime roots cache: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, mcpDefaultHTTPPath, nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	req.Header.Set(mcpSessionHeader, sess.id)
	done := make(chan struct{})
	go func() {
		h.handleGetStream(httptest.NewRecorder(), req)
		close(done)
	}()

	waitForAttachedHTTPStream(t, sess)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("HTTP stream did not stop after its context was canceled")
	}

	preserved, ok := h.sessions.get(sess.id)
	if !ok {
		t.Fatal("HTTP stream disconnect removed the MCP session")
	}
	gotRoots, err := preserved.roots.load(func() ([]mcpRoot, error) {
		return nil, errors.New("roots cache was unexpectedly cleared")
	})
	if err != nil {
		t.Fatalf("load cached roots after disconnect: %v", err)
	}
	if len(gotRoots) != 1 || gotRoots[0] != wantRoots[0] {
		t.Fatalf("cached roots after disconnect = %#v, want %#v", gotRoots, wantRoots)
	}
}

func waitForAttachedHTTPStream(t *testing.T, sess *httpSession) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		sess.streamMu.Lock()
		attached := sess.attached
		sess.streamMu.Unlock()
		if attached {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("HTTP stream was not attached")
}
