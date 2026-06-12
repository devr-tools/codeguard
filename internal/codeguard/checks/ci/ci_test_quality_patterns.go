package ci

import (
	"regexp"
	"strings"
)

// assertionLine classifies one assertion found inside a test block.
type assertionLine struct {
	line        int  // 1-based file line
	constant    bool // assertion only compares literal constants
	conditional bool // assertion sits inside a conditional block
	idiomatic   bool // idiomatic failure call (Go t.Error/t.Fatal style), exempt from the conditional rule
}

// testQualityPatterns holds the per-language regexes used to recognize
// assertions inside test blocks.
type testQualityPatterns struct {
	assertion  *regexp.Regexp
	idiomatic  *regexp.Regexp // may be nil
	constant   *regexp.Regexp
	braceBased bool
}

// literalPattern matches a constant literal argument.
const literalPattern = `(?:true|false|True|False|None|null|undefined|-?\d+(?:\.\d+)?|'[^']*'|"[^"]*")`

var (
	goAssertionPattern = regexp.MustCompile(`\bt\.(?:Error|Errorf|Fatal|Fatalf|Fail|FailNow)\b|\b(?:assert|require)\.\w+\s*\(`)
	goIdiomaticPattern = regexp.MustCompile(`\bt\.(?:Error|Errorf|Fatal|Fatalf|Fail|FailNow)\b`)
	goConstantPattern  = regexp.MustCompile(
		`\b(?:assert|require)\.(?:True|False)\(\s*t\s*,\s*(?:true|false)\s*[,)]` +
			`|\b(?:assert|require)\.(?:Equal|EqualValues|Exactly|NotEqual)\(\s*t\s*,\s*` + literalPattern + `\s*,\s*` + literalPattern + `\s*[,)]`)

	pythonAssertionPattern = regexp.MustCompile(`^\s*assert\b|\bself\.assert\w+\s*\(|\bpytest\.raises\s*\(|\b(?:self|pytest)\.fail\s*\(`)
	pythonIdiomaticPattern = regexp.MustCompile(`\b(?:self|pytest)\.fail\s*\(`)
	pythonConstantPattern  = regexp.MustCompile(
		`^\s*assert\s+(?:True|1)\s*(?:,.*)?$` +
			`|^\s*assert\s+` + literalPattern + `\s*==\s*` + literalPattern + `\s*(?:,.*)?$` +
			`|\bself\.assertTrue\(\s*True\s*[,)]` +
			`|\bself\.assertEqual\(\s*` + literalPattern + `\s*,\s*` + literalPattern + `\s*[,)]`)

	jsAssertionPattern = regexp.MustCompile(`\bexpect\s*\(|\bassert\s*\(|\bassert\.\w+\s*\(|\.should\b`)
	jsConstantPattern  = regexp.MustCompile(
		`\bexpect\s*\(\s*` + literalPattern + `\s*\)\s*\.\s*(?:not\s*\.\s*)?(?:toBe|toEqual|toStrictEqual)\s*\(\s*` + literalPattern + `\s*\)` +
			`|\bexpect\s*\(\s*(?:true|1|` + literalPattern + `\s*===?\s*` + literalPattern + `)\s*\)\s*\.\s*(?:toBeTruthy|toBeDefined|toBeFalsy)\s*\(\s*\)` +
			`|\bassert\s*\(\s*(?:true|1)\s*\)`)

	braceConditionalOpener  = regexp.MustCompile(`^\s*\}?\s*(?:else\s+)?if\b`)
	pythonConditionalOpener = regexp.MustCompile(`^\s*if\b.*:`)

	// conventionalAssertionPattern credits calls to identifiers named with the
	// assert/require/expect/verify/must/check prefixes. By convention such
	// helpers fail the test internally, so a call to one is a real assertion
	// even though the per-language patterns cannot see inside the helper.
	conventionalAssertionPattern = regexp.MustCompile(`(?i)\b(?:assert|require|expect|verify|must|check)\w*\s*\(`)
)

func testQualityPatternsFor(language string) (testQualityPatterns, bool) {
	switch language {
	case "", "go":
		return testQualityPatterns{assertion: goAssertionPattern, idiomatic: goIdiomaticPattern, constant: goConstantPattern, braceBased: true}, true
	case "python", "py":
		return testQualityPatterns{assertion: pythonAssertionPattern, idiomatic: pythonIdiomaticPattern, constant: pythonConstantPattern}, true
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		return testQualityPatterns{assertion: jsAssertionPattern, constant: jsConstantPattern, braceBased: true}, true
	default:
		return testQualityPatterns{}, false
	}
}

func assertionHelperPattern(helpers []string) *regexp.Regexp {
	names := make([]string, 0, len(helpers))
	for _, helper := range helpers {
		helper = strings.TrimSpace(helper)
		if helper != "" {
			names = append(names, regexp.QuoteMeta(helper))
		}
	}
	if len(names) == 0 {
		return nil
	}
	return regexp.MustCompile(`\b(?:` + strings.Join(names, "|") + `)\s*\(`)
}

func assertionLinesForBlock(patterns testQualityPatterns, helpers *regexp.Regexp, block testBlock) []assertionLine {
	conditional := conditionalLines(patterns, block.lines)
	asserts := make([]assertionLine, 0)
	for idx, line := range block.lines {
		isHelper := helpers != nil && helpers.MatchString(line)
		if !isHelper && !patterns.assertion.MatchString(line) {
			if !conventionalAssertionPattern.MatchString(line) {
				continue
			}
			isHelper = true
		}
		asserts = append(asserts, assertionLine{
			line:        block.startLine + idx,
			constant:    !isHelper && patterns.constant.MatchString(line),
			conditional: conditional[idx],
			idiomatic:   !isHelper && patterns.idiomatic != nil && patterns.idiomatic.MatchString(line),
		})
	}
	return asserts
}

func conditionalLines(patterns testQualityPatterns, lines []string) []bool {
	if patterns.braceBased {
		return braceConditionalLines(lines)
	}
	return pythonConditionalLines(lines)
}

// braceConditionalLines marks the lines of a brace-delimited block that sit
// inside an if-block, tracking brace depth line by line.
func braceConditionalLines(lines []string) []bool {
	marks := make([]bool, len(lines))
	depth := 0
	openers := make([]int, 0)
	for idx, line := range lines {
		if braceConditionalOpener.MatchString(line) {
			openers = append(openers, depth)
		}
		marks[idx] = len(openers) > 0
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		for len(openers) > 0 && depth <= openers[len(openers)-1] {
			openers = openers[:len(openers)-1]
		}
	}
	return marks
}

// pythonConditionalLines marks lines that have an if statement anywhere in
// their chain of enclosing indentation blocks.
func pythonConditionalLines(lines []string) []bool {
	marks := make([]bool, len(lines))
	for idx, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		enclosing := indentWidth(line)
		for prev := idx - 1; prev >= 0 && enclosing > 0; prev-- {
			if strings.TrimSpace(lines[prev]) == "" {
				continue
			}
			prevIndent := indentWidth(lines[prev])
			if prevIndent >= enclosing {
				continue
			}
			if pythonConditionalOpener.MatchString(lines[prev]) {
				marks[idx] = true
				break
			}
			enclosing = prevIndent
		}
	}
	return marks
}
