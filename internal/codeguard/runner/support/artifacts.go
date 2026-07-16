package support

import (
	"sort"
	"sync"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// ArtifactStore collects the artifacts produced across all sections. Its methods
// are safe for concurrent use so that sections may run in parallel.
type ArtifactStore struct {
	mu    sync.Mutex
	items map[string]core.Artifact
}

func NewArtifactStore() *ArtifactStore {
	return &ArtifactStore{items: make(map[string]core.Artifact)}
}

func (store *ArtifactStore) Put(artifact core.Artifact) {
	if store == nil || artifact.ID == "" {
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.items[artifact.ID] = artifact
}

func (store *ArtifactStore) Get(id string) (core.Artifact, bool) {
	if store == nil {
		return core.Artifact{}, false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	artifact, ok := store.items[id]
	return artifact, ok
}

func (store *ArtifactStore) List() []core.Artifact {
	if store == nil {
		return nil
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.items) == 0 {
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
// graphs) always observe every file. It reuses the shared per-scan corpus, so
// files are still walked and read only once across the whole scan.
func VisitTargetFiles(sc Context, target core.TargetConfig, include func(string) bool, visit func(rel string, data []byte)) {
	files, _ := sc.corpusFiles(target.Path)
	for _, file := range files {
		if !include(file) {
			continue
		}
		data, err := sc.corpusRead(target.Path, file)
		if err != nil {
			continue
		}
		visit(file, data)
	}
}

// ListTargetFiles returns every non-excluded file under the target root,
// sharing the per-scan corpus walk with every other section (the same listing
// VisitTargetFiles iterates). Callers apply their own include filter to the
// result.
func ListTargetFiles(sc Context, target core.TargetConfig) ([]string, error) {
	return sc.corpusFiles(target.Path)
}

// ReadTargetFile returns the bytes of target-root-relative rel via the shared
// per-scan corpus, so a file inspected by several checks is read from disk at
// most once per scan.
func ReadTargetFile(sc Context, target core.TargetConfig, rel string) ([]byte, error) {
	return sc.corpusRead(target.Path, rel)
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
