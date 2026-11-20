package action_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/githubaction"
)

const (
	reviewRepo     = "devr-tools/codeguard"
	reviewHeadSHA  = "0123456789abcdef0123456789abcdef01234567"
	reviewsPath    = "/repos/devr-tools/codeguard/pulls/7/reviews"
	reviewFiles    = "/repos/devr-tools/codeguard/pulls/7/files"
	reviewComments = "/repos/devr-tools/codeguard/pulls/7/comments"
)

// appPatch adds lines 2-4 of src/app.ts; lines 1 and 5 are context.
const appPatch = "@@ -1,2 +1,5 @@\n import x from \"x\";\n+debugger;\n+const a = 1;\n+const b = 2;\n export {};"

type capturedReview struct {
	CommitID string                       `json:"commit_id"`
	Body     string                       `json:"body"`
	Event    string                       `json:"event"`
	Comments []githubaction.ReviewComment `json:"comments"`
}

// fakeGitHub serves the PR files and review-comment listings and records
// every review POST. reviewStatus and commentStatus pick the reply status of
// successive POSTs; the last entry repeats.
type fakeGitHub struct {
	t             *testing.T
	files         string
	existing      string
	reviewStatus  []int
	commentStatus []int
	reviews       []capturedReview
	single        []githubaction.ReviewComment
}

func (f *fakeGitHub) publisher(limit int) githubaction.ReviewPublisher {
	return githubaction.ReviewPublisher{
		Comments: githubaction.CommentPublisher{
			BaseURL: "https://api.github.test",
			Token:   strings.Repeat("t", 8),
			Author:  githubaction.DefaultCommentAuthor,
			Client:  &http.Client{Transport: roundTripFunc(f.serve)},
		},
		Limit:    limit,
		CommitID: reviewHeadSHA,
		Suggest:  githubaction.DeterministicSuggestion,
	}
}

func nextStatus(statuses *[]int, fallback int) int {
	if len(*statuses) == 0 {
		return fallback
	}
	status := (*statuses)[0]
	if len(*statuses) > 1 {
		*statuses = (*statuses)[1:]
	}
	return status
}

func (f *fakeGitHub) serve(r *http.Request) (*http.Response, error) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == reviewFiles:
		return jsonResponse(http.StatusOK, f.files), nil
	case r.Method == http.MethodGet && r.URL.Path == reviewComments:
		if f.existing == "" {
			return jsonResponse(http.StatusOK, `[]`), nil
		}
		return jsonResponse(http.StatusOK, f.existing), nil
	case r.Method == http.MethodPost && r.URL.Path == reviewsPath:
		var review capturedReview
		decodeBody(f.t, r, &review)
		f.reviews = append(f.reviews, review)
		return jsonResponse(nextStatus(&f.reviewStatus, http.StatusOK), `{"id":1}`), nil
	case r.Method == http.MethodPost && r.URL.Path == reviewComments:
		var comment githubaction.ReviewComment
		decodeBody(f.t, r, &comment)
		f.single = append(f.single, comment)
		return jsonResponse(nextStatus(&f.commentStatus, http.StatusCreated), `{"id":2}`), nil
	default:
		f.t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		return nil, nil
	}
}

func decodeBody(t *testing.T, r *http.Request, out any) {
	t.Helper()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode request body %s: %v", data, err)
	}
}

func filesJSON(t *testing.T, patches map[string]string) string {
	t.Helper()
	files := make([]map[string]string, 0, len(patches))
	for name, patch := range patches {
		files = append(files, map[string]string{"filename": name, "status": "modified", "patch": patch})
	}
	data, err := json.Marshal(files)
	if err != nil {
		t.Fatalf("marshal files: %v", err)
	}
	return string(data)
}

func finding(rule string, path string, line int, fingerprint string) githubaction.Finding {
	return githubaction.Finding{
		RuleID:             rule,
		Level:              "warn",
		Message:            "message for " + fingerprint,
		Path:               path,
		Line:               line,
		Fingerprint:        "fp-" + fingerprint,
		ContextFingerprint: "ctx-" + fingerprint,
	}
}

