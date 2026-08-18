package quality

import "testing"

func TestCollectCloneCandidatesCapsIdenticalDocuments(t *testing.T) {
	const documentCount = 100
	docs := make([]cloneDocument, documentCount)
	occurrences := make([]cloneOccurrence, documentCount)
	for i := range docs {
		docs[i] = cloneDocument{Path: "same.go", Tokens: []cloneToken{{Value: "same", Hash: 1}}}
		occurrences[i] = cloneOccurrence{DocIndex: i}
	}

	candidates := collectCloneCandidates(cloneIndex{1: occurrences}, docs, 1)
	if len(candidates) != maxCloneCandidates {
		t.Fatalf("candidate count = %d, want safety cap %d", len(candidates), maxCloneCandidates)
	}
}
