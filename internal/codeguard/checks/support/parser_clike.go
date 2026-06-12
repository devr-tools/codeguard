package support

import (
	"sort"
	"strings"
)

// ParseCLike builds a lightweight AST for TS/JS, Java, or Rust source.
func ParseCLike(source string, lang CLikeLanguage) *ParsedFile {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	masked := MaskCLikeSource(source, lang)
	file := &ParsedFile{
		Language: string(lang),
		Source:   source,
		Masked:   masked,
		Module:   &ParsedFunction{Name: "<module>", StartLine: 1},
	}
	file.Imports = clikeImports(source, masked, lang)
	spans := clikeFunctionSpans(masked, lang)
	file.Functions = buildCLikeFunctions(file, spans, lang)
	file.Module.EndLine = LineNumberForOffset(source, len(source))
	return file
}

type clikeSpan struct {
	name       string
	start      int
	paramsOpen int
	bodyOpen   int
	bodyEnd    int
}

// buildCLikeFunctions converts offset spans into nested ParsedFunctions.
func buildCLikeFunctions(file *ParsedFile, spans []clikeSpan, lang CLikeLanguage) []*ParsedFunction {
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	ends := make([]int, 0, len(spans))
	parents := make([]*ParsedFunction, 0, len(spans))
	top := make([]*ParsedFunction, 0, len(spans))
	for _, span := range spans {
		fn := newCLikeFunction(file, span, lang)
		for len(ends) > 0 && span.start >= ends[len(ends)-1] {
			ends = ends[:len(ends)-1]
			parents = parents[:len(parents)-1]
		}
		if len(parents) > 0 {
			parent := parents[len(parents)-1]
			parent.Nested = append(parent.Nested, fn)
		} else {
			top = append(top, fn)
		}
		ends = append(ends, span.bodyEnd)
		parents = append(parents, fn)
	}
	return top
}

func newCLikeFunction(file *ParsedFile, span clikeSpan, lang CLikeLanguage) *ParsedFunction {
	paramsClose := matchBracketOffset(file.Masked, span.paramsOpen)
	paramText := ""
	if paramsClose > span.paramsOpen {
		paramText = file.Masked[span.paramsOpen+1 : paramsClose]
	}
	fn := &ParsedFunction{
		Name:      span.name,
		StartLine: LineNumberForOffset(file.Source, span.start),
		EndLine:   LineNumberForOffset(file.Source, span.bodyEnd),
		Signature: strings.TrimSpace(squashWhitespace(paramText)),
		Params:    parseCLikeParams(paramText, lang),
	}
	if span.bodyOpen >= 0 && span.bodyEnd > span.bodyOpen {
		populateCLikeBody(file, fn, span, lang)
	}
	return fn
}

func populateCLikeBody(file *ParsedFile, fn *ParsedFunction, span clikeSpan, lang CLikeLanguage) {
	bodyMasked := file.Masked[span.bodyOpen+1 : span.bodyEnd]
	bodyRaw := file.Source[span.bodyOpen+1 : span.bodyEnd]
	startLine := LineNumberForOffset(file.Source, span.bodyOpen+1)
	maskedLines := strings.Split(bodyMasked, "\n")
	rawLines := strings.Split(bodyRaw, "\n")
	for idx, masked := range maskedLines {
		if strings.TrimSpace(masked) == "" {
			continue
		}
		statement := ParsedStatement{
			Line:   startLine + idx,
			Indent: indentWidthOf(masked),
			Text:   masked,
			Raw:    rawLines[idx],
		}
		fn.Statements = append(fn.Statements, statement)
		fn.Assignments = append(fn.Assignments, clikeAssignments(statement, lang)...)
		fn.Calls = append(fn.Calls, clikeCalls(masked, statement.Line)...)
	}
}

// matchBracketOffset returns the offset of the bracket closing the one at
// open, or -1 when unbalanced.
func matchBracketOffset(masked string, open int) int {
	if open < 0 || open >= len(masked) {
		return -1
	}
	depth := 0
	for i := open; i < len(masked); i++ {
		switch masked[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// findBodyOpen scans forward from offset for the function body's opening
// brace, giving up at a top-level semicolon or arrow-less boundary.
func findBodyOpen(masked string, offset int, stopOnArrow bool) int {
	depth := 0
	for i := offset; i < len(masked); i++ {
		switch masked[i] {
		case '{':
			if depth == 0 {
				return i
			}
			depth++
		case '(', '[':
			depth++
		case ')', ']', '}':
			depth--
		case ';':
			if depth <= 0 {
				return -1
			}
		case '=':
			if stopOnArrow && depth == 0 && i+1 < len(masked) && masked[i+1] == '>' {
				return -1
			}
		}
	}
	return -1
}

func squashWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
