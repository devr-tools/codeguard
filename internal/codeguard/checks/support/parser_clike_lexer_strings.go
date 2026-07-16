package support

func (m *clikeMasker) maskQuoted(quote byte, allowNewline bool) {
	m.maskBytes(1)
	for m.idx < len(m.src) {
		ch := m.src[m.idx]
		switch {
		case ch == '\\':
			m.maskBytes(2)
		case ch == quote:
			m.maskBytes(1)
			return
		case ch == '\n' && !allowNewline:
			return
		default:
			m.maskBytes(1)
		}
	}
}

// handleSingleQuote distinguishes Rust lifetimes ('a, 'static) from char
// literals; other languages treat single quotes as plain string delimiters.
func handleSingleQuote(m *clikeMasker) {
	if m.lang != CLikeRust {
		m.maskQuoted('\'', false)
		return
	}
	if m.idx+2 < len(m.src) && m.src[m.idx+1] != '\\' && m.src[m.idx+2] != '\'' {
		m.idx++ // lifetime or loop label, leave visible
		return
	}
	m.maskQuoted('\'', false)
}

func (m *clikeMasker) maskTemplate() {
	m.maskBytes(1)
	for m.idx < len(m.src) {
		switch {
		case m.src[m.idx] == '\\':
			m.maskBytes(2)
		case m.src[m.idx] == '`':
			m.maskBytes(1)
			return
		case m.matches("${"):
			m.scanInterpolation()
		default:
			m.maskBytes(1)
		}
	}
}

// scanInterpolation keeps `${expr}` visible while masking nested literals.
func (m *clikeMasker) scanInterpolation() {
	m.idx++ // '$'
	depth := 0
	for m.idx < len(m.src) {
		ch := m.src[m.idx]
		if ch == '"' || ch == '\'' || ch == '`' || m.matches("//") || m.matches("/*") {
			m.step()
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

// maskGoRawString blanks a Go backquoted raw string literal, which has no
// escape sequences and may span multiple lines.
func maskGoRawString(m *clikeMasker) {
	m.maskBytes(1) // opening backquote
	for m.idx < len(m.src) {
		if m.src[m.idx] == '`' {
			m.maskBytes(1)
			return
		}
		m.maskBytes(1)
	}
}

func (m *clikeMasker) rustRawStringAhead() bool {
	if m.src[m.idx] != 'r' && !m.matches("br") {
		return false
	}
	if m.idx > 0 && isIdentByte(m.src[m.idx-1]) {
		return false
	}
	probe := m.idx + 1
	if m.matches("br") {
		probe = m.idx + 2
	}
	for probe < len(m.src) && m.src[probe] == '#' {
		probe++
	}
	return probe < len(m.src) && m.src[probe] == '"'
}

func (m *clikeMasker) maskRustRawString() {
	m.idx++ // 'r'
	if m.idx < len(m.src) && m.src[m.idx] == 'r' {
		m.idx++ // second letter of 'br'
	}
	hashes := 0
	for m.idx < len(m.src) && m.src[m.idx] == '#' {
		hashes++
		m.maskBytes(1)
	}
	m.maskBytes(1) // opening quote
	closing := `"` + repeatHash(hashes)
	for m.idx < len(m.src) {
		if m.matches(closing) {
			m.maskBytes(len(closing))
			return
		}
		m.maskBytes(1)
	}
}

func repeatHash(count int) string {
	out := make([]byte, count)
	for i := range out {
		out[i] = '#'
	}
	return string(out)
}

func (m *clikeMasker) cppRawStringAhead() bool {
	if m.idx+2 >= len(m.src) {
		return false
	}
	switch {
	case m.matches(`R"`):
		return true
	case m.matches(`u8R"`), m.matches(`uR"`), m.matches(`UR"`), m.matches(`LR"`):
		return true
	default:
		return false
	}
}

func (m *clikeMasker) maskCPPRawString() {
	switch {
	case m.matches(`u8R"`):
		m.maskBytes(4)
	case m.matches(`uR"`), m.matches(`UR"`), m.matches(`LR"`):
		m.maskBytes(3)
	default:
		m.maskBytes(2)
	}
	delimStart := m.idx
	for m.idx < len(m.src) && m.src[m.idx] != '(' {
		m.maskBytes(1)
	}
	if m.idx >= len(m.src) {
		return
	}
	delimiter := string(m.src[delimStart:m.idx])
	m.maskBytes(1) // '('
	closing := ")" + delimiter + `"`
	for m.idx < len(m.src) {
		if m.matches(closing) {
			m.maskBytes(len(closing))
			return
		}
		m.maskBytes(1)
	}
}
