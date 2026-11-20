package githubaction

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxReportBytes = 64 << 20

var commitSHAPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// ResolvePullRequestHeadSHA reads pull_request.head.sha from a GitHub event
// payload. It returns "" without error when the payload has no well-formed
// head commit.
func ResolvePullRequestHeadSHA(eventPath string) (string, error) {
	if strings.TrimSpace(eventPath) == "" {
		return "", errors.New("event-path is required")
	}
	data, err := os.ReadFile(filepath.Clean(eventPath))
	if err != nil {
		return "", err
	}
	var payload struct {
		PullRequest struct {
			Head struct {
				SHA string `json:"sha"`
			} `json:"head"`
		} `json:"pull_request"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	sha := strings.ToLower(strings.TrimSpace(payload.PullRequest.Head.SHA))
	if !commitSHAPattern.MatchString(sha) {
		return "", nil
	}
	return sha, nil
}

// DecodeReport parses a codeguard scan -format json report.
func DecodeReport(r io.Reader) (Report, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxReportBytes+1))
	if err != nil {
		return Report{}, err
	}
	if len(data) > maxReportBytes {
		return Report{}, fmt.Errorf("report exceeds %d bytes", maxReportBytes)
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return Report{}, fmt.Errorf("decode report: %w", err)
	}
	return report, nil
}