func existingComment(t *testing.T, login string, fingerprints ...string) string {
	t.Helper()
	comments := make([]map[string]any, 0, len(fingerprints))
	for i, fingerprint := range fingerprints {
		comments = append(comments, map[string]any{
			"id":   i + 1,
			"body": githubaction.ReviewMarkerPrefix + fingerprint + " -->\nold",
			"user": map[string]any{"login": login},
		})
	}
	data, err := json.Marshal(comments)
	if err != nil {
		t.Fatalf("marshal comments: %v", err)
	}
	return string(data)
}

func TestReviewPostsRightSideCommentsOnDiffLinesOnly(t *testing.T) {
	gh := &fakeGitHub{t: t, files: filesJSON(t, map[string]string{"src/app.ts": appPatch})}
	findings := []githubaction.Finding{
		finding("quality.typescript.debugger-statement", "src/app.ts", 2, "a"),
		finding("quality.long-function", "src/app.ts", 40, "b"),
		finding("quality.long-function", "src/other.ts", 3, "c"),
	}
	result, err := gh.publisher(25).Publish(context.Background(), reviewRepo, 7, findings)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if result.Posted != 1 || result.OutsideDiff != 2 || !result.ReviewCreated {
		t.Fatalf("result = %+v, want 1 posted and 2 outside the diff", result)
	}
	if len(gh.reviews) != 1 {
		t.Fatalf("created %d reviews, want 1", len(gh.reviews))
	}
	review := gh.reviews[0]
	if review.Event != "COMMENT" || review.CommitID != reviewHeadSHA {
		t.Fatalf("review event %q commit %q, want COMMENT on the head commit", review.Event, review.CommitID)
	}
	if len(review.Comments) != 1 {
		t.Fatalf("review has %d comments, want 1: %+v", len(review.Comments), review.Comments)
	}
	comment := review.Comments[0]
	if comment.Path != "src/app.ts" || comment.Line != 2 || comment.Side != "RIGHT" || comment.StartLine != 0 {
		t.Fatalf("comment anchored at %s:%d side %q start %d, want src/app.ts:2 RIGHT", comment.Path, comment.Line, comment.Side, comment.StartLine)
	}
	if !strings.HasPrefix(comment.Body, githubaction.ReviewMarkerPrefix+"ctx-a -->\n") {
		t.Fatalf("comment body lacks the fingerprint marker:\n%s", comment.Body)
	}
	if !strings.Contains(comment.Body, "```suggestion\n```\n") {
		t.Fatalf("debugger-only line should carry an empty (delete-line) suggestion:\n%s", comment.Body)
	}
}

func TestReviewSkipsFindingsAlreadyCommentedByPublisher(t *testing.T) {
	gh := &fakeGitHub{
		t:        t,
		files:    filesJSON(t, map[string]string{"src/app.ts": appPatch}),
		existing: existingComment(t, "GitHub-Actions[bot]", "ctx-a", "ctx-b"),
	}
	findings := []githubaction.Finding{
		finding("quality.long-function", "src/app.ts", 2, "a"),
		finding("quality.long-function", "src/app.ts", 3, "b"),
	}
	result, err := gh.publisher(25).Publish(context.Background(), reviewRepo, 7, findings)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if result.Duplicates != 2 || result.ReviewCreated || len(gh.reviews) != 0 {
		t.Fatalf("result = %+v with %d reviews; want both findings deduplicated and no review", result, len(gh.reviews))
	}
}

// A marker comment written by anyone but the publisher must not suppress a
// finding, or a PR author could hide findings by pre-posting fake markers.
func TestReviewIgnoresMarkersFromOtherAuthors(t *testing.T) {
	gh := &fakeGitHub{
		t:        t,
		files:    filesJSON(t, map[string]string{"src/app.ts": appPatch}),
		existing: existingComment(t, "mallory", "ctx-a"),
	}
	result, err := gh.publisher(25).Publish(context.Background(), reviewRepo, 7, []githubaction.Finding{finding("quality.long-function", "src/app.ts", 3, "a")})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if result.Duplicates != 0 || result.Posted != 1 {
		t.Fatalf("result = %+v; another user's marker must not deduplicate the finding", result)
	}
}

