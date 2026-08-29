package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type PruneOptions struct{ AllowInvalid bool }

func Load(path string) (core.BaselineFile, error) {
	data, err := os.ReadFile(path) //nolint:gosec // operator-supplied baseline path
	if err != nil {
		return core.BaselineFile{}, err
	}
	var file core.BaselineFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return core.BaselineFile{}, fmt.Errorf("decode baseline: %w", err)
	}
	return file, nil
}

func WritePruned(source, output string, result AuditResult, opts PruneOptions) error {
	if result.Counts.Invalid > 0 && !opts.AllowInvalid {
		return errors.New("baseline contains invalid entries; use the explicit invalid-entry override after review")
	}
	entries := result.ActiveEntries()
	if opts.AllowInvalid {
		for _, audited := range result.Entries {
			if audited.Status == "invalid" {
				entries = append(entries, audited.Entry)
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entryKey(entries[i]) < entryKey(entries[j]) })
	file := core.BaselineFile{GeneratedAt: time.Now().UTC().Format(time.RFC3339), Entries: entries}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if output == "" {
		output = source
	}
	if mkdirErr := os.MkdirAll(filepath.Dir(output), 0o750); mkdirErr != nil {
		return mkdirErr
	}
	tmp, err := os.CreateTemp(filepath.Dir(output), ".codeguard-baseline-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, output)
}
