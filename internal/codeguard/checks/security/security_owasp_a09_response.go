package security

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	// Go: http.Error(w, err.Error(), ...) and fmt.Fprint*(w, ..., err) where
	// the first argument is a conventionally named response writer.
	goHTTPErrorRawPattern = regexp.MustCompile(`\bhttp\.Error\s*\(\s*[A-Za-z_]\w*\s*,\s*[A-Za-z_][\w.]*\.Error\s*\(\s*\)`)
	goFprintRawErrPattern = regexp.MustCompile(`\bfmt\.Fprint(?:f|ln)?\s*\(\s*(?:w|wr|rw|res|resp|rsp|writer)\s*,[^)]*\berr\b`)

	// TS/JS: res/resp/response .send/.json/.end with an error-named identifier
	// (optionally via .status(...) chaining, String(...), or .stack/.message).
	scriptRawErrResponsePattern = regexp.MustCompile(`\b(?:res|resp|response)\s*(?:\.\s*status\s*\([^)]*\)\s*)?\.\s*(?:send|json|end)\s*\(\s*(?:String\s*\(\s*)?(?:err|error|e|ex)\b`)

	pythonExceptPattern      = regexp.MustCompile(`^[ \t]*except\b([^:]*):`)
	pythonExceptAliasPattern = regexp.MustCompile(`\bas\s+([A-Za-z_]\w*)`)
)

const unsanitizedErrorResponseMessage = "raw error value written to the HTTP response; return a generic message and log the detail server-side"

// unsanitizedErrorResponseFinding flags single-line patterns that write a raw
// error value directly into an HTTP response. Matching runs on the masked
// line, so error words inside string literals or comments cannot fire.
func unsanitizedErrorResponseFinding(env support.Context, line a09Line, excepts *pythonExceptTracker) *core.Finding {
	switch line.language {
	case a09LanguageGo:
		if goHTTPErrorRawPattern.MatchString(line.masked) || goFprintRawErrPattern.MatchString(line.masked) {
			return newA09Finding(env, line, "security.unsanitized-error-response", unsanitizedErrorResponseMessage)
		}
	case a09LanguageScript:
		if scriptRawErrResponsePattern.MatchString(line.masked) {
			return newA09Finding(env, line, "security.unsanitized-error-response", unsanitizedErrorResponseMessage)
		}
	case a09LanguagePython:
		return pythonUnsanitizedErrorFinding(env, line, excepts)
	}
	return nil
}

// pythonUnsanitizedErrorFinding flags `return str(<exc>)` and
// `HttpResponse(str(<exc>))` on lines inside an except block, where <exc> is
// the block's `as` alias (or a conventional exception name when the block has
// none). Lines outside except blocks never fire.
func pythonUnsanitizedErrorFinding(env support.Context, line a09Line, excepts *pythonExceptTracker) *core.Finding {
	alias, inside := excepts.observe(line.masked)
	if !inside {
		return nil
	}
	if !pythonRawExceptionResponsePattern(alias).MatchString(line.masked) {
		return nil
	}
	return newA09Finding(env, line, "security.unsanitized-error-response", unsanitizedErrorResponseMessage)
}

// pythonRawExceptionResponsePattern matches `return str(<name>)` and
// `HttpResponse(str(<name>))` for the except alias; blocks without an alias
// accept the conventional exception variable names.
func pythonRawExceptionResponsePattern(alias string) *regexp.Regexp {
	name := `(?:e|err|ex|exc|exception|error)`
	if alias != "" {
		name = regexp.QuoteMeta(alias)
	}
	return compileDynamicPattern(`\b(?:return\s+str\s*\(\s*` + name + `\s*\)|HttpResponse\s*\(\s*str\s*\(\s*` + name + `\s*\))`)
}

// pythonExceptFrame records one open except block: its indentation and its
// `as` alias ("" when the clause binds no name).
type pythonExceptFrame struct {
	indent int
	alias  string
}

// pythonExceptTracker tracks which except blocks are open while scanning a
// Python file top to bottom, using indentation to detect block exits.
type pythonExceptTracker struct {
	frames []pythonExceptFrame
}

// observe consumes one masked line and reports whether the line is inside an
// except block (including the except line itself, for one-line handlers) and
// the alias of the innermost block. Blank lines keep the current block open.
func (t *pythonExceptTracker) observe(masked string) (alias string, inside bool) {
	trimmed := strings.TrimLeft(masked, " \t")
	if trimmed == "" {
		return t.innermost()
	}
	indent := len(masked) - len(trimmed)
	for len(t.frames) > 0 && indent <= t.frames[len(t.frames)-1].indent {
		t.frames = t.frames[:len(t.frames)-1]
	}
	if match := pythonExceptPattern.FindStringSubmatch(masked); match != nil {
		frame := pythonExceptFrame{indent: indent}
		if aliasMatch := pythonExceptAliasPattern.FindStringSubmatch(match[1]); aliasMatch != nil {
			frame.alias = aliasMatch[1]
		}
		t.frames = append(t.frames, frame)
		return frame.alias, true
	}
	return t.innermost()
}

func (t *pythonExceptTracker) innermost() (alias string, inside bool) {
	if len(t.frames) == 0 {
		return "", false
	}
	return t.frames[len(t.frames)-1].alias, true
}
