package history

import "github.com/devr-tools/codeguard/internal/codeguard/checks/security"

// Options configures a history scan.
type Options struct {
	RepoPath   string
	MaxCommits int  // 0 scans all reachable commits
	AllRefs    bool // scan every ref rather than just HEAD
	Scanner    security.Scanner
}

// Finding is a single secret detected at a path/line in a specific commit.
type Finding struct {
	RuleID     string `json:"rule_id"`
	Level      string `json:"level"`
	Confidence string `json:"confidence,omitempty"`
	Message    string `json:"message"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
	Commit     string `json:"commit"`
}

// Report is the result of a history scan.
type Report struct {
	Findings       []Finding `json:"findings"`
	CommitsScanned int       `json:"commits_scanned"`
}