func TestReviewCapsCommentsAndListsOverflowInBody(t *testing.T) {
	gh := &fakeGitHub{t: t, files: filesJSON(t, map[string]string{"src/app.ts": appPatch})}
	findings := []githubaction.Finding{
		finding("quality.long-function", "src/app.ts", 1, "a"),
		finding("quality.long-function", "src/app.ts", 2, "b"),
		finding("quality.long-function", "src/app.ts", 3, "c"),
		finding("quality.long-function", "src/app.ts", 4, "d"),
	}
	result, err := gh.publisher(2).Publish(context.Background(), reviewRepo, 7, findings)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if result.Posted != 2 || result.Overflow != 2 {
		t.Fatalf("result = %+v, want 2 posted and 2 overflow", result)
	}
	review := gh.reviews[0]
	if len(review.Comments) != 2 {
		t.Fatalf("review has %d comments, want 2", len(review.Comments))
	}
	if !strings.HasPrefix(review.Body, githubaction.ReviewBodyMarker) || !strings.Contains(review.Body, "2 more finding(s)") {
		t.Fatalf("review body does not report the overflow:\n%s", review.Body)
	}
	for _, line := range []string{"src/app.ts:3", "src/app.ts:4"} {
		if !strings.Contains(review.Body, line) {
			t.Fatalf("review body does not list overflow finding %s:\n%s", line, review.Body)
		}
	}
}

// GitHub rejects the whole review with 422 when any comment is invalid. The
// publisher then posts comments one by one, skips the ones still rejected and
// lists them in a body-only review.
func TestReviewFallsBackToSingleCommentsOn422(t *testing.T) {
	gh := &fakeGitHub{
		t:             t,
		files:         filesJSON(t, map[string]string{"src/app.ts": appPatch}),
		reviewStatus:  []int{http.StatusUnprocessableEntity, http.StatusOK},
		commentStatus: []int{http.StatusCreated, http.StatusUnprocessableEntity},
	}
	findings := []githubaction.Finding{
		finding("quality.long-function", "src/app.ts", 2, "a"),
		finding("quality.long-function", "src/app.ts", 3, "b"),
	}
	result, err := gh.publisher(25).Publish(context.Background(), reviewRepo, 7, findings)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if result.Posted != 1 || result.Rejected != 1 || !result.ReviewCreated {
		t.Fatalf("result = %+v, want 1 posted and 1 rejected", result)
	}
	if len(gh.single) != 2 || len(gh.reviews) != 2 {
		t.Fatalf("got %d single comments and %d reviews, want 2 and 2", len(gh.single), len(gh.reviews))
	}
	if gh.single[0].Line != 2 || gh.single[0].Side != "RIGHT" {
		t.Fatalf("single comment anchored at line %d side %q", gh.single[0].Line, gh.single[0].Side)
	}
	summary := gh.reviews[1]
	if len(summary.Comments) != 0 || summary.Event != "COMMENT" || !strings.Contains(summary.Body, "GitHub rejected 1 inline comment(s)") || !strings.Contains(summary.Body, "src/app.ts:3") {
		t.Fatalf("summary review does not list the rejected finding: %+v", summary)
	}
}

func TestReviewReturnsNonValidationErrors(t *testing.T) {
	gh := &fakeGitHub{
		t:            t,
		files:        filesJSON(t, map[string]string{"src/app.ts": appPatch}),
		reviewStatus: []int{http.StatusForbidden},
	}
	_, err := gh.publisher(25).Publish(context.Background(), reviewRepo, 7, []githubaction.Finding{finding("quality.long-function", "src/app.ts", 2, "a")})
	if err == nil || !strings.Contains(err.Error(), "Forbidden") {
		t.Fatalf("publish error = %v, want the Forbidden response surfaced", err)
	}
	if len(gh.single) != 0 {
		t.Fatal("a 403 must not trigger the per-comment fallback")
	}
}

