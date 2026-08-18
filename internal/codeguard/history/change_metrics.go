package history

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	changeCommitMarker         = "@@CG-CHANGE@@"
	maxChangeLogBytes          = 16 << 20
	maxFilesPerChangeCommit    = 200
	maxStoredCoChangeRelations = 100_000
)

var defectSubjectPattern = regexp.MustCompile(`(?i)\b(fix|bug|bugfix|hotfix|regression|revert|incident|defect|broken|failure)\b`)

// ChangeMetricsOptions configures a bounded, read-only git-history summary.
type ChangeMetricsOptions struct {
	RepoPath   string
	MaxCommits int
}

// ChangeMetricsReport summarizes file-level history for maintainability
// signals. Available is false when the directory has no usable git history;
// callers should treat that as "no evidence" instead of a scan failure.
type ChangeMetricsReport struct {
	Available      bool
	CommitsScanned int
	Files          map[string]FileChangeMetrics
}

// FileChangeMetrics aggregates bounded git-log evidence for one path.
type FileChangeMetrics struct {
	Path             string
	Commits          int
	Additions        int
	Deletions        int
	Churn            int
	DefectCommits    int
	Subjects         []string
	CoChangePartners map[string]int
}

type changeCommit struct {
	subject string
	files   map[string]fileDelta
}

type fileDelta struct {
	additions int
	deletions int
}

// CollectChangeMetrics walks recent git history using only local repository
// data. Git failures are returned as an unavailable report with a nil error so
// quality checks degrade gracefully in shallow, detached, or no-history repos.
func CollectChangeMetrics(ctx context.Context, opts ChangeMetricsOptions) (ChangeMetricsReport, error) {
	repo := strings.TrimSpace(opts.RepoPath)
	if repo == "" {
		repo = "."
	}
	maxCommits := opts.MaxCommits
	if maxCommits <= 0 {
		maxCommits = 200
	}

	args := []string{"-C", repo, "log", "--numstat", "--no-color", "--format=" + changeCommitMarker + "%H%x00%s", fmt.Sprintf("-n%d", maxCommits), "--", "."}
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed git log subcommand; repo path and commit limit are caller-controlled scan inputs
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ChangeMetricsReport{}, err
	}
	if err := cmd.Start(); err != nil {
		return ChangeMetricsReport{Files: map[string]FileChangeMetrics{}}, nil
	}
	limitedStdout := &io.LimitedReader{R: stdout, N: maxChangeLogBytes + 1}
	report := parseChangeMetricsContext(ctx, limitedStdout)
	if limitedStdout.N == 0 {
		// Do not leave git blocked while writing the remainder of an oversized log.
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return ChangeMetricsReport{Files: map[string]FileChangeMetrics{}}, nil
	}
	if err := cmd.Wait(); err != nil {
		return ChangeMetricsReport{Files: map[string]FileChangeMetrics{}}, nil
	}
	report.Available = report.CommitsScanned > 0
	if report.Files == nil {
		report.Files = map[string]FileChangeMetrics{}
	}
	return report, nil
}

func parseChangeMetrics(reader io.Reader) ChangeMetricsReport {
	return parseChangeMetricsContext(context.Background(), reader)
}

func parseChangeMetricsContext(ctx context.Context, reader io.Reader) ChangeMetricsReport {
	parser := &changeMetricsParser{ctx: ctx, report: ChangeMetricsReport{Files: map[string]FileChangeMetrics{}}}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			break
		}
		parser.handleLine(strings.TrimRight(scanner.Text(), "\r"))
	}
	parser.flush()
	return parser.report
}

type changeMetricsParser struct {
	ctx                     context.Context
	report                  ChangeMetricsReport
	current                 changeCommit
	active                  bool
	oversizedCommit         bool
	storedCoChangeRelations int
}

func (p *changeMetricsParser) handleLine(line string) {
	if strings.HasPrefix(line, changeCommitMarker) {
		p.flush()
		var subject string
		if idx := strings.IndexByte(line, 0); idx >= 0 {
			subject = line[idx+1:]
		} else {
			subject = strings.TrimPrefix(line, changeCommitMarker)
		}
		p.current = changeCommit{subject: strings.TrimSpace(subject), files: map[string]fileDelta{}}
		p.active = true
		p.oversizedCommit = false
		return
	}
	if !p.active {
		return
	}
	added, deleted, path, ok := parseNumstatLine(line)
	if !ok {
		return
	}
	if p.oversizedCommit {
		return
	}
	if _, exists := p.current.files[path]; !exists && len(p.current.files) >= maxFilesPerChangeCommit {
		// Discard the whole commit rather than retain a path-dependent sample.
		// This also bounds the quadratic co-change work performed by flush.
		p.current.files = nil
		p.oversizedCommit = true
		return
	}
	p.current.files[path] = fileDelta{additions: added, deletions: deleted}
}

func (p *changeMetricsParser) flush() {
	if !p.active {
		return
	}
	if p.oversizedCommit || len(p.current.files) == 0 {
		p.active = false
		return
	}
	p.report.CommitsScanned++
	paths := make([]string, 0, len(p.current.files))
	for path := range p.current.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	defect := defectSubjectPattern.MatchString(p.current.subject)
	for _, path := range paths {
		if p.ctx.Err() != nil {
			p.active = false
			return
		}
		delta := p.current.files[path]
		metric := p.report.Files[path]
		if metric.Path == "" {
			metric.Path = path
		}
		metric.Commits++
		metric.Additions += delta.additions
		metric.Deletions += delta.deletions
		metric.Churn += delta.additions + delta.deletions
		if defect {
			metric.DefectCommits++
		}
		if p.current.subject != "" && len(metric.Subjects) < 12 {
			metric.Subjects = append(metric.Subjects, p.current.subject)
		}
		if metric.CoChangePartners == nil {
			metric.CoChangePartners = map[string]int{}
		}
		for _, partner := range paths {
			if partner != path {
				if _, exists := metric.CoChangePartners[partner]; exists {
					metric.CoChangePartners[partner]++
				} else if p.storedCoChangeRelations < maxStoredCoChangeRelations {
					metric.CoChangePartners[partner] = 1
					p.storedCoChangeRelations++
				}
			}
		}
		p.report.Files[path] = metric
	}
	p.active = false
}

func parseNumstatLine(line string) (int, int, string, bool) {
	parts := strings.Split(line, "\t")
	if len(parts) < 3 {
		return 0, 0, "", false
	}
	path := normalizeNumstatPath(parts[len(parts)-1])
	if path == "" {
		return 0, 0, "", false
	}
	added := parseNumstatCount(parts[0])
	deleted := parseNumstatCount(parts[1])
	return added, deleted, path, true
}

func parseNumstatCount(value string) int {
	if value == "-" {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func normalizeNumstatPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/dev/null" {
		return ""
	}
	if strings.Contains(path, " => ") {
		path = path[strings.LastIndex(path, " => ")+4:]
		path = strings.Trim(path, "{}")
	}
	return filepath.ToSlash(path)
}
