package support

import "strings"

// MaskPythonSource blanks comment text and string literal contents while
// preserving byte offsets and line breaks exactly. Interpolated expressions
// inside f-strings are kept so dataflow analysis can see identifiers.
func MaskPythonSource(source string) string {
	masker := &pythonMasker{sourceMasker: newSourceMasker(source)}
	for masker.idx < len(masker.src) {
		masker.step()
	}
	return string(masker.out)
}

type pythonMasker struct {
	sourceMasker
}

func (m *pythonMasker) step() {
	switch m.src[m.idx] {
	case '#':
		m.maskUntilNewline()
	case '\'', '"':
		m.maskString()
	default:
		m.idx++
	}
}

type pythonStringSpec struct {
	quote  byte
	triple bool
	raw    bool
	fstr   bool
}

func (m *pythonMasker) maskString() {
	spec := pythonStringSpec{quote: m.src[m.idx]}
	spec.raw, spec.fstr = m.stringPrefixFlags()
	if m.idx+2 < len(m.src) && m.src[m.idx+1] == spec.quote && m.src[m.idx+2] == spec.quote {
		spec.triple = true
	}
	delim := 1
	if spec.triple {
		delim = 3
	}
	for i := 0; i < delim; i++ {
		m.out[m.idx] = ' '
		m.idx++
	}
	m.maskStringBody(spec)
}

// stringPrefixFlags inspects identifier letters immediately before the quote
// (r, b, u, f in any case or combination) without consuming them.
func (m *pythonMasker) stringPrefixFlags() (raw bool, fstr bool) {
	start := m.idx
	for start > 0 && isPythonPrefixLetter(m.src[start-1]) {
		start--
	}
	if start > 0 && isIdentByte(m.src[start-1]) {
		return false, false
	}
	if m.idx-start > 3 {
		return false, false
	}
	prefix := strings.ToLower(m.src[start:m.idx])
	return strings.Contains(prefix, "r"), strings.Contains(prefix, "f")
}

func (m *pythonMasker) maskStringBody(spec pythonStringSpec) {
	for m.idx < len(m.src) {
		if m.stringEndsHere(spec) {
			return
		}
		ch := m.src[m.idx]
		switch {
		case ch == '\n':
			if !spec.triple {
				return
			}
			m.idx++
		case ch == '\\' && !spec.raw:
			m.maskBytes(2)
		case spec.fstr && ch == '{':
			m.handleFStringBrace()
		default:
			m.maskBytes(1)
		}
	}
}

func (m *pythonMasker) stringEndsHere(spec pythonStringSpec) bool {
	if m.src[m.idx] != spec.quote {
		return false
	}
	if !spec.triple {
		m.maskBytes(1)
		return true
	}
	if m.idx+2 < len(m.src) && m.src[m.idx+1] == spec.quote && m.src[m.idx+2] == spec.quote {
		m.maskBytes(3)
		return true
	}
	m.maskBytes(1)
	return false
}
