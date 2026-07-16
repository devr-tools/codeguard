package githubaction

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const MaxCommentBodyBytes = 65000

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
