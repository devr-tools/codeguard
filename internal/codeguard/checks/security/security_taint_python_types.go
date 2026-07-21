package security

// pyTaint tracks one tainted Python value. paramIndex >= 0 marks values that
// are only tainted when the enclosing function receives a tainted argument.
type pyTaint struct {
	source     string
	sourceLine int
	chain      []string
	paramIndex int
	model      string
	sinkModel  string
}

func (t *pyTaint) extended(step string) *pyTaint {
	next := *t
	next.chain = append(append([]string{}, t.chain...), step)
	return &next
}

func preferPyTaint(left *pyTaint, right *pyTaint) *pyTaint {
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

type pyParamSink struct {
	paramIndex int
	sink       string
	line       int
}

type pySummary struct {
	returnTaint    *pyTaint
	paramsToReturn map[int]bool
	paramsToSink   []pyParamSink
}
