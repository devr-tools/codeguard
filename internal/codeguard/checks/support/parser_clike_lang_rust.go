package support

func rustSpans(masked string) []clikeSpan {
	spans := make([]clikeSpan, 0, 8)
	for _, match := range rustFnHead.FindAllStringSubmatchIndex(masked, -1) {
		parenAt := rustParamsOpen(masked, match[1])
		if parenAt < 0 {
			continue
		}
		span, ok := resolveSpan(masked, match[0], parenAt, false)
		if !ok {
			continue
		}
		span.name = masked[match[2]:match[3]]
		spans = append(spans, span)
	}
	return spans
}

// rustParamsOpen skips an optional generic parameter list after the
// function name and returns the offset of the opening paren.
func rustParamsOpen(masked string, offset int) int {
	i := offset
	for i < len(masked) && (masked[i] == ' ' || masked[i] == '\t' || masked[i] == '\n') {
		i++
	}
	if i < len(masked) && masked[i] == '<' {
		depth := 0
		for ; i < len(masked); i++ {
			if masked[i] == '<' {
				depth++
			}
			if masked[i] == '>' {
				depth--
				if depth == 0 {
					i++
					break
				}
			}
		}
	}
	for i < len(masked) && (masked[i] == ' ' || masked[i] == '\t' || masked[i] == '\n') {
		i++
	}
	if i < len(masked) && masked[i] == '(' {
		return i
	}
	return -1
}
