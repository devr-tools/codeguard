package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var securityTaintCatalog = map[string]core.RuleMetadata{
	"security.taint.go": {
		ID:               "security.taint.go",
		Section:          "Security",
		DefaultLevel:     "fail",
		ExecutionModel:   core.RuleExecutionModelGoNative,
		LanguageCoverage: core.FixedRuleLanguageCoverage(core.RuleLanguageGo),
		Title:            "Go taint flow",
		Description:      "Fails when untrusted input (HTTP request data, environment, arguments, stdin) flows into a dangerous sink such as exec.Command, SQL query text, file paths, or template parsing. The finding message includes the source-to-sink chain.",
		HowToFix:         "Validate or sanitize the value before the sink: use parameterized queries, strconv parsing, allow-lists for commands and paths, or static templates.",
	},
	"security.taint.python": {
		ID:               "security.taint.python",
		Section:          "Security",
		DefaultLevel:     "fail",
		ExecutionModel:   core.RuleExecutionModelGoNative,
		LanguageCoverage: core.FixedRuleLanguageCoverage(core.RuleLanguagePython),
		Title:            "Python taint flow",
		Description:      "Fails when untrusted input (input(), os.environ, sys.argv, web request attributes) flows into a dangerous sink such as os.system, subprocess with a shell or string command, eval/exec, or SQL execute with interpolated query text. The finding message includes the source-to-sink chain.",
		HowToFix:         "Sanitize the value before the sink: use shlex.quote for shell arguments, parameterized cursor.execute arguments, or int/float parsing for numeric input.",
	},
}
