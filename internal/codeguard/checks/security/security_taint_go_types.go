package security

// goTaint tracks one tainted value: where it came from and the assignment
// chain it travelled through. paramIndex >= 0 marks values that are only
// tainted when the enclosing function receives a tainted argument.
type goTaint struct {
	source     string
	sourceLine int
	chain      []string
	paramIndex int
	model      string
	sinkModel  string
}

func (t *goTaint) withSinkModel(model string) *goTaint {
	if t == nil || t.sinkModel != "" || model == "" {
		return t
	}
	next := *t
	next.sinkModel = model
	return &next
}

func (t *goTaint) extended(step string) *goTaint {
	next := *t
	next.chain = append(append([]string{}, t.chain...), step)
	return &next
}

type goParamSink struct {
	paramIndex int
	sink       string
	line       int
}

// goFuncSummary captures cross-function taint behavior of one declaration.
type goFuncSummary struct {
	returnTaint    *goTaint
	paramsToReturn map[int]bool
	paramsToSink   []goParamSink
}
