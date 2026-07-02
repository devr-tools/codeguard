//go:build cgo

package treesitter

import "testing"

// BenchmarkCGoParseOnly is the native-runtime counterpart of
// BenchmarkPureGoParseOnly.
func BenchmarkCGoParseOnly(b *testing.B) {
	engine, err := newCGoEngine()
	if err != nil {
		b.Fatal(err)
	}
	for _, corpus := range benchCorpora(b) {
		b.Run(corpus.name, func(b *testing.B) {
			b.SetBytes(int64(len(corpus.data)))
			for b.Loop() {
				tree, err := engine.parse(corpus.data)
				if err != nil {
					b.Fatal(err)
				}
				tree.Close()
			}
		})
	}
}

// BenchmarkCGoRulesOnCachedTree is the native-runtime counterpart of
// BenchmarkPureGoRulesOnCachedTree.
func BenchmarkCGoRulesOnCachedTree(b *testing.B) {
	engine, err := newCGoEngine()
	if err != nil {
		b.Fatal(err)
	}
	for _, corpus := range benchCorpora(b) {
		tree, err := engine.parse(corpus.data)
		if err != nil {
			b.Fatal(err)
		}
		defer tree.Close()
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
