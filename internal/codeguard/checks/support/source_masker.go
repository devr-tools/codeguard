package support

// sourceMasker holds the masking state shared by the language lexers: the
// original source, the masked output buffer, and the current byte offset.
type sourceMasker struct {
	src string
	out []byte
	idx int
}

func newSourceMasker(source string) sourceMasker {
	return sourceMasker{src: source, out: []byte(source)}
}

func (m *sourceMasker) matches(needle string) bool {
	return m.idx+len(needle) <= len(m.src) && m.src[m.idx:m.idx+len(needle)] == needle
}

// maskUntilNewline blanks bytes up to (excluding) the next newline.
func (m *sourceMasker) maskUntilNewline() {
	for m.idx < len(m.src) && m.src[m.idx] != '\n' {
		m.out[m.idx] = ' '
		m.idx++
	}
}

// maskBytes blanks up to count bytes, preserving newlines.
func (m *sourceMasker) maskBytes(count int) {
	for i := 0; i < count && m.idx < len(m.src); i++ {
		if m.src[m.idx] != '\n' {
			m.out[m.idx] = ' '
		}
		m.idx++
	}
}
