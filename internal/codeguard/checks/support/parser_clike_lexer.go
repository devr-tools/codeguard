package support

// CLikeLanguage selects lexing rules for the brace-delimited language family.
type CLikeLanguage string

const (
	CLikeTypeScript CLikeLanguage = "typescript"
	CLikeJava       CLikeLanguage = "java"
	CLikeRust       CLikeLanguage = "rust"
	CLikeGo         CLikeLanguage = "go"
)

// MaskCLikeSource blanks comments and string literal contents for TS/JS,
// Java, Rust, and Go while preserving byte offsets and newlines exactly.
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
	switch {
	case m.matches("//"):
		m.maskUntilNewline()
	case m.matches("/*"):
		m.maskBlockComment()
	case m.lang == CLikeJava && m.matches(`"""`):
		m.maskJavaTextBlock()
	case m.src[m.idx] == '"':
		m.maskQuoted('"', m.lang == CLikeRust)
	case m.src[m.idx] == '\'':
		m.handleSingleQuote()
	case m.lang == CLikeTypeScript && m.src[m.idx] == '`':
		m.maskTemplate()
	case m.lang == CLikeGo && m.src[m.idx] == '`':
		m.maskGoRawString()
	case m.lang == CLikeRust && m.rustRawStringAhead():
		m.maskRustRawString()
	default:
		m.idx++
	}
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
