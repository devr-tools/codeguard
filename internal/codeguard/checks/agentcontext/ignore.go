package agentcontext

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// defaultAmbiguousBasenameIgnore lists source-file basenames whose repetition
// is imposed by a language or framework convention rather than chosen by the
// author: agents already know that every package has an __init__.py and every
// module directory an index.ts, so repeats of these names carry no
// navigational ambiguity. The set is deliberately limited to names a
// toolchain or dominant framework prescribes:
//   - JS/TS module entrypoints: index.{ts,tsx,js,jsx,mjs,cjs}
//   - file-system routers (Next.js, SvelteKit, Remix-style): route.ts,
//     routes.ts, page.tsx, layout.tsx
//   - Python package markers: __init__.py, __main__.py
//   - Rust module layout: mod.rs, lib.rs, main.rs
//   - Go idioms: main.go (package main), doc.go (package docs), types.go
//     (per-package type declarations)
//
// context_rules.ambiguous_symbol_ignore REPLACES this list when set.
var defaultAmbiguousBasenameIgnore = []string{
	"index.ts", "index.tsx", "index.js", "index.jsx", "index.mjs", "index.cjs",
	"route.ts", "routes.ts", "page.tsx", "layout.tsx",
	"__init__.py", "__main__.py",
	"mod.rs", "lib.rs", "main.rs",
	"main.go", "doc.go", "types.go",
}

// ambiguousIgnoreSet resolves the effective conventional-basename ignore set.
// Replace semantics: a non-nil config list fully replaces the defaults, so
// users can both extend the set (re-list defaults plus their own names) and
// disable it entirely (set it to []). Matching is case-insensitive.
func ambiguousIgnoreSet(rules core.ContextRulesConfig) map[string]struct{} {
	list := defaultAmbiguousBasenameIgnore
	if rules.AmbiguousSymbolIgnore != nil {
		list = rules.AmbiguousSymbolIgnore
	}
	set := make(map[string]struct{}, len(list))
	for _, name := range list {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			set[name] = struct{}{}
		}
	}
	return set
}
