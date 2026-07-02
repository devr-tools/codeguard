package treesitter

// Engine is one tree-sitter runtime under evaluation. Scan parses the
// source and evaluates both spike rules against the resulting tree.
type Engine interface {
	Name() string
	Scan(source []byte) ([]Finding, error)
}

// engines collects the runtimes compiled into this build: the pure-Go
// runtime is always present; the CGo runtime registers itself from
// engine_cgo.go when the build has cgo enabled.
var engines []Engine
