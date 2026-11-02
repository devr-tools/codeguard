package security

// taintTrace tracks one tainted value for the Go, Python and C++ taint
// engines: where it came from and the assignment/call chain it travelled
// through. paramIndex >= 0 marks a value that is only tainted when the
// enclosing function receives a tainted argument at that position. model and
// sinkModel name the framework models that recognised the source and sink;
// engines without framework models leave them empty.
type taintTrace struct {
	source     string
	sourceLine int
	chain      []string
	paramIndex int
	model      string
	sinkModel  string
}

// sourceTrace starts a concrete (caller-independent) trace at a source.
func sourceTrace(source string, line int, model string) *taintTrace {
	return &taintTrace{source: source, sourceLine: line, chain: []string{source}, paramIndex: -1, model: model}
}

func (t *taintTrace) extended(step string) *taintTrace {
	next := *t
	next.chain = append(append([]string{}, t.chain...), step)
	return &next
}

// withSinkModel tags the trace with the framework model that recognised its
// sink. An existing sink model is kept.
func (t *taintTrace) withSinkModel(model string) *taintTrace {
	if t == nil || t.sinkModel != "" || model == "" {
		return t
	}
	next := *t
	next.sinkModel = model
	return &next
}

// preferTaint picks a concrete source over a parameter-conditional taint.
func preferTaint(left *taintTrace, right *taintTrace) *taintTrace {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	if left.paramIndex >= 0 && right.paramIndex < 0 {
		return right
	}
	return left
}

// paramSink records that the parameter at paramIndex reaches sink at line.
type paramSink struct {
	paramIndex int
	sink       string
	line       int
}

// funcSummary captures the cross-function taint behaviour of one function:
// a concrete tainted return, parameters that flow to the return, and
// parameters that reach sinks.
type funcSummary struct {
	returnTaint    *taintTrace
	paramsToReturn map[int]bool
	paramsToSink   []paramSink
}

func newFuncSummary() *funcSummary {
	return &funcSummary{paramsToReturn: map[int]bool{}}
}
