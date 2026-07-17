package compdb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var conventionalDatabasePaths = []string{
	"compile_commands.json",
	"build/compile_commands.json",
	"cmake-build-debug/compile_commands.json",
	"cmake-build-release/compile_commands.json",
}

// Find resolves an explicitly configured database or a conventional CMake
// location. A configured relative path must remain beneath root.
func Find(root, configured string) (string, error) {
	root, err := canonicalRoot(root)
	if err != nil {
		return "", err
	}
	if configured != "" {
		return findConfigured(root, configured)
	}
	return findConventional(root)
}

func findConfigured(root, configured string) (string, error) {
	if filepath.IsAbs(configured) {
		return "", fmt.Errorf("compile_commands path must be relative to the C++ target")
	}
	candidate := filepath.Clean(filepath.Join(root, filepath.FromSlash(configured)))
	if !within(root, candidate) {
		return "", fmt.Errorf("compile_commands path escapes the C++ target")
	}
	if _, err := os.Stat(candidate); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%w at %s", ErrNotFound, filepath.ToSlash(configured))
		}
		return "", err
	}
	return containedDatabasePath(root, candidate)
}

func findConventional(root string) (string, error) {
	for _, rel := range conventionalDatabasePaths {
		candidate := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if resolved, err := containedDatabasePath(root, candidate); err == nil {
			return resolved, nil
		}
	}
	return "", ErrNotFound
}

func containedDatabasePath(root, candidate string) (string, error) {
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil || !within(root, resolved) {
		return "", fmt.Errorf("compile_commands path resolves outside the C++ target")
	}
	return resolved, nil
}
