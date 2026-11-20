package main

import (
	"bytes"
	"context"
	"errors"
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

type reviewFlags struct {
	reportFile string
	eventPath  string
	repository string
	token      string
	apiURL     string
	limit      int
}

func parseFlags(args []string, stderr io.Writer) (reviewFlags, error) {
	var flags reviewFlags
	fs := flag.NewFlagSet("codeguard-action-review", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&flags.reportFile, "report-file", "", "path to a codeguard scan -format json report")
	fs.StringVar(&flags.eventPath, "event-path", os.Getenv("GITHUB_EVENT_PATH"), "path to the GitHub event payload")
	fs.StringVar(&flags.repository, "repository", os.Getenv("GITHUB_REPOSITORY"), "repository in owner/name format")
	fs.StringVar(&flags.token, "token", os.Getenv("GITHUB_TOKEN"), "GitHub token")
	fs.StringVar(&flags.apiURL, "api-url", os.Getenv("GITHUB_API_URL"), "GitHub API base URL")
	fs.IntVar(&flags.limit, "limit", githubaction.DefaultReviewCommentLimit, "maximum inline comments per review")
	if err := fs.Parse(args); err != nil {
		return flags, err
	}
	for name, value := range map[string]string{"report-file": flags.reportFile, "repository": flags.repository, "token": flags.token} {
		if strings.TrimSpace(value) == "" {
			return flags, fmt.Errorf("%s is required", name)
		}
	}
	if flags.limit <= 0 {
		return flags, errors.New("limit must be a positive integer")
	}
	return flags, nil
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	flags, err := parseFlags(args, stderr)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	report, err := readReport(flags.reportFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "read report: %v\n", err)
		return 1
	}
	prNumber, err := githubaction.ResolvePullRequestNumber(flags.eventPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve pull request number: %v\n", err)
		return 1
	}
	headSHA, err := githubaction.ResolvePullRequestHeadSHA(flags.eventPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve pull request head: %v\n", err)
		return 1
	}
	publisher := githubaction.ReviewPublisher{
		Comments: githubaction.CommentPublisher{
			BaseURL: githubaction.NormalizeAPIURL(flags.apiURL),
			Token:   strings.TrimSpace(flags.token),
			Client:  http.DefaultClient,
		},
		Limit:    flags.limit,
		CommitID: headSHA,
		Suggest:  githubaction.DeterministicSuggestion,
	}
	result, err := publisher.Publish(context.Background(), flags.repository, prNumber, githubaction.ReportFindings(report))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "publish review: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "CodeGuard review on #%d: %d posted, %d already commented, %d outside the diff, %d over the limit, %d rejected\n",
		prNumber, result.Posted, result.Duplicates, result.OutsideDiff, result.Overflow, result.Rejected)
	return 0
}

func readReport(path string) (githubaction.Report, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return githubaction.Report{}, err
	}
	return githubaction.DecodeReport(bytes.NewReader(data))
}
