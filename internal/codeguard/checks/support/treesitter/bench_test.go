package treesitter

import (
	"bytes"
	"strings"
	"testing"
)

// benchCorpora returns the benchmark inputs: the realistic corpus as-is
// (~10 KB, a typical source file) and a 20x concatenation (~200 KB, a large
// generated file).
func benchCorpora(b *testing.B) []struct {
	name string
	data []byte
} {
	small := readCorpus(b, "realistic.ts")
	return []struct {
		name string
		data []byte
	}{
		{name: "small-10KB", data: small},
		{name: "large-200KB", data: bytes.Repeat(small, 20)},
	}
}

func benchID(engine Engine) string {
	return strings.Fields(engine.Name())[0]
}

// BenchmarkFullScan measures end-to-end per-file cost: for the baseline,
// strip + both regexes; for the tree-sitter engines, parse + both rule
// queries.
func BenchmarkFullScan(b *testing.B) {
	for _, corpus := range benchCorpora(b) {
		b.Run("baseline-regex/"+corpus.name, func(b *testing.B) {
			b.SetBytes(int64(len(corpus.data)))
			for b.Loop() {
				if findings := BaselineScan(corpus.data); len(findings) == 0 {
					b.Fatal("expected findings")
				}
			}
		})
		for _, engine := range engines {
			b.Run(benchID(engine)+"/"+corpus.name, func(b *testing.B) {
				b.SetBytes(int64(len(corpus.data)))
				for b.Loop() {
					findings, err := engine.Scan(corpus.data)
					if err != nil {
						b.Fatal(err)
					}
					if len(findings) == 0 {
						b.Fatal("expected findings")
					}
				}
			})
		}
	}
}

// BenchmarkPureGoParseOnly isolates parse cost from query cost for the
// pure-Go runtime; the difference against BenchmarkFullScan is what a
// corpus-level tree cache saves for every rule after the first.
func BenchmarkPureGoParseOnly(b *testing.B) {
	engine, err := newPureGoEngine()
	if err != nil {
		b.Fatal(err)
	}
	for _, corpus := range benchCorpora(b) {
		b.Run(corpus.name, func(b *testing.B) {
			b.SetBytes(int64(len(corpus.data)))
			for b.Loop() {
				if _, err := engine.parse(corpus.data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkPureGoRulesOnCachedTree measures both rule queries against an
// already-parsed tree: the steady-state per-rule cost under a tree cache.
func BenchmarkPureGoRulesOnCachedTree(b *testing.B) {
	engine, err := newPureGoEngine()
	if err != nil {
		b.Fatal(err)
	}
	for _, corpus := range benchCorpora(b) {
		tree, err := engine.parse(corpus.data)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(corpus.name, func(b *testing.B) {
			b.SetBytes(int64(len(corpus.data)))
			for b.Loop() {
				if findings := engine.scanTree(tree, corpus.data); len(findings) == 0 {
					b.Fatal("expected findings")
				}
			}
		})
	}
}
