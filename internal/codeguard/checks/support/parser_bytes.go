package support

func isPythonPrefixLetter(ch byte) bool {
	switch ch {
	case 'r', 'R', 'b', 'B', 'u', 'U', 'f', 'F':
		return true
	default:
		return false
	}
}

func isIdentByte(ch byte) bool {
	switch {
	case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9', ch == '_':
		return true
	default:
		return false
	}
}

// bracketDelta is the net change in bracket nesting across a masked line.
func bracketDelta(maskedLine string) int {
	delta := 0
	for i := 0; i < len(maskedLine); i++ {
		switch maskedLine[i] {
		case '(', '[', '{':
			delta++
		case ')', ']', '}':
			delta--
		}
	}
	return delta
}

// indentWidthOf measures leading indentation, counting tabs as four columns.
func indentWidthOf(line string) int {
	width := 0
	for _, ch := range line {
		switch ch {
		case ' ':
			width++
		case '\t':
			width += 4
		default:
			return width
		}
	}
	return width
}
