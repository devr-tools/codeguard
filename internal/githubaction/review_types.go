package githubaction

const (
	levelFail = "fail"
	levelWarn = "warn"
)

// Report is the part of a codeguard scan -format json report the review
// publisher reads. It mirrors the JSON wire format instead of importing the
// scanner's core types, so the action helper depends only on the report.
type Report struct {
	Sections []ReportSection `json:"sections"`
}

// ReportSection holds one check section's findings.
type ReportSection struct {
	Findings []Finding `json:"findings"`
}

// Finding is one report finding, limited to the fields a review comment uses.
type Finding struct {
	RuleID             string            `json:"rule_id"`
	Level              string            `json:"level"`
	Message            string            `json:"message"`
	HowToFix           string            `json:"how_to_fix,omitempty"`
	Path               string            `json:"path,omitempty"`
	Line               int               `json:"line,omitempty"`
	Fingerprint        string            `json:"fingerprint"`
	ContextFingerprint string            `json:"context_fingerprint,omitempty"`
	Suppressed         bool              `json:"suppressed,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

// ReviewComment is one line-level comment in a pull request review. Line and
// StartLine are new-file line numbers on the RIGHT side of the diff.
type ReviewComment struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Side      string `json:"side"`
	StartLine int    `json:"start_line,omitempty"`
	StartSide string `json:"start_side,omitempty"`
	Body      string `json:"body"`
}

// ReviewResult counts what a ReviewPublisher run did with its findings.
type ReviewResult struct {
	// Posted is the number of inline comments GitHub accepted.
	Posted int
	// Duplicates were already commented by the publisher on an earlier run.
	Duplicates int
	// OutsideDiff were skipped because their line is not in the PR diff.
	OutsideDiff int
	// Overflow exceeded the comment limit and went to the review body.
	Overflow int
	// Rejected were refused by GitHub (422) and went to the review body.
	Rejected int
	// ReviewCreated reports whether a review was submitted at all.
	ReviewCreated bool
}

type pullRequestFile struct {
	Filename string `json:"filename"`
	Status   string `json:"status"`
	Patch    string `json:"patch"`
}

type pullReviewComment struct {
	ID   int64       `json:"id"`
	Body string      `json:"body"`
	User commentUser `json:"user"`
}

type createReviewRequest struct {
	CommitID string          `json:"commit_id,omitempty"`
	Body     string          `json:"body,omitempty"`
	Event    string          `json:"event"`
	Comments []ReviewComment `json:"comments,omitempty"`
}

type singleReviewCommentRequest struct {
	CommitID  string `json:"commit_id"`
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Side      string `json:"side"`
	StartLine int    `json:"start_line,omitempty"`
	StartSide string `json:"start_side,omitempty"`
	Body      string `json:"body"`
}
