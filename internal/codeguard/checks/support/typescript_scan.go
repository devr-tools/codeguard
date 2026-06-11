package support

type typeScriptStripper struct {
	out   []byte
	state string
}

func StripTypeScriptCommentsAndStrings(source string) string {
	scanner := typeScriptStripper{
		out:   []byte(source),
		state: "code",
	}
	for idx := 0; idx < len(scanner.out); idx++ {
		idx = scanner.step(idx)
	}
	return string(scanner.out)
}

func (scanner *typeScriptStripper) step(idx int) int {
	switch scanner.state {
	case "line-comment":
		return scanner.handleLineComment(idx)
	case "block-comment":
		return scanner.handleBlockComment(idx)
	case "'":
		return scanner.handleQuoted(idx, '\'')
	case `"`:
		return scanner.handleQuoted(idx, '"')
	case "template":
		return scanner.handleTemplate(idx)
	default:
		return scanner.handleCode(idx)
	}
}

func (scanner *typeScriptStripper) handleCode(idx int) int {
	if idx+1 < len(scanner.out) && scanner.out[idx] == '/' && scanner.out[idx+1] == '/' {
		scanner.out[idx], scanner.out[idx+1] = ' ', ' '
		scanner.state = "line-comment"
		return idx + 1
	}
	if idx+1 < len(scanner.out) && scanner.out[idx] == '/' && scanner.out[idx+1] == '*' {
		scanner.out[idx], scanner.out[idx+1] = ' ', ' '
		scanner.state = "block-comment"
		return idx + 1
	}

	switch scanner.out[idx] {
	case '\'', '"':
		quote := scanner.out[idx]
		scanner.out[idx] = ' '
		scanner.state = string(quote)
	case '`':
		scanner.out[idx] = ' '
		scanner.state = "template"
	}
	return idx
}

func (scanner *typeScriptStripper) handleLineComment(idx int) int {
	if scanner.out[idx] == '\n' {
		scanner.state = "code"
		return idx
	}
	scanner.out[idx] = ' '
	return idx
}

func (scanner *typeScriptStripper) handleBlockComment(idx int) int {
	if idx+1 < len(scanner.out) && scanner.out[idx] == '*' && scanner.out[idx+1] == '/' {
		scanner.out[idx], scanner.out[idx+1] = ' ', ' '
		scanner.state = "code"
		return idx + 1
	}
	if scanner.out[idx] != '\n' {
		scanner.out[idx] = ' '
	}
	return idx
}

func (scanner *typeScriptStripper) handleQuoted(idx int, quote byte) int {
	if scanner.out[idx] == '\\' && idx+1 < len(scanner.out) {
		scanner.out[idx], scanner.out[idx+1] = ' ', ' '
		return idx + 1
	}
	if scanner.out[idx] == '\n' {
		scanner.state = "code"
		return idx
	}
	if scanner.out[idx] == quote {
		scanner.state = "code"
	}
	scanner.out[idx] = ' '
	return idx
}

func (scanner *typeScriptStripper) handleTemplate(idx int) int {
	if scanner.out[idx] == '\\' && idx+1 < len(scanner.out) {
		if scanner.out[idx] != '\n' {
			scanner.out[idx] = ' '
		}
		if scanner.out[idx+1] != '\n' {
			scanner.out[idx+1] = ' '
		}
		return idx + 1
	}
	if scanner.out[idx] == '`' {
		scanner.out[idx] = ' '
		scanner.state = "code"
		return idx
	}
	if scanner.out[idx] != '\n' {
		scanner.out[idx] = ' '
	}
	return idx
}
