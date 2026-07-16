package support

// CLikeLanguage selects lexing rules for the brace-delimited language family.
type CLikeLanguage string

const (
	CLikeTypeScript CLikeLanguage = "typescript"
	CLikeJava       CLikeLanguage = "java"
	CLikeRust       CLikeLanguage = "rust"
	CLikeCPP        CLikeLanguage = "cpp"
	CLikeGo         CLikeLanguage = "go"
)

// MaskCLikeSource blanks comments and string literal contents for TS/JS,
// Java, Rust, C++, and Go while preserving byte offsets and newlines exactly.
// Template literal interpolations (`${expr}`) stay visible.
func MaskCLikeSource(source string, lang CLikeLanguage) string {
	masker := &clikeMasker{sourceMasker: newSourceMasker(source), lang: lang}
	for masker.idx < len(masker.src) {
		masker.step()
	}
	return string(masker.out)
}

type clikeMasker struct {
	sourceMasker
	lang CLikeLanguage
}

func (m *clikeMasker) step() {
	if m.maskCommentOrString() {
		return
	}
	m.idx++
}

func (m *clikeMasker) maskCommentOrString() bool {
	if m.maskCommentStart() {
		return true
	}
	return m.maskLiteralStart()
}

func (m *clikeMasker) maskCommentStart() bool {
	switch {
	case m.matches("//"):
		m.maskUntilNewline()
	case m.matches("/*"):
		m.maskBlockComment()
	default:
		return false
	}
	return true
}

func (m *clikeMasker) maskLiteralStart() bool {
	switch {
	case m.lang == CLikeJava && m.matches(`"""`):
		m.maskJavaTextBlock()
	case m.src[m.idx] == '"':
		m.maskQuoted('"', m.lang == CLikeRust)
	case m.src[m.idx] == '\'':
		handleSingleQuote(m)
	case m.lang == CLikeTypeScript && m.src[m.idx] == '`':
		m.maskTemplate()
	case m.lang == CLikeGo && m.src[m.idx] == '`':
		maskGoRawString(m)
	case m.lang == CLikeCPP && m.cppRawStringAhead():
		m.maskCPPRawString()
	case m.lang == CLikeRust && m.rustRawStringAhead():
		m.maskRustRawString()
	default:
		return false
	}
	return true
}

func (m *clikeMasker) maskBlockComment() {
	depth := 0
	for m.idx < len(m.src) {
		switch {
		case m.matches("/*"):
			depth++
			m.maskBytes(2)
		case m.matches("*/"):
			depth--
			m.maskBytes(2)
			if depth == 0 {
				return
			}
		default:
			m.maskBytes(1)
		}
		if m.lang != CLikeRust && depth > 1 {
			depth = 1
		}
	}
}

func (m *clikeMasker) maskJavaTextBlock() {
	m.maskBytes(3)
	for m.idx < len(m.src) {
		if m.matches(`"""`) {
			m.maskBytes(3)
			return
		}
		m.maskBytes(1)
	}
}
