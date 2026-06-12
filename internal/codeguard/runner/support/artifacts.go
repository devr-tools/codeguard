package support

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type ArtifactStore struct {
	items map[string]core.Artifact
}

func NewArtifactStore() *ArtifactStore {
	return &ArtifactStore{items: make(map[string]core.Artifact)}
}

func (store *ArtifactStore) Put(artifact core.Artifact) {
	if store == nil || artifact.ID == "" {
		return
	}
	store.items[artifact.ID] = artifact
}

func (store *ArtifactStore) Get(id string) (core.Artifact, bool) {
	if store == nil {
		return core.Artifact{}, false
	}
	artifact, ok := store.items[id]
	return artifact, ok
}

func (store *ArtifactStore) List() []core.Artifact {
	if store == nil || len(store.items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(store.items))
	for id := range store.items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	artifacts := make([]core.Artifact, 0, len(ids))
	for _, id := range ids {
		artifacts = append(artifacts, store.items[id])
	}
	return artifacts
}

// VisitTargetFiles walks target files like ScanTargetFiles but bypasses the
// findings cache, so callers that build cross-file state (such as import
// graphs) always observe every file.
func VisitTargetFiles(sc Context, target core.TargetConfig, include func(string) bool, visit func(rel string, data []byte)) {
	files, _ := WalkFiles(target.Path, sc.Cfg.Exclude, include)
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(target.Path, file))
		if err != nil {
			continue
		}
		visit(file, data)
	}
}

// ChangedDiffFiles returns the sorted set of changed file paths in diff mode.
func ChangedDiffFiles(sc Context) []string {
	if len(sc.Diff) == 0 {
		return nil
	}
	files := make([]string, 0, len(sc.Diff))
	for path := range sc.Diff {
		files = append(files, path)
	}
	sort.Strings(files)
	return files
}
