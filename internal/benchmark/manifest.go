// Package benchmark defines the reproducible, repository-local PR benchmark
// protocol. It deliberately does not clone repositories: CI or a maintainer
// must provision a frozen checkout before this package is invoked.
package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const SchemaVersion = 1

// Manifest identifies immutable PR snapshots. Worktree is deliberately a
// relative directory under the harness work root, never an arbitrary path.
type Manifest struct {
	Version int     `json:"version"`
	Corpus  string  `json:"corpus"`
	Entries []Entry `json:"entries"`
}

type Entry struct {
	ID           string `json:"id"`
	Language     string `json:"language"`
	Repository   string `json:"repository"`
	PullRequest  int    `json:"pull_request"`
	BaseRevision string `json:"base_revision"`
	HeadRevision string `json:"head_revision"`
	Worktree     string `json:"worktree"`
	Config       string `json:"config"`
}

// Load reads and validates a benchmark manifest.
func Load(path string) (Manifest, error) {
	// #nosec G304 -- the caller explicitly selects the benchmark manifest.
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read benchmark manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse benchmark manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (m Manifest) Validate() error {
	if m.Version != SchemaVersion {
		return fmt.Errorf("benchmark manifest version must be %d", SchemaVersion)
	}
	if strings.TrimSpace(m.Corpus) == "" {
		return fmt.Errorf("benchmark manifest corpus is required")
	}
	if len(m.Entries) == 0 {
		return fmt.Errorf("benchmark manifest must contain at least one entry")
	}
	seen := make(map[string]bool, len(m.Entries))
	for _, entry := range m.Entries {
		if err := entry.validate(); err != nil {
			return err
		}
		if seen[entry.ID] {
			return fmt.Errorf("benchmark manifest has duplicate entry id %q", entry.ID)
		}
		seen[entry.ID] = true
	}
	return nil
}

func (e Entry) validate() error {
	for field, value := range map[string]string{
		"id": e.ID, "language": e.Language, "repository": e.Repository,
		"base_revision": e.BaseRevision, "head_revision": e.HeadRevision,
		"worktree": e.Worktree, "config": e.Config,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("benchmark entry %q: %s is required", e.ID, field)
		}
	}
	if e.PullRequest < 1 {
		return fmt.Errorf("benchmark entry %q: pull_request must be positive", e.ID)
	}
	if !isSafeRelativePath(e.Worktree) || !isSafeRelativePath(e.Config) {
		return fmt.Errorf("benchmark entry %q: worktree and config must be safe relative paths", e.ID)
	}
	return nil
}

func isSafeRelativePath(path string) bool {
	if filepath.IsAbs(path) || path == "." || strings.TrimSpace(path) != path {
		return false
	}
	for _, part := range strings.FieldsFunc(filepath.ToSlash(path), func(r rune) bool { return r == '/' }) {
		if part == ".." || part == "." || part == "" {
			return false
		}
	}
	return true
}

// Export is a stable, machine-readable inventory suitable for publishing with
// benchmark results. It intentionally omits local worktree/config paths.
func (m Manifest) Export() Export {
	entries := make([]ExportEntry, 0, len(m.Entries))
	for _, entry := range m.Entries {
		entries = append(entries, ExportEntry{ID: entry.ID, Language: entry.Language, Repository: entry.Repository, PullRequest: entry.PullRequest, BaseRevision: entry.BaseRevision, HeadRevision: entry.HeadRevision})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return Export{Version: SchemaVersion, Corpus: m.Corpus, Entries: entries}
}

type Export struct {
	Version int           `json:"version"`
	Corpus  string        `json:"corpus"`
	Entries []ExportEntry `json:"entries"`
}

type ExportEntry struct {
	ID           string `json:"id"`
	Language     string `json:"language"`
	Repository   string `json:"repository"`
	PullRequest  int    `json:"pull_request"`
	BaseRevision string `json:"base_revision"`
	HeadRevision string `json:"head_revision"`
}

func WriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode benchmark JSON: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write benchmark JSON: %w", err)
	}
	return nil
}
