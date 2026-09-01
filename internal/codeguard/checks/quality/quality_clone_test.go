package quality

import (
	"fmt"
	"testing"

	checksupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func BenchmarkCloneWindowIndexHighEntropy(b *testing.B) {
	for _, tokens := range []int{10_000, 100_000} {
		b.Run(fmt.Sprintf("tokens-%d", tokens), func(b *testing.B) {
			doc := cloneDocument{Path: "unique.go", Tokens: make([]cloneToken, tokens)}
			for i := range doc.Tokens {
				doc.Tokens[i] = cloneToken{Value: fmt.Sprintf("token%d", i), Hash: uint64(i + 1)}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				index, truncated := cloneWindowIndexBounded([]cloneDocument{doc}, 90, defaultCloneAnalysisBudget.maxWindows)
				if truncated || len(index) == 0 {
					b.Fatalf("unexpected bounded index result: keys=%d truncated=%v", len(index), truncated)
				}
			}
		})
	}
}

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

func TestCloneWindowIndexSkipsMultiplierForOversizedThreshold(t *testing.T) {
	docs := []cloneDocument{
		{Tokens: []cloneToken{{Hash: 1}}},
		{Tokens: []cloneToken{{Hash: 1}}},
	}
	threshold := int(^uint(0) >> 1)

	if index, _ := cloneWindowIndexBounded(docs, threshold, defaultCloneAnalysisBudget.maxWindows); len(index) != 0 {
		t.Fatalf("cloneWindowIndex() returned %d windows, want 0", len(index))
	}
}

func TestCloneWindowIndexStopsAtOccurrenceBudget(t *testing.T) {
	tokens := make([]cloneToken, 10)
	for i := range tokens {
		tokens[i] = cloneToken{Value: string(rune('a' + i)), Hash: uint64(i + 1)}
	}
	index, truncated := cloneWindowIndexBounded([]cloneDocument{{Path: "unique.go", Tokens: tokens}}, 1, 5)
	if !truncated {
		t.Fatal("clone window index did not report the exhausted occurrence budget")
	}
	occurrences := 0
	for _, bucket := range index {
		occurrences += len(bucket)
	}
	if occurrences != 5 {
		t.Fatalf("indexed occurrences = %d, want hard budget 5", occurrences)
	}
}

func TestCloneDocumentsStopAtAggregateTokenBudget(t *testing.T) {
	env := checksupport.Context{
		Config: core.Config{Checks: core.CheckConfig{QualityRules: core.QualityRulesConfig{CloneTokenThreshold: 1}}},
		VisitTargetFiles: func(_ core.TargetConfig, _ func(string) bool, visit func(string, []byte)) {
			visit("app.ts", []byte("const one = alpha + beta + gamma + delta;"))
			visit("more.ts", []byte("const two = epsilon + zeta;"))
		},
	}
	docs, truncated := cloneDocumentsForTargetWithBudget(env, core.TargetConfig{Language: "typescript"}, cloneAnalysisBudget{
		maxSourceBytes: 1 << 20,
		maxTokens:      5,
		maxWindows:     5,
	})
	if !truncated {
		t.Fatal("clone document collection did not report the exhausted token budget")
	}
	total := 0
	for _, doc := range docs {
		total += len(doc.Tokens)
	}
	if total > 5 {
		t.Fatalf("retained clone tokens = %d, budget = 5", total)
	}

	analysis := cloneFindingsForTargetWithBudget(env, core.TargetConfig{Name: "repo", Language: "typescript"}, cloneAnalysisBudget{
		maxSourceBytes: 1 << 20,
		maxTokens:      5,
		maxWindows:     5,
	})
	if len(analysis.diagnostics) != 1 || analysis.diagnostics[0].ID != "quality.duplicate-code-budget" {
		t.Fatalf("budget diagnostics = %+v, want quality.duplicate-code-budget", analysis.diagnostics)
	}
}
