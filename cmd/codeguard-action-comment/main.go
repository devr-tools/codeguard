package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/githubaction"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("codeguard-action-comment", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bodyFile := fs.String("body-file", "", "path to markdown body file")
	eventPath := fs.String("event-path", os.Getenv("GITHUB_EVENT_PATH"), "path to the GitHub event payload")
	repository := fs.String("repository", os.Getenv("GITHUB_REPOSITORY"), "repository in owner/name format")
	token := fs.String("token", os.Getenv("GITHUB_TOKEN"), "GitHub token")
	apiURL := fs.String("api-url", os.Getenv("GITHUB_API_URL"), "GitHub API base URL")
	marker := fs.String("marker", "codeguard-action-comment", "sticky comment marker")
	mode := fs.String("mode", "sticky", "comment mode: sticky or new")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*bodyFile) == "" {
		_, _ = fmt.Fprintln(stderr, "body-file is required")
		return 1
	}
	if strings.TrimSpace(*repository) == "" {
		_, _ = fmt.Fprintln(stderr, "repository is required")
		return 1
	}
	if strings.TrimSpace(*token) == "" {
		_, _ = fmt.Fprintln(stderr, "token is required")
		return 1
	}

	body, err := os.ReadFile(filepath.Clean(*bodyFile))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "read body file: %v\n", err)
		return 1
	}
	prNumber, err := githubaction.ResolvePullRequestNumber(*eventPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve pull request number: %v\n", err)
		return 1
	}

	commentBody := githubaction.WrapCommentBody(string(body), *marker)
	client := githubaction.CommentPublisher{
		BaseURL: githubaction.NormalizeAPIURL(*apiURL),
		Token:   strings.TrimSpace(*token),
		Client:  http.DefaultClient,
	}

	if err := client.Publish(*repository, prNumber, commentBody, strings.TrimSpace(*mode)); err != nil {
		_, _ = fmt.Fprintf(stderr, "publish comment: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "published CodeGuard PR comment on #%d\n", prNumber)
	return 0
}
