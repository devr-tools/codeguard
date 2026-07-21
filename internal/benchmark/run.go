package benchmark

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"time"
)

// Result is the machine-readable output schema for one harness invocation.
// cold is the first fresh process for an entry; warm repeats the exact scan in
// the same provisioned checkout and can use CodeGuard's normal local cache.
type Result struct {
	Version   int         `json:"version"`
	Corpus    string      `json:"corpus"`
	Tool      string      `json:"tool"`
	Generated time.Time   `json:"generated_at"`
	Platform  Platform    `json:"platform"`
	Runs      []RunResult `json:"runs"`
}

type Platform struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
}

type RunResult struct {
	ID       string        `json:"id"`
	Language string        `json:"language"`
	Mode     string        `json:"mode"`
	Attempt  int           `json:"attempt"`
	Duration time.Duration `json:"duration_ns"`
	ExitCode int           `json:"exit_code"`
	Error    string        `json:"error,omitempty"`
}

type RunOptions struct {
	Binary      string
	WorkRoot    string
	WarmRepeats int
	Now         func() time.Time
}

// Run executes diff scans against already-provisioned immutable checkouts.
// It neither fetches nor modifies source; cache behavior belongs to the
// checked-out configuration. A non-zero scan is retained as data, so a corpus
// containing intentional findings does not make the entire measurement fail.
func Run(ctx context.Context, manifest Manifest, options RunOptions) (Result, error) {
	if err := manifest.Validate(); err != nil {
		return Result{}, err
	}
	if options.Binary == "" || options.WorkRoot == "" {
		return Result{}, fmt.Errorf("benchmark binary and work root are required")
	}
	if options.WarmRepeats < 1 {
		return Result{}, fmt.Errorf("warm repeats must be at least 1")
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	result := Result{Version: SchemaVersion, Corpus: manifest.Corpus, Tool: options.Binary, Generated: now().UTC(), Platform: Platform{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}}
	entries := append([]Entry(nil), manifest.Entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	for _, entry := range entries {
		worktree := filepath.Join(options.WorkRoot, entry.Worktree)
		if err := runEntry(ctx, &result, options.Binary, worktree, entry, "cold", 1); err != nil {
			return result, err
		}
		for attempt := 1; attempt <= options.WarmRepeats; attempt++ {
			if err := runEntry(ctx, &result, options.Binary, worktree, entry, "warm", attempt); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func runEntry(ctx context.Context, result *Result, binary, worktree string, entry Entry, mode string, attempt int) error {
	info, err := os.Stat(worktree)
	if err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("not a directory")
		}
		return fmt.Errorf("benchmark entry %q worktree %q: %w", entry.ID, worktree, err)
	}
	config := filepath.Join(worktree, entry.Config)
	args := []string{"scan", "-config", config, "-mode", "diff", "-base-ref", entry.BaseRevision}
	started := time.Now()
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = worktree
	err = command.Run()
	run := RunResult{ID: entry.ID, Language: entry.Language, Mode: mode, Attempt: attempt, Duration: time.Since(started)}
	if exitErr, ok := err.(*exec.ExitError); ok {
		run.ExitCode = exitErr.ExitCode()
	} else if err != nil {
		run.Error = err.Error()
	}
	result.Runs = append(result.Runs, run)
	return nil
}
