package action_test

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func TestActionDefaultsToBundledCodeGuardAndPinsSetupGo(t *testing.T) {
	action, err := os.ReadFile(filepath.Join("..", "..", "action.yml"))
	if err != nil {
		t.Fatalf("read action.yml: %v", err)
	}
	contents := string(action)
	if strings.Contains(contents, "actions/setup-go@v") {
		t.Fatal("setup-go must be pinned to an immutable commit SHA")
	}
	if !strings.Contains(contents, "actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e") {
		t.Fatal("expected the pinned setup-go v7 commit")
	}
	if strings.Contains(contents, "cmd/codeguard@latest") {
		t.Fatal("the action must not silently install the latest CodeGuard release")
	}
	if !strings.Contains(contents, "go install ./cmd/codeguard") {
		t.Fatal("expected the default install to build CodeGuard from the bundled action source")
	}
}

func TestPublishStickyUpdatesExistingComment(t *testing.T) {
	var updated bool
	publisher := githubaction.CommentPublisher{
		BaseURL: "https://api.github.test",
		Token:   "test-token",
		Author:  githubaction.DefaultCommentAuthor,
		Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/repos/devr-tools/codeguard/issues/7/comments":
				return jsonResponse(http.StatusOK, `[{"id":99,"body":"<!-- codeguard:codeguard-action-comment -->\nold","user":{"login":"github-actions[bot]"}}]`), nil
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

func TestPublishStickyUpdatesOnlyMatchingMarker(t *testing.T) {
	var updatedID string
	publisher := githubaction.CommentPublisher{
		BaseURL: "https://api.github.test",
		Token:   "test-token",
		Author:  githubaction.DefaultCommentAuthor,
		Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.Method {
			case http.MethodGet:
				return jsonResponse(http.StatusOK, `[
					{"id":98,"body":"<!-- codeguard:security -->\nmentions <!-- codeguard:design -->","user":{"login":"github-actions[bot]"}},
					{"id":99,"body":"<!-- codeguard:design -->\nold","user":{"login":"github-actions[bot]"}}
				]`), nil
			case http.MethodPatch:
				updatedID = r.URL.Path
				return jsonResponse(http.StatusOK, `{"id":99}`), nil
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				return nil, nil
			}
		})},
	}
	if err := publisher.Publish("devr-tools/codeguard", 7, githubaction.WrapCommentBody("body", "design"), "sticky"); err != nil {
		t.Fatalf("publish sticky: %v", err)
	}
	if want := "/repos/devr-tools/codeguard/issues/comments/99"; updatedID != want {
		t.Fatalf("updated comment %q, want %q", updatedID, want)
	}
}

func TestPublishStickyFindsMatchingCommentOnLaterPage(t *testing.T) {
	firstPage := make([]map[string]any, 100)
	for i := range firstPage {
		firstPage[i] = map[string]any{
			"id":   i + 1,
			"body": fmt.Sprintf("<!-- codeguard:other-%d -->\nold", i),
			"user": map[string]any{"login": githubaction.DefaultCommentAuthor},
		}
	}
	firstPageJSON, err := json.Marshal(firstPage)
	if err != nil {
		t.Fatalf("marshal first page: %v", err)
	}

	var pages []string
	var updated bool
	publisher := githubaction.CommentPublisher{
		BaseURL: "https://api.github.test",
		Token:   "test-token",
		Author:  githubaction.DefaultCommentAuthor,
		Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case r.Method == http.MethodGet:
				pages = append(pages, r.URL.Query().Get("page"))
				if r.URL.Query().Get("page") == "1" {
					return jsonResponse(http.StatusOK, string(firstPageJSON)), nil
				}
				return jsonResponse(http.StatusOK, `[{"id":101,"body":"<!-- codeguard:design -->\nold","user":{"login":"github-actions[bot]"}}]`), nil
			case r.Method == http.MethodPatch && r.URL.Path == "/repos/devr-tools/codeguard/issues/comments/101":
				updated = true
				return jsonResponse(http.StatusOK, `{"id":101}`), nil
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				return nil, nil
			}
		})},
	}
	if err := publisher.Publish("devr-tools/codeguard", 7, githubaction.WrapCommentBody("body", "design"), "sticky"); err != nil {
		t.Fatalf("publish sticky: %v", err)
	}
	if got := strings.Join(pages, ","); got != "1,2" {
		t.Fatalf("requested pages %q, want %q", got, "1,2")
	}
	if !updated {
		t.Fatal("expected matching comment on second page to be updated")
	}
}

func TestPublishStickyCreatesCommentWhenMissing(t *testing.T) {
	var created bool
	publisher := githubaction.CommentPublisher{
		BaseURL: "https://api.github.test",
		Token:   "test-token",
		Author:  githubaction.DefaultCommentAuthor,
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

// Anyone who can comment on the PR can post a comment whose first line is the
// sticky marker. The publisher must not write its report into that comment,
// which the PR author owns and could later edit into a fake result.
func TestPublishStickyIgnoresMarkerCommentsFromOtherAuthors(t *testing.T) {
	var created bool
	publisher := githubaction.CommentPublisher{
		BaseURL: "https://api.github.test",
		Token:   "test-token",
		Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/user":
				return jsonResponse(http.StatusForbidden, `{"message":"Resource not accessible by integration"}`), nil
			case r.Method == http.MethodGet && r.URL.Path == "/repos/devr-tools/codeguard/issues/7/comments":
				return jsonResponse(http.StatusOK, `[{"id":66,"body":"<!-- codeguard:codeguard-action-comment -->\nall clear","user":{"login":"mallory"}}]`), nil
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
		t.Fatal("expected a new comment instead of updating another author's marker comment")
	}
}

// With a personal access token the publisher resolves its own login through
// GET /user and updates that user's sticky comment.
func TestPublishStickyResolvesAuthorFromToken(t *testing.T) {
	var updated bool
	publisher := githubaction.CommentPublisher{
		BaseURL: "https://api.github.test",
		Token:   "test-token",
		Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/user":
				return jsonResponse(http.StatusOK, `{"login":"codeguard-bot"}`), nil
			case r.Method == http.MethodGet && r.URL.Path == "/repos/devr-tools/codeguard/issues/7/comments":
				return jsonResponse(http.StatusOK, `[
					{"id":66,"body":"<!-- codeguard:codeguard-action-comment -->\nfake","user":{"login":"github-actions[bot]"}},
					{"id":67,"body":"<!-- codeguard:codeguard-action-comment -->\nold","user":{"login":"CodeGuard-Bot"}}
				]`), nil
			case r.Method == http.MethodPatch && r.URL.Path == "/repos/devr-tools/codeguard/issues/comments/67":
				updated = true
				return jsonResponse(http.StatusOK, `{"id":67}`), nil
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
		t.Fatal("expected the token owner's sticky comment to be updated")
	}
}
