package githubaction

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ReviewPublisher posts findings as line-level comments in one pull request
// review with event COMMENT. It never approves or requests changes.
type ReviewPublisher struct {
	// Comments supplies the API base URL, token, HTTP client and the author
	// identity whose earlier review comments count for deduplication.
	Comments CommentPublisher
	// Limit caps inline comments per review; findings past it are listed in
	// the review body. Zero or less means DefaultReviewCommentLimit.
	Limit int
	// CommitID pins the review to the scanned head commit. It is also what
	// lets a rejected batch fall back to posting comments one at a time.
	CommitID string
	// Suggest returns exact replacements for suggestion blocks; nil disables
	// suggestions.
	Suggest SuggestionFunc
}

// Publish creates one review for the findings that are on PR diff lines and
// not already commented by the publisher's own identity.
func (p ReviewPublisher) Publish(ctx context.Context, repository string, prNumber int, findings []Finding) (ReviewResult, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = DefaultReviewCommentLimit
	}
	diff, err := p.listDiff(ctx, repository, prNumber)
	if err != nil {
		return ReviewResult{}, fmt.Errorf("list pull request files: %w", err)
	}
	existing, err := p.existingFingerprints(ctx, repository, prNumber)
	if err != nil {
		return ReviewResult{}, fmt.Errorf("list review comments: %w", err)
	}
	plan := planReview(findings, diff, existing, limit, p.Suggest)
	result := ReviewResult{Duplicates: plan.duplicates, OutsideDiff: plan.outsideDiff, Overflow: len(plan.overflow)}
	if len(plan.comments) == 0 && len(plan.overflow) == 0 {
		return result, nil
	}
	comments := make([]ReviewComment, 0, len(plan.comments))
	for _, planned := range plan.comments {
		comments = append(comments, planned.comment)
	}
	err = p.createReview(ctx, repository, prNumber, createReviewRequest{
		CommitID: p.CommitID,
		Body:     renderReviewBody(len(comments), limit, plan.overflow, nil),
		Event:    "COMMENT",
		Comments: comments,
	})
	if err == nil {
		result.Posted, result.ReviewCreated = len(comments), true
		return result, nil
	}
	if !isUnprocessable(err) {
		return result, fmt.Errorf("create review: %w", err)
	}
	return p.publishIndividually(ctx, repository, prNumber, plan, limit, result)
}

// publishIndividually handles a review GitHub rejected as a whole (422, for
// example when one line left the diff after a new push). It posts each
// comment on its own, skips the ones GitHub still rejects, and lists those in
// a body-only review.
func (p ReviewPublisher) publishIndividually(ctx context.Context, repository string, prNumber int, plan reviewPlan, limit int, result ReviewResult) (ReviewResult, error) {
	var rejected []Finding
	for _, planned := range plan.comments {
		if strings.TrimSpace(p.CommitID) == "" {
			rejected = append(rejected, planned.finding)
			continue
		}
		err := p.createComment(ctx, repository, prNumber, planned.comment)
		switch {
		case err == nil:
			result.Posted++
		case isUnprocessable(err):
			rejected = append(rejected, planned.finding)
		default:
			return result, fmt.Errorf("create review comment: %w", err)
		}
	}
	result.Rejected = len(rejected)
	if len(rejected) == 0 && len(plan.overflow) == 0 {
		return result, nil
	}
	err := p.createReview(ctx, repository, prNumber, createReviewRequest{
		CommitID: p.CommitID,
		Body:     renderReviewBody(result.Posted, limit, plan.overflow, rejected),
		Event:    "COMMENT",
	})
	if err != nil {
		return result, fmt.Errorf("create summary review: %w", err)
	}
	result.ReviewCreated = true
	return result, nil
}

func isUnprocessable(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnprocessableEntity
}

// listDiff reads the RIGHT-side diff lines of every file in the pull request.
// A file without a patch (binary, or too large for GitHub to inline) gets no
// lines, so it never receives an inline comment.
func (p ReviewPublisher) listDiff(ctx context.Context, repository string, prNumber int) (DiffLines, error) {
	diff := DiffLines{}
	for page := 1; ; page++ {
		var files []pullRequestFile
		if err := p.getPage(ctx, fmt.Sprintf("/repos/%s/pulls/%d/files", escapeRepository(repository), prNumber), page, &files); err != nil {
			return nil, err
		}
		for _, file := range files {
			if file.Status != "removed" && file.Patch != "" {
				diff[file.Filename] = ParsePatch(file.Patch)
			}
		}
		if len(files) < 100 {
			return diff, nil
		}
	}
}

// existingFingerprints collects the fingerprints of review comments the
// publisher's own identity wrote earlier. A marker in anyone else's comment is
// ignored: trusting it would let a PR author silence a finding by posting a
// comment that claims it was already reported.
func (p ReviewPublisher) existingFingerprints(ctx context.Context, repository string, prNumber int) (map[string]bool, error) {
	author := resolveCommentAuthor(ctx, p.Comments)
	fingerprints := map[string]bool{}
	for page := 1; ; page++ {
		var comments []pullReviewComment
		if err := p.getPage(ctx, fmt.Sprintf("/repos/%s/pulls/%d/comments", escapeRepository(repository), prNumber), page, &comments); err != nil {
			return nil, err
		}
		for _, comment := range comments {
			if !strings.EqualFold(comment.User.Login, author) {
				continue
			}
			if fingerprint, ok := reviewFingerprintFromBody(comment.Body); ok {
				fingerprints[fingerprint] = true
			}
		}
		if len(comments) < 100 {
			return fingerprints, nil
		}
	}
}

func (p ReviewPublisher) getPage(ctx context.Context, path string, page int, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s%s?per_page=100&page=%d", p.Comments.BaseURL, path, page), nil)
	if err != nil {
		return err
	}
	return p.Comments.doJSON(req, http.StatusOK, out)
}

func (p ReviewPublisher) createReview(ctx context.Context, repository string, prNumber int, request createReviewRequest) error {
	return p.postJSON(ctx, fmt.Sprintf("/repos/%s/pulls/%d/reviews", escapeRepository(repository), prNumber), request, http.StatusOK)
}

func (p ReviewPublisher) createComment(ctx context.Context, repository string, prNumber int, comment ReviewComment) error {
	return p.postJSON(ctx, fmt.Sprintf("/repos/%s/pulls/%d/comments", escapeRepository(repository), prNumber), singleReviewCommentRequest{
		CommitID:  p.CommitID,
		Path:      comment.Path,
		Line:      comment.Line,
		Side:      comment.Side,
		StartLine: comment.StartLine,
		StartSide: comment.StartSide,
		Body:      comment.Body,
	}, http.StatusCreated)
}

func (p ReviewPublisher) postJSON(ctx context.Context, path string, payload any, wantStatus int) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Comments.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return p.Comments.doJSON(req, wantStatus, nil)
}
