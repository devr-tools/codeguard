// Package cachefile provides shared JSON cache persistence used by the scan
// cache and the AI verdict caches.
package cachefile

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxCacheFileBytes caps how much of a cache file is read into memory, guarding
// against an oversized or malicious file exhausting memory.
const maxCacheFileBytes = 32 << 20

// Load reads a JSON cache file into payload and reports whether payload was
// populated from disk. A blank path, missing file, or malformed payload is
// treated as a cache miss.
func Load(path string, payload any) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	f, err := os.Open(path) //nolint:gosec // config-supplied cache path; read is size-capped by LimitReader below
	if err != nil {
		return false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxCacheFileBytes))
	if err != nil {
		return false
	}
	return json.Unmarshal(data, payload) == nil
}

// Write marshals payload with indentation and writes it to path, creating
// parent directories as needed.
func Write(path string, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

type entriesEnvelope[V any] struct {
	Version int          `json:"version"`
	Entries map[string]V `json:"entries"`
}

// LoadEntries reads a versioned {version, entries} cache file and returns its
// entries, or nil when the file is absent, malformed, or version-mismatched.
func LoadEntries[V any](path string, version int) map[string]V {
	var file entriesEnvelope[V]
	if !Load(path, &file) || file.Version != version {
		return nil
	}
	return file.Entries
}

// WriteEntries persists entries inside a versioned {version, entries} envelope.
func WriteEntries[V any](path string, version int, entries map[string]V) error {
	return Write(path, entriesEnvelope[V]{Version: version, Entries: entries})
}
