package support

// handleFStringBrace keeps `{expr}` interpolations visible; `{{` stays masked.
func (m *pythonMasker) handleFStringBrace() {
	if m.idx+1 < len(m.src) && m.src[m.idx+1] == '{' {
		m.maskBytes(2)
		return
	}
	depth := 0
	for m.idx < len(m.src) && m.src[m.idx] != '\n' {
		ch := m.src[m.idx]
		if ch == '\'' || ch == '"' {
			m.maskNestedQuote(ch)
			continue
		}
		if ch == '{' {
			depth++
		}
		if ch == '}' {
			depth--
		}
		m.idx++
		if depth == 0 {
			return
		}
	}
}

// maskNestedQuote blanks a simple string nested inside an f-string expression.
func (m *pythonMasker) maskNestedQuote(quote byte) {
	m.out[m.idx] = ' '
	m.idx++
	for m.idx < len(m.src) && m.src[m.idx] != quote && m.src[m.idx] != '\n' {
		if m.src[m.idx] == '\\' {
			m.maskBytes(1)
			if m.idx >= len(m.src) {
				return
			}
		}
		m.out[m.idx] = ' '
		m.idx++
	}
	if m.idx < len(m.src) && m.src[m.idx] == quote {
		m.out[m.idx] = ' '
		m.idx++
	}
}
