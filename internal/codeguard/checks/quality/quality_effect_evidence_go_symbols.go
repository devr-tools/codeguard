package quality

type goScope struct {
	id      goScopeID
	parent  *goScope
	symbols map[string]*goSymbol
}

func (scope *goScope) lookup(name string) *goSymbol {
	for current := scope; current != nil; current = current.parent {
		if symbol := current.symbols[name]; symbol != nil {
			return symbol
		}
	}
	return nil
}

func snapshotGoScope(scope *goScope) *goScope {
	if scope == nil {
		return nil
	}
	snapshot := &goScope{id: scope.id, parent: snapshotGoScope(scope.parent), symbols: make(map[string]*goSymbol, len(scope.symbols))}
	for name, symbol := range scope.symbols {
		snapshot.symbols[name] = symbol
	}
	return snapshot
}

// goSymbolTable allocates scopes and symbols and resolves alias and content
// ownership chains between symbols.
type goSymbolTable struct {
	nextID    int
	nextScope goScopeID
	symbols   map[int]*goSymbol
}

func (table *goSymbolTable) newScope(parent *goScope) *goScope {
	table.nextScope++
	return &goScope{id: table.nextScope, parent: parent, symbols: make(map[string]*goSymbol)}
}

func (table *goSymbolTable) declare(scope *goScope, name string, kind goSymbolKind, shape goReferenceShape, origin string, target string, line int) *goSymbol {
	if name == "" || name == "_" {
		return nil
	}
	symbol := table.allocate(&goSymbol{Name: name, Kind: kind, Scope: scope.id, Shape: shape, Origin: origin, Target: target, DeclLine: line})
	scope.symbols[name] = symbol
	return symbol
}

// allocate assigns symbol the next ID and registers it in the table without
// binding a name in any scope.
func (table *goSymbolTable) allocate(symbol *goSymbol) *goSymbol {
	table.nextID++
	symbol.ID = table.nextID
	table.symbols[symbol.ID] = symbol
	return symbol
}

func (table *goSymbolTable) resolveAlias(symbol *goSymbol) *goSymbol {
	seen := make(map[int]bool)
	for hops := 0; symbol != nil && symbol.AliasOf != 0 && hops < goAliasHopLimit; hops++ {
		if seen[symbol.ID] {
			return nil
		}
		seen[symbol.ID] = true
		symbol = table.symbols[symbol.AliasOf]
	}
	return symbol
}

func (table *goSymbolTable) resolveContentOwner(symbol *goSymbol) *goSymbol {
	seen := make(map[int]bool)
	for hops := 0; symbol != nil && hops < goAliasHopLimit; hops++ {
		if seen[symbol.ID] {
			return nil
		}
		seen[symbol.ID] = true
		if symbol.AliasOf != 0 {
			symbol = table.symbols[symbol.AliasOf]
			continue
		}
		if symbol.ContentOf != 0 {
			symbol = table.symbols[symbol.ContentOf]
			continue
		}
		return symbol
	}
	return nil
}

func (table *goSymbolTable) fieldBindingsEquivalent(first *goSymbol, second *goSymbol) bool {
	if first == nil || second == nil || !goReferenceShapesEqual(first.Shape, second.Shape) {
		return false
	}
	firstOwner := table.resolveContentOwner(first)
	secondOwner := table.resolveContentOwner(second)
	if firstOwner == nil || secondOwner == nil {
		return false
	}
	firstGlobal := firstOwner.Kind == goSymbolGlobal
	secondGlobal := secondOwner.Kind == goSymbolGlobal
	return firstGlobal == secondGlobal && firstOwner.Origin == secondOwner.Origin && firstOwner.Target == secondOwner.Target
}

func (table *goSymbolTable) intersectFieldBindings(first map[string]*goSymbol, second map[string]*goSymbol) map[string]*goSymbol {
	intersection := make(map[string]*goSymbol)
	for key, firstSymbol := range first {
		if table.fieldBindingsEquivalent(firstSymbol, second[key]) {
			intersection[key] = firstSymbol
		}
	}
	return intersection
}
