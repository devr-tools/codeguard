// Package treesitter is a DESIGN-SPIKE prototype (see docs/treesitter-spike.md).
// It is deliberately its own Go module so that its third-party dependencies
// (tree-sitter CGo bindings) never land in the root module's go.mod/go.sum.
// The root `go build ./...` / `go test ./...` skip this directory entirely.
module github.com/devr-tools/codeguard/internal/codeguard/checks/support/treesitter

go 1.24

require (
	github.com/devr-tools/codeguard v0.0.0
	github.com/odvcencio/gotreesitter v0.20.8
	github.com/tree-sitter/go-tree-sitter v0.25.0
	github.com/tree-sitter/tree-sitter-typescript v0.23.2
)

require (
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/devr-tools/codeguard => ../../../../..
