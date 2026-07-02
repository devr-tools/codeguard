package support

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// ListChangedFiles returns the files that differ between the diff base ref and
// the working tree for the given target. Outside diff mode it returns nil so
// base-comparison checks can no-op gracefully.
func ListChangedFiles(ctx context.Context, sc Context, target core.TargetConfig) ([]core.ChangedFile, error) {
	if sc.Opts.Mode != core.ScanModeDiff {
		return nil, nil
	}
	if err := ValidateBaseRef(sc.Opts.BaseRef); err != nil {
		return nil, err
	}
	var lastErr error
	for _, ref := range []string{sc.Opts.BaseRef, sc.Opts.BaseRef + "...HEAD"} {
		output, err := runGitCapture(ctx, "-C", target.Path, "diff", "--name-status", "--no-renames", "--no-color", "--end-of-options", ref, "--")
		if err == nil {
			return parseNameStatus(string(output)), nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("diff mode requires git diff --name-status against %q: %w", sc.Opts.BaseRef, lastErr)
}

func parseNameStatus(output string) []core.ChangedFile {
	changed := make([]core.ChangedFile, 0)
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(strings.TrimRight(line, "\r"), "\t", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			continue
		}
		changed = append(changed, core.ChangedFile{
			Status: core.ChangedFileStatus(parts[0][:1]),
			Path:   filepath.ToSlash(parts[1]),
		})
	}
	return changed
}

// ReadBaseFile returns the contents of a target-relative file at the diff
// base ref.
func ReadBaseFile(ctx context.Context, sc Context, target core.TargetConfig, rel string) ([]byte, error) {
	if err := ValidateBaseRef(sc.Opts.BaseRef); err != nil {
		return nil, err
	}
	return runGitCapture(ctx, "-C", target.Path, "show", "--end-of-options", sc.Opts.BaseRef+":./"+filepath.ToSlash(rel))
}
