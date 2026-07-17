package security

// cppTaint tracks one C++ value whose origin is untrusted. paramIndex is set
// for a function parameter whose taint depends on its caller.
type cppTaint struct {
	source     string
	sourceLine int
	chain      []string
	paramIndex int
}

func (t *cppTaint) extended(step string) *cppTaint {
	next := *t
	next.chain = append(append([]string{}, t.chain...), step)
	return &next
}

func preferCPPTaint(left *cppTaint, right *cppTaint) *cppTaint {
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

type cppParamSink struct {
	paramIndex int
	sink       string
	line       int
}

type cppSummary struct {
	returnTaint    *cppTaint
	paramsToReturn map[int]bool
	paramsToSink   []cppParamSink
}
