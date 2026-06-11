package support

import (
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
