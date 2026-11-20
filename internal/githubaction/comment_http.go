package githubaction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultClientTimeout = 30 * time.Second

// APIError is a GitHub API response with an unexpected status. Callers match
// StatusCode to tell a rejected payload (422) from an operational failure.
type APIError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("github api returned %s: %s", e.Status, e.Body)
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
		return &APIError{StatusCode: resp.StatusCode, Status: resp.Status, Body: strings.TrimSpace(string(body))}
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, out)
}
