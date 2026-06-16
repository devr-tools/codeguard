package support

// SymbolKind classifies a name resolved inside a function scope.
type SymbolKind string

const (
	SymbolParam  SymbolKind = "param"
	SymbolLocal  SymbolKind = "local"
	SymbolImport SymbolKind = "import"
)

// ParsedParam is a single declared parameter of a function.
type ParsedParam struct {
	Name string
	Type string
}

// ParsedAssignment records "name = expr" style statements inside a scope.
// Expr is taken from the masked source: string contents are blanked while
// interpolated expressions (f-strings, template literals) are preserved.
type ParsedAssignment struct {
	Name      string
	Expr      string
	Line      int
	Augmented bool
}

// ParsedCall is a call expression discovered inside a scope.
type ParsedCall struct {
	Callee string
	Args   []string
	Line   int
}

// ParsedImport records one imported binding.
type ParsedImport struct {
	Module string
	Name   string
	Alias  string
	Line   int
}

// ParsedStatement is one logical statement. Text is the masked form and is
// byte-for-byte aligned with Raw (masking never changes lengths or newlines).
type ParsedStatement struct {
	Line   int
	Indent int
	Text   string
	Raw    string
}

// ParsedFunction is a lightweight AST node for one function or method.
type ParsedFunction struct {
	Name        string
	StartLine   int
	EndLine     int
	Signature   string
	Params      []ParsedParam
	Statements  []ParsedStatement
	Assignments []ParsedAssignment
	Calls       []ParsedCall
	Nested      []*ParsedFunction
}

// ParsedFile is the result of parsing one source file.
type ParsedFile struct {
	Language  string
	Source    string
	Masked    string
	Imports   []ParsedImport
	Functions []*ParsedFunction
	Module    *ParsedFunction
}
