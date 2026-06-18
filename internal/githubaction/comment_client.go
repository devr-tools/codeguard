package githubaction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	switch strings.TrimSpace(mode) {
	case "", "sticky":
		return p.publishSticky(repository, prNumber, body)
	case "new":
		return p.createComment(repository, prNumber, body)
	default:
		return fmt.Errorf("unsupported mode %q", mode)
	}
}

func (p CommentPublisher) publishSticky(repository string, prNumber int, body string) error {
	comments, err := p.listComments(repository, prNumber)
	if err != nil {
		return err
	}
	for _, comment := range comments {
		if strings.Contains(comment.Body, StickyMarkerPrefix) {
			return p.updateComment(repository, comment.ID, body)
		}
	}
	return p.createComment(repository, prNumber, body)
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

func (p CommentPublisher) listComments(repository string, prNumber int) ([]issueComment, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/repos/%s/issues/%d/comments?per_page=100", p.BaseURL, escapeRepository(repository), prNumber), nil)
	if err != nil {
		return nil, err
	}
	var comments []issueComment
	if err := p.doJSON(req, http.StatusOK, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

func (p CommentPublisher) createComment(repository string, prNumber int, body string) error {
	return p.sendCommentRequest(http.MethodPost, fmt.Sprintf("%s/repos/%s/issues/%d/comments", p.BaseURL, escapeRepository(repository), prNumber), body, http.StatusCreated)
}

func (p CommentPublisher) updateComment(repository string, commentID int64, body string) error {
	return p.sendCommentRequest(http.MethodPatch, fmt.Sprintf("%s/repos/%s/issues/comments/%d", p.BaseURL, escapeRepository(repository), commentID), body, http.StatusOK)
}

func (p CommentPublisher) sendCommentRequest(method string, url string, body string, wantStatus int) error {
	payload, err := json.Marshal(issueCommentRequest{Body: TruncateCommentBody(body)})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return p.doJSON(req, wantStatus, nil)
}

func (p CommentPublisher) doJSON(req *http.Request, wantStatus int, out any) error {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set("User-Agent", "codeguard-action")

	resp, err := p.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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
