package githubaction

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const MaxCommentBodyBytes = 65000

// DefaultCommentAuthor is the login of the default GITHUB_TOKEN, which cannot
// call GET /user to discover itself.
const DefaultCommentAuthor = "github-actions[bot]"

type CommentPublisher struct {
	BaseURL string
	Token   string
	Client  *http.Client
	// Author is the login whose comments a sticky publish may update. When
	// empty it is resolved from the token with GET /user (a personal access
	// token), falling back to DefaultCommentAuthor (the GITHUB_TOKEN).
	Author string
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
	marker, exactMarker := stickyMarkerFromBody(body)
	author := p.commentAuthor(ctx)
	for _, comment := range comments {
		// Anyone who can comment on the PR can post a comment that starts
		// with the marker. Updating it would put the report in a comment the
		// PR author owns and can later edit into a fake result, so only the
		// publisher's own comments are candidates.
		if !strings.EqualFold(comment.User.Login, author) {
			continue
		}
		commentMarker, hasCommentMarker := stickyMarkerFromBody(comment.Body)
		matches := exactMarker && hasCommentMarker && commentMarker == marker
		if !exactMarker {
			matches = strings.Contains(comment.Body, marker)
		}
		if matches {
			return p.updateComment(ctx, repository, comment.ID, body)
		}
	}
	return p.createComment(ctx, repository, prNumber, body)
}

// commentAuthor returns the login whose comments publishSticky may update.
func (p CommentPublisher) commentAuthor(ctx context.Context) string {
	if author := strings.TrimSpace(p.Author); author != "" {
		return author
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL+"/user", nil)
	if err != nil {
		return DefaultCommentAuthor
	}
	var user commentUser
	if err := p.doJSON(req, http.StatusOK, &user); err != nil || strings.TrimSpace(user.Login) == "" {
		return DefaultCommentAuthor
	}
	return user.Login
}

// stickyMarkerFromBody returns the complete marker emitted by WrapCommentBody.
func stickyMarkerFromBody(body string) (string, bool) {
	firstLine, _, _ := strings.Cut(body, "\n")
	firstLine = strings.TrimSpace(firstLine)
	if strings.HasPrefix(firstLine, StickyMarkerPrefix) && strings.HasSuffix(firstLine, "-->") {
		return firstLine, true
	}
	return StickyMarkerPrefix, false
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
	var comments []issueComment
	for page := 1; ; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/issues/%d/comments?per_page=100&page=%d", p.BaseURL, escapeRepository(repository), prNumber, page), nil)
		if err != nil {
			return nil, err
		}
		var pageComments []issueComment
		if err := p.doJSON(req, http.StatusOK, &pageComments); err != nil {
			return nil, err
		}
		comments = append(comments, pageComments...)
		if len(pageComments) < 100 {
			return comments, nil
		}
	}
}

func (p CommentPublisher) createComment(ctx context.Context, repository string, prNumber int, body string) error {
	return p.sendCommentRequest(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/issues/%d/comments", p.BaseURL, escapeRepository(repository), prNumber), body, http.StatusCreated)
}

func (p CommentPublisher) updateComment(ctx context.Context, repository string, commentID int64, body string) error {
	return p.sendCommentRequest(ctx, http.MethodPatch, fmt.Sprintf("%s/repos/%s/issues/comments/%d", p.BaseURL, escapeRepository(repository), commentID), body, http.StatusOK)
}
