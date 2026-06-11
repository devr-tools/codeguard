package support

import (
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestArtifactStoreListSortsAndReplaces(t *testing.T) {
	store := NewArtifactStore()
	store.Put(core.Artifact{ID: "b", Kind: "dependency_graph", Language: "python"})
	store.Put(core.Artifact{ID: "a", Kind: "dependency_graph", Language: "go"})
	store.Put(core.Artifact{ID: "b", Kind: "dependency_graph", Language: "typescript"})

	artifacts := store.List()
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(artifacts))
	}
	if artifacts[0].ID != "a" || artifacts[1].ID != "b" {
		t.Fatalf("expected sorted artifact IDs [a b], got [%s %s]", artifacts[0].ID, artifacts[1].ID)
	}
	if artifacts[1].Language != "typescript" {
		t.Fatalf("expected replacement artifact language typescript, got %q", artifacts[1].Language)
	}
}
