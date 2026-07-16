package benchregression

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/cachefile"
)

// baselineVersion guards the on-disk format; a mismatched file is treated as
// missing so a format change re-baselines instead of mis-comparing (mirrors
// runner/support slop_history.go).
const baselineVersion = 1

// BaselineEntry is one stored benchmark measurement.
type BaselineEntry struct {
	NsPerOp     float64 `json:"ns_per_op"`
	BytesPerOp  float64 `json:"bytes_per_op,omitempty"`
	AllocsPerOp float64 `json:"allocs_per_op,omitempty"`
}

type baselineFile struct {
	Version    int                      `json:"version"`
	Benchmarks map[string]BaselineEntry `json:"benchmarks"`
}

// BaselinePathForBase derives the default benchmark-baseline path from the
// scan cache path (".codeguard/cache.json" -> ".codeguard/cache.bench-baseline.json"),
// mirroring the slop-history naming convention so the file lands in the
// already-contained cache directory.
func BaselinePathForBase(base string) string {
	trimmed := strings.TrimSpace(base)
	if trimmed == "" {
		return ""
	}
	ext := filepath.Ext(trimmed)
	if ext == "" {
		return trimmed + ".bench-baseline"
	}
	return strings.TrimSuffix(trimmed, ext) + ".bench-baseline" + ext
}

// LoadBaseline reads the stored baseline. ok is false when the file is
// missing, unreadable, or from another format version — callers then treat
// the current run as the first one and write a fresh baseline.
func LoadBaseline(path string) (map[string]BaselineEntry, bool) {
	var file baselineFile
	if !cachefile.Load(path, &file) || file.Version != baselineVersion || file.Benchmarks == nil {
		return nil, false
	}
	return file.Benchmarks, true
}

// WriteBaseline persists results as the new baseline.
func WriteBaseline(path string, results []Result) error {
	benchmarks := make(map[string]BaselineEntry, len(results))
	for _, result := range results {
		benchmarks[result.Name] = entryFromResult(result)
	}
	return saveBaseline(path, benchmarks)
}

// MergeNewBenchmarks adds results whose names are absent from baseline and
// persists the merged file. Existing entries are never overwritten: the
// baseline must stay stable, otherwise a regressed run would silently become
// the new normal on the next scan. It returns whether anything was added.
func MergeNewBenchmarks(path string, baseline map[string]BaselineEntry, results []Result) (bool, error) {
	added := false
	for _, result := range results {
		if _, ok := baseline[result.Name]; ok {
			continue
		}
		baseline[result.Name] = entryFromResult(result)
		added = true
	}
	if !added {
		return false, nil
	}
	return true, saveBaseline(path, baseline)
}

func entryFromResult(result Result) BaselineEntry {
	return BaselineEntry{
		NsPerOp:     result.NsPerOp,
		BytesPerOp:  result.BytesPerOp,
		AllocsPerOp: result.AllocsPerOp,
	}
}

func saveBaseline(path string, benchmarks map[string]BaselineEntry) error {
	payload := baselineFile{Version: baselineVersion, Benchmarks: benchmarks}
	return cachefile.Write(path, payload)
}
