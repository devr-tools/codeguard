package ci

import "regexp"

type testQualitySpec struct {
	testDefinitionPatterns []*regexp.Regexp
	assertionTokens        []string
	assertionPatterns      []*regexp.Regexp
	alwaysTruePatterns     []*regexp.Regexp
}

var testQualitySpecs = map[string]testQualitySpec{
	"go": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*func\s+Test[[:word:]]*\s*\(`),
		},
		assertionTokens: []string{
			"t.Fatal(", "t.Fatalf(", "t.Error(", "t.Errorf(", "t.Fail(", "t.FailNow(",
			"assert.", "require.", "cmp.Diff(", "panic(",
		},
		assertionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`\bassert[A-Z]\w*\s*\(`),
			regexp.MustCompile(`\brequire[A-Z]\w*\s*\(`),
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`\b(?:assert|require)\.True\s*\(\s*t\s*,\s*true\s*(?:,|\))`),
			regexp.MustCompile(`\b(?:assert|require)\.(?:Equal|Exactly)\s*\(\s*t\s*,\s*true\s*,\s*true\s*(?:,|\))`),
		},
	},
	"python": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*def\s+test_[[:word:]]*\s*\(`),
			regexp.MustCompile(`(?m)^\s*class\s+Test[[:word:]]*[\(:]`),
		},
		assertionTokens: []string{
			"assert ", "self.assert", "pytest.fail(", "pytest.raises(", "raise AssertionError",
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`^\s*assert\s+True\b`),
			regexp.MustCompile(`\bself\.assertTrue\s*\(\s*True\s*\)`),
		},
	},
	"typescript": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`\b(?:it|test)\s*\(`),
		},
		assertionTokens: []string{
			"expect(", "assert.", "assert(", "should.", ".should(", "toThrow(",
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`\bexpect\s*\(\s*true\s*\)\s*\.(?:toBe|toEqual|toStrictEqual)\s*\(\s*true\s*\)`),
			regexp.MustCompile(`\bassert(?:\.ok)?\s*\(\s*true\s*(?:,|\))`),
		},
	},
	"javascript": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`\b(?:it|test)\s*\(`),
		},
		assertionTokens: []string{
			"expect(", "assert.", "assert(", "should.", ".should(", "toThrow(",
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`\bexpect\s*\(\s*true\s*\)\s*\.(?:toBe|toEqual|toStrictEqual)\s*\(\s*true\s*\)`),
			regexp.MustCompile(`\bassert(?:\.ok)?\s*\(\s*true\s*(?:,|\))`),
		},
	},
	"rust": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*#\s*\[\s*test\s*\]`),
		},
		assertionTokens: []string{
			"assert!(", "assert_eq!(", "assert_ne!(", "panic!(",
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`\bassert!\s*\(\s*true\s*\)`),
			regexp.MustCompile(`\bassert_eq!\s*\(\s*true\s*,\s*true\s*\)`),
		},
	},
	"java": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*@Test\b`),
			regexp.MustCompile(`(?m)^\s*public\s+void\s+test[[:word:]]*\s*\(`),
		},
		assertionTokens: []string{
			"assert", "Assertions.", "assertThat(", "fail(",
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`\bassertTrue\s*\(\s*true\s*\)`),
			regexp.MustCompile(`\bAssertions\.assertTrue\s*\(\s*true\s*\)`),
		},
	},
	"csharp": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*\[\s*(?:Fact|Theory|Test)\s*\]`),
		},
		assertionTokens: []string{
			"Assert.", "Should().", "FluentActions.",
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`\bAssert\.True\s*\(\s*true\s*\)`),
		},
	},
	"ruby": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*def\s+test_[[:word:]]*[!?=]?\s*$`),
			regexp.MustCompile(`\b(?:it|specify|test)\s+["']`),
		},
		assertionTokens: []string{
			"assert", "refute", "expect(", "raise_error",
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`\bassert\s*\(?\s*true\s*\)?`),
			regexp.MustCompile(`\bexpect\s*\(\s*true\s*\)\.to\s+eq\s*\(\s*true\s*\)`),
		},
	},
}
