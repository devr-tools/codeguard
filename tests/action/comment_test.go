package action_test

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/githubaction"
)

func TestResolvePullRequestNumber(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "event.json")
	if err := os.WriteFile(path, []byte(`{"pull_request":{"number":42}}`), 0o600); err != nil {
		t.Fatalf("write event: %v", err)
	}

	number, err := githubaction.ResolvePullRequestNumber(path)
	if err != nil {
		t.Fatalf("resolve pull request number: %v", err)
	}
	if number != 42 {
		t.Fatalf("expected pull request number 42, got %d", number)
	}
}

func TestWrapCommentBodyAddsMarker(t *testing.T) {
	body := githubaction.WrapCommentBody("## CodeGuard\n\ncontent", "sticky")
	if !strings.Contains(body, "<!-- codeguard:sticky -->") {
		t.Fatalf("expected sticky marker, got %q", body)
	}
	if !strings.Contains(body, "## CodeGuard") {
		t.Fatalf("expected body content, got %q", body)
	}
}

func TestPublishStickyUpdatesExistingComment(t *testing.T) {
	var updated bool
	publisher := githubaction.CommentPublisher{
		BaseURL: "https://api.github.test",
		Token:   "test-token",
		Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/repos/devr-tools/codeguard/issues/7/comments":
				return jsonResponse(http.StatusOK, `[{"id":99,"body":"<!-- codeguard:codeguard-action-comment -->\nold"}]`), nil
			case r.Method == http.MethodPatch && r.URL.Path == "/repos/devr-tools/codeguard/issues/comments/99":
				updated = true
				return jsonResponse(http.StatusOK, `{"id":99}`), nil
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				return nil, nil
			}
		})},
	}
	if err := publisher.Publish("devr-tools/codeguard", 7, githubaction.WrapCommentBody("body", "codeguard-action-comment"), "sticky"); err != nil {
		t.Fatalf("publish sticky: %v", err)
	}
	if !updated {
		t.Fatal("expected existing comment to be updated")
	}
}

func TestPublishStickyCreatesCommentWhenMissing(t *testing.T) {
	var created bool
	publisher := githubaction.CommentPublisher{
		BaseURL: "https://api.github.test",
		Token:   "test-token",
		Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/repos/devr-tools/codeguard/issues/7/comments":
				return jsonResponse(http.StatusOK, `[]`), nil
			case r.Method == http.MethodPost && r.URL.Path == "/repos/devr-tools/codeguard/issues/7/comments":
				created = true
				return jsonResponse(http.StatusCreated, `{"id":100}`), nil
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				return nil, nil
			}
		})},
	}
	if err := publisher.Publish("devr-tools/codeguard", 7, githubaction.WrapCommentBody("body", "codeguard-action-comment"), "sticky"); err != nil {
		t.Fatalf("publish sticky: %v", err)
	}
	if !created {
		t.Fatal("expected new comment to be created")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}