func TestReviewMultiLineSuggestionFromTestDouble(t *testing.T) {
	gh := &fakeGitHub{t: t, files: filesJSON(t, map[string]string{"src/app.ts": appPatch})}
	publisher := gh.publisher(25)
	publisher.Suggest = func(f githubaction.Finding, diff githubaction.DiffLines) (githubaction.Suggestion, bool) {
		return githubaction.Suggestion{StartLine: 3, EndLine: 4, Lines: []string{"const ab = [1, 2];"}}, true
	}
	findings := []githubaction.Finding{
		finding("quality.duplicate-code", "src/app.ts", 3, "a"),
		finding("security.hardcoded-secret", "src/app.ts", 4, "b"),
	}
	if _, err := publisher.Publish(context.Background(), reviewRepo, 7, findings); err != nil {
		t.Fatalf("publish: %v", err)
	}
	comments := gh.reviews[0].Comments
	if len(comments) != 2 {
		t.Fatalf("got %d comments, want 2", len(comments))
	}
	multi := comments[0]
	if multi.StartLine != 3 || multi.Line != 4 || multi.StartSide != "RIGHT" {
		t.Fatalf("multi-line suggestion anchored %d-%d start side %q, want 3-4 RIGHT", multi.StartLine, multi.Line, multi.StartSide)
	}
	if !strings.Contains(multi.Body, "```suggestion\nconst ab = [1, 2];\n```\n") {
		t.Fatalf("missing suggestion block:\n%s", multi.Body)
	}
	if secret := comments[1]; strings.Contains(secret.Body, "suggestion") || secret.StartLine != 0 {
		t.Fatalf("security findings must never carry a suggestion block:\n%s", secret.Body)
	}
}

