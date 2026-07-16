package buildregression

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/cachefile"
)

const baselineVersion = 1

// BaselineEntry is one stored build-command duration.
type BaselineEntry struct {
	DurationMillis float64 `json:"duration_millis"`
}

type baselineFile struct {
	Version  int                      `json:"version"`
	Commands map[string]BaselineEntry `json:"commands"`
}

// BaselinePathForBase derives the default build-regression baseline path from
// the scan cache path (".codeguard/cache.json" ->
// ".codeguard/cache.build-baseline.json").
func BaselinePathForBase(base string) string {
	trimmed := strings.TrimSpace(base)
	if trimmed == "" {
		return ""
	}
	ext := filepath.Ext(trimmed)
	if ext == "" {
		return trimmed + ".build-baseline"
	}
	return strings.TrimSuffix(trimmed, ext) + ".build-baseline" + ext
}

func LoadBaseline(path string) (map[string]BaselineEntry, bool) {
	var file baselineFile
	if !cachefile.Load(path, &file) || file.Version != baselineVersion || file.Commands == nil {
		return nil, false
	}
	return file.Commands, true
}

func WriteBaseline(path string, results []Result) error {
	commands := make(map[string]BaselineEntry, len(results))
	for _, result := range results {
		commands[result.Name] = BaselineEntry{DurationMillis: result.DurationMillis}
	}
	return saveBaseline(path, commands)
}

func MergeNewCommands(path string, baseline map[string]BaselineEntry, results []Result) (bool, error) {
	added := false
	for _, result := range results {
		if _, ok := baseline[result.Name]; ok {
			continue
		}
		baseline[result.Name] = BaselineEntry{DurationMillis: result.DurationMillis}
		added = true
	}
	if !added {
		return false, nil
	}
	return true, saveBaseline(path, baseline)
}

func saveBaseline(path string, commands map[string]BaselineEntry) error {
	return cachefile.Write(path, baselineFile{Version: baselineVersion, Commands: commands})
}
