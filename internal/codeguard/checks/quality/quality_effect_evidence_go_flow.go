package quality

import (
	"strconv"
)

// goFlowState is the path-sensitive binding state of a walk: which local
// symbols have escaped, which struct fields are bound to known symbols, and
// (through table) each symbol's current binding. Branches snapshot it, walk
// independently, and merge the results conservatively.
type goFlowState struct {
	table     *goSymbolTable
	escapedAt map[int]int
	fields    map[string]*goSymbol
}

func (flow *goFlowState) snapshot() goBranchState {
	return goBranchState{
		escapes: cloneGoEscapeState(flow.escapedAt),
		fields:  cloneGoFieldBindings(flow.fields),
		symbols: cloneGoSymbolBindings(flow.table.symbols),
	}
}

func (flow *goFlowState) restore(state goBranchState) {
	flow.escapedAt = cloneGoEscapeState(state.escapes)
	flow.fields = cloneGoFieldBindings(state.fields)
	for id, binding := range state.symbols {
		if symbol := flow.table.symbols[id]; symbol != nil {
			restoreGoSymbolBinding(symbol, binding)
		}
	}
}

func (flow *goFlowState) merge(before goBranchState, states ...goBranchState) goBranchState {
	if len(states) == 0 {
		return before
	}
	merged := goBranchState{
		escapes: cloneGoEscapeState(states[0].escapes),
		fields:  cloneGoFieldBindings(states[0].fields),
		symbols: cloneGoSymbolBindingStates(before.symbols),
	}
	for _, state := range states[1:] {
		merged.escapes = intersectGoEscapeStates(merged.escapes, state.escapes)
		merged.fields = flow.table.intersectFieldBindings(merged.fields, state.fields)
	}
	for id := range before.symbols {
		candidate, exists := states[0].symbols[id]
		if !exists {
			continue
		}
		equivalent := true
		for _, state := range states[1:] {
			other, ok := state.symbols[id]
			if !ok || !goSymbolBindingStatesEqual(candidate, other) {
				equivalent = false
				break
			}
		}
		if equivalent {
			merged.symbols[id] = candidate
		}
	}
	return merged
}

type goBranchState struct {
	escapes map[int]int
	fields  map[string]*goSymbol
	symbols map[int]goSymbolBindingState
}

type goSymbolBindingState struct {
	shape     goReferenceShape
	origin    string
	target    string
	aliasOf   int
	contentOf int
}

func snapshotGoSymbolBinding(symbol *goSymbol) goSymbolBindingState {
	return goSymbolBindingState{
		shape: symbol.Shape, origin: symbol.Origin, target: symbol.Target,
		aliasOf: symbol.AliasOf, contentOf: symbol.ContentOf,
	}
}

func restoreGoSymbolBinding(symbol *goSymbol, state goSymbolBindingState) {
	symbol.Shape = state.shape
	symbol.Origin = state.origin
	symbol.Target = state.target
	symbol.AliasOf = state.aliasOf
	symbol.ContentOf = state.contentOf
}

func cloneGoSymbolBindings(symbols map[int]*goSymbol) map[int]goSymbolBindingState {
	bindings := make(map[int]goSymbolBindingState, len(symbols))
	for id, symbol := range symbols {
		bindings[id] = snapshotGoSymbolBinding(symbol)
	}
	return bindings
}

func cloneGoSymbolBindingStates(states map[int]goSymbolBindingState) map[int]goSymbolBindingState {
	clone := make(map[int]goSymbolBindingState, len(states))
	for id, state := range states {
		clone[id] = state
	}
	return clone
}

func goSymbolBindingStatesEqual(first goSymbolBindingState, second goSymbolBindingState) bool {
	return goReferenceShapesEqual(first.shape, second.shape) &&
		first.origin == second.origin && first.target == second.target &&
		first.aliasOf == second.aliasOf && first.contentOf == second.contentOf
}

func cloneGoEscapeState(state map[int]int) map[int]int {
	clone := make(map[int]int, len(state))
	for symbolID, escapedAt := range state {
		clone[symbolID] = escapedAt
	}
	return clone
}

func intersectGoEscapeStates(first map[int]int, second map[int]int) map[int]int {
	intersection := make(map[int]int)
	for symbolID, firstEscape := range first {
		secondEscape, exists := second[symbolID]
		if !exists {
			continue
		}
		if secondEscape > firstEscape {
			firstEscape = secondEscape
		}
		intersection[symbolID] = firstEscape
	}
	return intersection
}

func cloneGoFieldBindings(bindings map[string]*goSymbol) map[string]*goSymbol {
	clone := make(map[string]*goSymbol, len(bindings))
	for key, symbol := range bindings {
		clone[key] = symbol
	}
	return clone
}

// goEvidenceRecorder accumulates de-duplicated mutation and unresolved
// evidence for one function.
type goEvidenceRecorder struct {
	analysis mutationAnalysis
	seen     map[string]struct{}
	unseen   map[string]struct{}
}

func (recorder *goEvidenceRecorder) addMutation(target string, effect string, origin string, line int, detail string) {
	key := target + "|" + effect + "|" + origin + "|" + detail
	if _, exists := recorder.seen[key]; exists {
		return
	}
	recorder.seen[key] = struct{}{}
	recorder.analysis.Mutations = append(recorder.analysis.Mutations, mutationEvidence{Target: target, Effect: effect, Origin: origin, Line: line, Detail: detail})
}

func (recorder *goEvidenceRecorder) addUnresolved(line int, operation string, symbol string, reason string) {
	if symbol == "" {
		return
	}
	key := operation + "|" + symbol + "|" + strconv.Itoa(line)
	if _, exists := recorder.unseen[key]; exists {
		return
	}
	recorder.unseen[key] = struct{}{}
	recorder.analysis.Unresolved = append(recorder.analysis.Unresolved, unresolvedMutationEvidence{
		Language: "go", Line: line, Operation: operation, Symbol: symbol, Reason: reason,
	})
}
