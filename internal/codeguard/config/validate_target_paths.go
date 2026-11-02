package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// ValidateTargetPaths checks that every configured target path names an
// existing, readable directory. It is deliberately separate from Validate,
// which stays a pure check of the config's shape: SDK callers and tests
// validate in-memory configs whose paths need not exist on the machine doing
// the validating.
//
// Paths are checked exactly as the scanner will walk them. LoadFile has already
// joined a relative target path onto the config's directory, so a relative path
// that reaches here is relative to the process working directory, which is also
// where the walk resolves it.
//
// A missing or unreadable target must be an error rather than an empty walk:
// an empty walk produces no findings, and no findings reads as a clean pass.
func ValidateTargetPaths(cfg core.Config) error {
	for _, target := range cfg.Targets {
		if err := validateTargetPath(target); err != nil {
			return err
		}
	}
	return nil
}

func validateTargetPath(target core.TargetConfig) error {
	path := strings.TrimSpace(target.Path)
	if path == "" {
		// Validate reports a blank path; nothing to resolve here.
		return nil
	}
	label := targetLabel(target)
	if abs, absErr := filepath.Abs(path); absErr == nil {
		// Report the path the scanner would actually walk, so a relative
		// path is not misread as relative to the config file.
		path = abs
	}
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("target %s path %q does not exist", label, path)
	case err != nil:
		return fmt.Errorf("target %s path %q: %w", label, path, err)
	case !info.IsDir():
		return fmt.Errorf("target %s path %q is not a directory", label, path)
	}
	// Stat succeeds on a directory the scanner cannot list; open it so a
	// permission problem surfaces here instead of as an empty walk.
	dir, err := os.Open(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("target %s path %q is not readable: %w", label, path, err)
	}
	defer func() { _ = dir.Close() }()
	if _, err := dir.ReadDir(1); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("target %s path %q is not readable: %w", label, path, err)
	}
	return nil
}

func targetLabel(target core.TargetConfig) string {
	if name := strings.TrimSpace(target.Name); name != "" {
		return fmt.Sprintf("%q", name)
	}
	if logical := strings.TrimSpace(target.LogicalPath); logical != "" {
		return fmt.Sprintf("%q", logical)
	}
	return "(unnamed)"
}
