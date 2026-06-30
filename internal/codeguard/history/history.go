// Package history scans a repository's git history for hardcoded secrets that
// were committed in the past. Working-tree and diff scans only see the current
// state, so a secret removed from HEAD but still present in history (and thus
// still leaked) is invisible to them. This pass walks added lines across commits
// and reports them so the credential can be rotated, not just deleted.
package history

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/security"
)

const commitMarker = "@@CG-COMMIT@@ "

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)`)

// Options configures a history scan.
type Options struct {
	RepoPath   string
	MaxCommits int  // 0 scans all reachable commits
	AllRefs    bool // scan every ref rather than just HEAD
	Scanner    security.Scanner
}

// Finding is a single secret detected at a path/line in a specific commit.
type Finding struct {
	RuleID  string `json:"rule_id"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Commit  string `json:"commit"`
}

// Report is the result of a history scan.
type Report struct {
	Findings       []Finding `json:"findings"`
	CommitsScanned int       `json:"commits_scanned"`
}

// Scan walks git history and returns deduplicated secret findings. Findings are
// deduplicated by rule, path, and masked value, keeping the most recent commit
// that introduced the value (git log is newest-first).
func Scan(ctx context.Context, opts Options) (Report, error) {
	repo := strings.TrimSpace(opts.RepoPath)
	if repo == "" {
		repo = "."
	}

	args := []string{"-C", repo, "log", "-p", "-U0", "--no-color", "--no-merges", "--format=" + commitMarker + "%H"}
	if opts.AllRefs {
		args = append(args, "--all")
	}
	if opts.MaxCommits > 0 {
		args = append(args, fmt.Sprintf("-n%d", opts.MaxCommits))
	}

	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed git log subcommand; args are tool-controlled constants plus the scan repo path
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Report{}, err
	}
	if err := cmd.Start(); err != nil {
		return Report{}, fmt.Errorf("git log: %w", err)
	}

	report := parseLog(stdout, opts.Scanner)

	if err := cmd.Wait(); err != nil {
		return Report{}, fmt.Errorf("git log: %w", err)
	}
	return report, nil
}

// logParser holds the streaming state of a `git log -p` walk.
type logParser struct {
	scanner  security.Scanner
	report   Report
	commit   string
	file     string
	newLine  int
	skipFile bool
	seen     map[string]struct{}
	commits  map[string]struct{}
}

func parseLog(reader io.Reader, scanner security.Scanner) Report {
	// bufio.Reader.ReadString grows for arbitrarily long lines, unlike
	// bufio.Scanner, which silently stops on an over-long token — a dangerous
	// failure mode for a security scan (it would skip the rest of history).
	buf := bufio.NewReaderSize(reader, 64*1024)
	parser := &logParser{
		scanner: scanner,
		seen:    make(map[string]struct{}),
		commits: make(map[string]struct{}),
	}
	for {
		raw, err := buf.ReadString('\n')
		if len(raw) > 0 {
			parser.handleLine(strings.TrimRight(raw, "\r\n"))
		}
		if err != nil {
			break
		}
	}
	parser.report.CommitsScanned = len(parser.commits)
	return parser.report
}

// handleLine advances parser state for one line of `git log -p -U0` output.
func (p *logParser) handleLine(line string) {
	switch {
	case strings.HasPrefix(line, commitMarker):
		p.setCommit(strings.TrimSpace(strings.TrimPrefix(line, commitMarker)))
	case strings.HasPrefix(line, "diff --git"):
		p.file, p.skipFile = "", false
	case strings.HasPrefix(line, "+++ "):
		p.setFile(strings.TrimSpace(strings.TrimPrefix(line, "+++ ")))
	case strings.HasPrefix(line, "@@"):
		if m := hunkHeader.FindStringSubmatch(line); m != nil {
			p.newLine, _ = strconv.Atoi(m[1])
		}
	case strings.HasPrefix(line, "--- "), strings.HasPrefix(line, `\`), strings.HasPrefix(line, "-"):
		// old-file header, "\ No newline" marker, and removed lines: no content,
		// and they do not advance the new-file line counter.
	case strings.HasPrefix(line, "+"):
		p.recordAdded(strings.TrimPrefix(line, "+"))
		p.newLine++
	default:
		p.newLine++ // context line
	}
}

func (p *logParser) setCommit(commit string) {
	p.commit = commit
	if commit != "" {
		p.commits[commit] = struct{}{}
	}
}

func (p *logParser) setFile(target string) {
	if target == "/dev/null" {
		p.file, p.skipFile = "", true
		return
	}
	p.file = strings.TrimPrefix(target, "b/")
	p.skipFile = p.scanner.SkipPath(p.file)
}

// recordAdded scans one added line and appends any new (deduplicated) findings.
func (p *logParser) recordAdded(content string) {
	if p.skipFile || p.file == "" {
		return
	}
	for _, match := range p.scanner.ScanContent(content) {
		key := match.RuleID + "|" + p.file + "|" + match.Message
		if _, ok := p.seen[key]; ok {
			continue
		}
		p.seen[key] = struct{}{}
		p.report.Findings = append(p.report.Findings, Finding{
			RuleID:  match.RuleID,
			Level:   match.Level,
			Message: match.Message,
			Path:    p.file,
			Line:    p.newLine,
			Commit:  p.commit,
		})
	}
}
