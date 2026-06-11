package githubaction

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const StickyMarkerPrefix = "<!-- codeguard:"

func ResolvePullRequestNumber(eventPath string) (int, error) {
	if strings.TrimSpace(eventPath) == "" {
		return 0, errors.New("event-path is required")
	}
	data, err := os.ReadFile(filepath.Clean(eventPath))
	if err != nil {
		return 0, err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return 0, err
	}
	if pr, ok := payload["pull_request"].(map[string]any); ok {
		if number, ok := pr["number"].(float64); ok && number > 0 {
			return int(number), nil
		}
	}
	if number, ok := payload["number"].(float64); ok && number > 0 {
		return int(number), nil
	}
	return 0, errors.New("event payload does not contain a pull request number")
}

func WrapCommentBody(body string, marker string) string {
	trimmedMarker := strings.TrimSpace(marker)
	if trimmedMarker == "" {
		trimmedMarker = "codeguard-action-comment"
	}
	trimmedBody := strings.TrimSpace(body)
	return fmt.Sprintf("<!-- codeguard:%s -->\n%s\n", trimmedMarker, trimmedBody)
}

func TruncateCommentBody(body string) string {
	if len(body) <= MaxCommentBodyBytes {
		return body
	}
	suffix := "\n\n_Comment truncated to fit GitHub comment size limits._\n"
	limit := MaxCommentBodyBytes - len(suffix)
	if limit < 0 {
		return suffix
	}
	return body[:limit] + suffix
}

func NormalizeAPIURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "https://api.github.com"
	}
	return strings.TrimRight(trimmed, "/")
}