// A suggestion that reaches outside the diff hunk would be rejected by GitHub,
// so the comment is posted without one.
func TestReviewDropsSuggestionOutsideDiff(t *testing.T) {
	gh := &fakeGitHub{t: t, files: filesJSON(t, map[string]string{"src/app.ts": appPatch})}
	publisher := gh.publisher(25)
	publisher.Suggest = func(f githubaction.Finding, diff githubaction.DiffLines) (githubaction.Suggestion, bool) {
		return githubaction.Suggestion{StartLine: 4, EndLine: 9, Lines: []string{"x"}}, true
	}
	if _, err := publisher.Publish(context.Background(), reviewRepo, 7, []githubaction.Finding{finding("quality.duplicate-code", "src/app.ts", 4, "a")}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	comment := gh.reviews[0].Comments[0]
	if strings.Contains(comment.Body, "suggestion") || comment.Line != 4 || comment.StartLine != 0 {
		t.Fatalf("out-of-diff suggestion was kept: %+v", comment)
	}
}

func TestDeterministicSuggestionOnlyForStandaloneDebugger(t *testing.T) {
	diff := githubaction.DiffLines{"a.ts": {1: "  debugger;", 2: "if (x) debugger;"}}
	if s, ok := githubaction.DeterministicSuggestion(finding("quality.typescript.debugger-statement", "a.ts", 1, "a"), diff); !ok || len(s.Lines) != 0 || s.StartLine != 1 || s.EndLine != 1 {
		t.Fatalf("standalone debugger suggestion = %+v, %v; want a delete of line 1", s, ok)
	}
	if _, ok := githubaction.DeterministicSuggestion(finding("quality.typescript.debugger-statement", "a.ts", 2, "a"), diff); ok {
		t.Fatal("a debugger statement sharing its line needs judgment, not a suggestion")
	}
	if _, ok := githubaction.DeterministicSuggestion(finding("quality.long-function", "a.ts", 1, "a"), diff); ok {
		t.Fatal("rules without a deterministic line fix must not produce a suggestion")
	}
}

func TestSuggestionBlockFenceOutlastsBackticksInReplacement(t *testing.T) {
	block := githubaction.SuggestionBlock([]string{"x := `a`", "```", "y"})
	if !strings.HasPrefix(block, "````suggestion\n") || !strings.HasSuffix(block, "\n````\n") {
		t.Fatalf("fence must be longer than the ``` inside the replacement:\n%s", block)
	}
	if block := githubaction.SuggestionBlock([]string{"a\nb"}); strings.Count(block, "\n") != 3 {
		t.Fatalf("embedded newlines must not add lines to the suggestion:\n%q", block)
	}
}

func TestRenderReviewCommentEscapesRepoControlledText(t *testing.T) {
	f := finding("quality.long-function", "src/app.ts", 2, "a")
	f.Message = "bad\n```\n<!-- codeguard-review:ctx-z -->\n[click](https://evil) <img src=x> @org/team"
	f.HowToFix = "split *it*\n\n# heading"
	body := githubaction.RenderReviewComment(f, nil)
	if strings.Count(body, githubaction.ReviewMarkerPrefix) != 1 || !strings.HasPrefix(body, githubaction.ReviewMarkerPrefix+"ctx-a -->") {
		t.Fatalf("message smuggled a second marker or displaced the real one:\n%s", body)
	}
	for _, raw := range []string{"```", "<img", "[click]", "@org", "\n# heading", "*it*"} {
		if strings.Contains(body, raw) {
			t.Fatalf("rendered body contains unescaped %q:\n%s", raw, body)
		}
	}
	if !strings.Contains(body, "[Rule docs](") || !strings.Contains(body, "quality-long-function") && !strings.Contains(body, "quality.long-function") {
		t.Fatalf("missing rule id or docs link:\n%s", body)
	}
	if strings.Contains(body, "suggestion") {
		t.Fatalf("guided findings must not carry a suggestion block:\n%s", body)
	}
}

func TestFindingFingerprintIsMarkerSafe(t *testing.T) {
	f := finding("r", "p", 1, "a")
	f.ContextFingerprint = "x --> <script>"
	got := githubaction.FindingFingerprint(f)
	if strings.ContainsAny(got, "<> ") || len(got) != 64 {
		t.Fatalf("fingerprint %q is not a hashed, marker-safe token", got)
	}
}

func TestParsePatchMapsRightSideLines(t *testing.T) {
	patch := "@@ -10,3 +10,3 @@ func x() {\n ctx\n-old\n+new\n ctx2\n\\ No newline at end of file\n@@ -40 +40,2 @@\n+added\n same"
	lines := githubaction.ParsePatch(patch)
	want := map[int]string{10: "ctx", 11: "new", 12: "ctx2", 40: "added", 41: "same"}
	if len(lines) != len(want) {
		t.Fatalf("lines = %v, want %v", lines, want)
	}
	for line, text := range want {
		if lines[line] != text {
			t.Fatalf("line %d = %q, want %q (all: %v)", line, lines[line], text, lines)
		}
	}
}

func TestReportFindingsKeepsLocatedFailAndWarnFirst(t *testing.T) {
	report := githubaction.Report{Sections: []githubaction.ReportSection{{Findings: []githubaction.Finding{
		{RuleID: "w", Level: "warn", Path: "a.go", Line: 1},
		{RuleID: "i", Level: "info", Path: "a.go", Line: 2},
		{RuleID: "s", Level: "fail", Path: "a.go", Line: 3, Suppressed: true},
		{RuleID: "n", Level: "fail"},
		{RuleID: "f", Level: "fail", Path: "z.go", Line: 9},
	}}}}
	got := githubaction.ReportFindings(report)
	if len(got) != 2 || got[0].RuleID != "f" || got[1].RuleID != "w" {
		t.Fatalf("ReportFindings = %+v, want [f w]", got)
	}
}

func TestResolvePullRequestHeadSHA(t *testing.T) {
	dir := t.TempDir()
	for name, tc := range map[string]struct{ payload, want string }{
		"valid":     {fmt.Sprintf(`{"pull_request":{"number":7,"head":{"sha":%q}}}`, reviewHeadSHA), reviewHeadSHA},
		"malformed": {`{"pull_request":{"head":{"sha":"main; rm -rf /"}}}`, ""},
	} {
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, []byte(tc.payload), 0o600); err != nil {
			t.Fatalf("write event: %v", err)
		}
		got, err := githubaction.ResolvePullRequestHeadSHA(path)
		if err != nil || got != tc.want {
			t.Fatalf("%s: head sha = %q, %v; want %q", name, got, err, tc.want)
		}
	}
}

// The helper decodes the scanner's JSON report through its own wire types;
// a renamed core JSON field would silently drop findings, so pin the mapping.
func TestDecodeReportReadsScannerJSON(t *testing.T) {
	data, err := json.Marshal(core.Report{Sections: []core.SectionResult{{Findings: []core.Finding{{
		RuleID: "quality.x", Level: "warn", Message: "m", HowToFix: "h", Path: "a.go", Line: 3,
		Fingerprint: "fp", ContextFingerprint: "ctx", Metadata: map[string]string{"secret_type": "k"},
	}}}}})
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	report, err := githubaction.DecodeReport(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("decode report: %v", err)
	}
	got := report.Sections[0].Findings[0]
	want := githubaction.Finding{RuleID: "quality.x", Level: "warn", Message: "m", HowToFix: "h", Path: "a.go", Line: 3,
		Fingerprint: "fp", ContextFingerprint: "ctx", Metadata: map[string]string{"secret_type": "k"}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("decoded finding = %+v, want %+v", got, want)
	}
}
