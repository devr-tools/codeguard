package design

// moduleGraphImport retains the source-level import behind a graph edge. The
// dependency graph intentionally contains local modules only, while boundary
// policies also need to inspect imports of external packages and SDKs.
type moduleGraphImport struct {
	from       string
	to         string
	sourceFile string
	specifier  string
	line       int
}
