package quality

import "testing"

func TestCloneWindowIndexSkipsMultiplierForOversizedThreshold(t *testing.T) {
	docs := []cloneDocument{
		{Tokens: []cloneToken{{Hash: 1}}},
		{Tokens: []cloneToken{{Hash: 1}}},
	}
	threshold := int(^uint(0) >> 1)

	if index := cloneWindowIndex(docs, threshold); len(index) != 0 {
		t.Fatalf("cloneWindowIndex() returned %d windows, want 0", len(index))
	}
}
