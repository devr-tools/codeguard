package support

// AllFunctions returns every function in the file, including nested ones.
func (file *ParsedFile) AllFunctions() []*ParsedFunction {
	out := make([]*ParsedFunction, 0, len(file.Functions))
	var walk func(fns []*ParsedFunction)
	walk = func(fns []*ParsedFunction) {
		for _, fn := range fns {
			out = append(out, fn)
			walk(fn.Nested)
		}
	}
	walk(file.Functions)
	return out
}

// FunctionByName returns the first top-level or nested function with name.
func (file *ParsedFile) FunctionByName(name string) *ParsedFunction {
	for _, fn := range file.AllFunctions() {
		if fn.Name == name {
			return fn
		}
	}
	return nil
}

// Lookup resolves an identifier against the function's local symbol table.
func (fn *ParsedFunction) Lookup(name string) (SymbolKind, bool) {
	for _, param := range fn.Params {
		if param.Name == name {
			return SymbolParam, true
		}
	}
	for _, assign := range fn.Assignments {
		if assign.Name == name {
			return SymbolLocal, true
		}
	}
	return "", false
}

// LineCount reports how many source lines the function spans.
func (fn *ParsedFunction) LineCount() int {
	if fn.EndLine < fn.StartLine {
		return 1
	}
	return fn.EndLine - fn.StartLine + 1
}
