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
		ID:           "security.taint.python",
		Section:      "Security",
		DefaultLevel: "fail",
		// language-agnostic: the Python taint engine runs on codeguard's
		// hand-rolled Python parser (checks/support), not on Go-specific source
		// structure or Go-only integrations.
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(core.RuleLanguagePython),
		Title:            "Python taint flow",
		Description:      "Fails when untrusted input (input(), os.environ, sys.argv, web request attributes) flows into a dangerous sink such as os.system, subprocess with a shell or string command, eval/exec, or SQL execute with interpolated query text. The finding message includes the source-to-sink chain.",
		HowToFix:         "Sanitize the value before the sink: use shlex.quote for shell arguments, parameterized cursor.execute arguments, or int/float parsing for numeric input.",
	},
	"security.ssrf.go": {
		ID:               "security.ssrf.go",
		Section:          "Security",
		DefaultLevel:     "fail",
		ExecutionModel:   core.RuleExecutionModelGoNative,
		LanguageCoverage: core.FixedRuleLanguageCoverage(core.RuleLanguageGo),
		Title:            "Go server-side request forgery",
		Description:      "Fails when untrusted input flows into the URL of an outbound HTTP request (http.Get/Post/Head/PostForm/NewRequest), letting an attacker make the server reach arbitrary or internal hosts. The finding message includes the source-to-sink chain.",
		HowToFix:         "Validate the destination against an allowlist of trusted hosts and block private/link-local addresses before issuing the request.",
	},
	"security.ssrf.python": {
		ID:           "security.ssrf.python",
		Section:      "Security",
		DefaultLevel: "fail",
		// language-agnostic: same hand-rolled Python parser as
		// security.taint.python.
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(core.RuleLanguagePython),
		Title:            "Python server-side request forgery",
		Description:      "Fails when untrusted input flows into the URL of an outbound HTTP request (requests.get/post/etc., urllib urlopen), letting an attacker make the server reach arbitrary or internal hosts. The finding message includes the source-to-sink chain.",
		HowToFix:         "Validate the destination against an allowlist of trusted hosts and block private/link-local addresses before issuing the request.",
	},
}
