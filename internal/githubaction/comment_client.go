package githubaction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const MaxCommentBodyBytes = 65000

// defaultClientTimeout bounds GitHub API requests when the caller-supplied
// client has no timeout of its own.
const defaultClientTimeout = 30 * time.Second

type CommentPublisher struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func (p CommentPublisher) Publish(repository string, prNumber int, body string, mode string) error {
	// TODO(harden): thread caller ctx through Publish once the cmd entrypoint
	// is updated to supply one.
	ctx := context.Background()
	switch strings.TrimSpace(mode) {
	case "", "sticky":
		return p.publishSticky(ctx, repository, prNumber, body)
	case "new":
		return p.createComment(ctx, repository, prNumber, body)
	default:
		return fmt.Errorf("unsupported mode %q", mode)
	}
}

func (p CommentPublisher) publishSticky(ctx context.Context, repository string, prNumber int, body string) error {
	comments, err := p.listComments(ctx, repository, prNumber)
	if err != nil {
		return err
	}
	for _, comment := range comments {
		if strings.Contains(comment.Body, StickyMarkerPrefix) {
			return p.updateComment(ctx, repository, comment.ID, body)
		}
	}
	return p.createComment(ctx, repository, prNumber, body)
}

// escapeRepository percent-encodes each segment of an "owner/repo" identifier
// so it cannot break out of or inject into the request path.
func escapeRepository(repository string) string {
	parts := strings.Split(repository, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func (p CommentPublisher) listComments(ctx context.Context, repository string, prNumber int) ([]issueComment, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/issues/%d/comments?per_page=100", p.BaseURL, escapeRepository(repository), prNumber), nil)
	if err != nil {
		return nil, err
	}
	var comments []issueComment
	if err := p.doJSON(req, http.StatusOK, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

func (p CommentPublisher) createComment(ctx context.Context, repository string, prNumber int, body string) error {
	return p.sendCommentRequest(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/issues/%d/comments", p.BaseURL, escapeRepository(repository), prNumber), body, http.StatusCreated)
}

func (p CommentPublisher) updateComment(ctx context.Context, repository string, commentID int64, body string) error {
	return p.sendCommentRequest(ctx, http.MethodPatch, fmt.Sprintf("%s/repos/%s/issues/comments/%d", p.BaseURL, escapeRepository(repository), commentID), body, http.StatusOK)
}

func (p CommentPublisher) sendCommentRequest(ctx context.Context, method string, url string, body string, wantStatus int) error {
	payload, err := json.Marshal(issueCommentRequest{Body: TruncateCommentBody(body)})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return p.doJSON(req, wantStatus, nil)
}

// httpClient returns the caller-supplied client, falling back to one with a
// sane timeout so a hung GitHub endpoint cannot block indefinitely.
func (p CommentPublisher) httpClient() *http.Client {
	if p.Client == nil {
		return &http.Client{Timeout: defaultClientTimeout}
	}
	if p.Client.Timeout <= 0 {
		clone := *p.Client
		clone.Timeout = defaultClientTimeout
		return &clone
	}
	return p.Client
}

func (p CommentPublisher) doJSON(req *http.Request, wantStatus int, out any) (err error) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set("User-Agent", "codeguard-action")

	// The request host is the constant GitHub API base (api.github.com or the
	// GHES equivalent passed via BaseURL), not attacker-controlled, so the URL
	// taint flagged by gosec G704 is not exploitable here.
	resp, err := p.httpClient().Do(req) //nolint:gosec // host is the constant GitHub API base
	if err != nil {
		return err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != wantStatus {
		return fmt.Errorf("github api returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, out)
}
