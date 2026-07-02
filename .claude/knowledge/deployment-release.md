# Deployment & Release

How code gets to production. Release processes, environment promotion, rollback procedures, gotchas.

- **Grammar-subset build tags are required for release-size binaries.** `make build` and `.goreleaser.yaml` pass `-tags grammar_subset,grammar_subset_typescript,grammar_subset_tsx,grammar_subset_javascript` so only the TS/TSX/JS tree-sitter grammars embed (13.6MB -> 18.9MB). A plain `go build` / `go install` embeds all 206 grammars (~41MB) — functional but fat; users installing via `go install` get the fat binary by design. When adding a language to the tree-sitter path, add its `grammar_subset_<lang>` tag in BOTH the Makefile and .goreleaser.yaml.
- `CGO_ENABLED=0` is a hard release constraint: the tree-sitter engine is deliberately pure-Go (gotreesitter, pinned exactly) — never introduce a CGo dependency; it breaks the goreleaser cross-compile matrix and `go install` distribution of the GitHub Action.
