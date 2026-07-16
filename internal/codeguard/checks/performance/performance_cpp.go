package performance

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	cppLoopStartPattern     = regexp.MustCompile(`(?:^|[^\w])(?:for|while)\b`)
	cppRegexDeclPattern     = regexp.MustCompile(`\bstd::(?:(?:w|u8|u16|u32)?regex|basic_regex\s*<[^>]+>)\s+[A-Za-z_]\w*\s*[\({]`)
	cppRegexLiteralCtor     = regexp.MustCompile(`[\({][ \t]*(?:u8|u|U|L)?(?:R"|")`)
	cppStringDeclPattern    = regexp.MustCompile(`^\s*(?:constexpr\s+|static\s+|inline\s+|const\s+|volatile\s+|mutable\s+)*(?:std::)?(?:basic_string\s*<[^>]+>|string|wstring|u8string|u16string|u32string)\s+([A-Za-z_]\w*)\b`)
	cppStringReservePattern = regexp.MustCompile(`\b([A-Za-z_]\w*)\.reserve\s*\(`)
	cppStringConcatPattern  = regexp.MustCompile(`^\s*([A-Za-z_]\w*)\s*\+=`)
	cppStringAppendPattern  = regexp.MustCompile(`\b([A-Za-z_]\w*)\.(?:append|push_back)\s*\(`)
	cppThreadSleepPattern   = regexp.MustCompile(`\bstd::this_thread::sleep_(?:for|until)\s*\(`)
)

type cppStringState struct {
	reserved bool
}

type cppPerformanceScan struct {
	env       support.Context
	file      string
	rules     core.PerformanceRulesConfig
	depth     int
	loops     []int
	stringVar map[string]cppStringState
	findings  []core.Finding
}

func cppPerformanceFindings(env support.Context, file string, data []byte) []core.Finding {
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	masked := support.MaskCLikeSource(source, support.CLikeCPP)
	scan := &cppPerformanceScan{
		env:       env,
		file:      file,
		rules:     env.Config.Checks.PerformanceRules,
		stringVar: make(map[string]cppStringState),
	}
	rawLines := strings.Split(source, "\n")
	for idx, line := range strings.Split(masked, "\n") {
		scan.consumeLine(idx+1, line, rawLines[idx])
	}
	return scan.findings
}

func (s *cppPerformanceScan) consumeLine(lineNo int, line string, rawLine string) {
	if m := cppStringDeclPattern.FindStringSubmatch(rawLine); m != nil {
		state := s.stringVar[m[1]]
		s.stringVar[m[1]] = state
	}
	if m := cppStringReservePattern.FindStringSubmatch(line); m != nil {
		state := s.stringVar[m[1]]
		state.reserved = true
		s.stringVar[m[1]] = state
	}
	startsLoop := cppLoopStartPattern.MatchString(line)
	s.checkLine(lineNo, line, rawLine, len(s.loops) > 0 || startsLoop)
	s.depth, s.loops = consumeBraceLoopLine(s.depth, s.loops, line, startsLoop)
}

func (s *cppPerformanceScan) checkLine(lineNo int, line string, rawLine string, inLoop bool) {
	if !inLoop {
		return
	}
	if toggleEnabled(s.rules.DetectRegexCompileInLoop) && cppRegexDeclPattern.MatchString(line) && cppRegexLiteralCtor.MatchString(rawLine) {
		s.addFinding("performance.regex-compile-in-loop", lineNo,
			"std::regex constructed inside a loop recompiles the pattern every iteration; hoist it out of the loop and reuse it")
	}
	if toggleEnabled(s.rules.DetectAllocInLoop) && s.isStringGrowth(line) {
		s.addFinding("performance.cpp.alloc-in-loop", lineNo,
			"std::string grown inside a loop without visible reserve(); reserve once before the loop or collect fragments and join once")
	}
	if toggleEnabled(s.rules.DetectSleepInLoop) && cppThreadSleepPattern.MatchString(line) {
		s.addFinding("performance.cpp.sleep-in-loop", lineNo,
			"std::this_thread::sleep_* inside a loop usually marks a poll; prefer a condition variable, timer primitive, or bounded backoff helper")
	}
}

func (s *cppPerformanceScan) isStringGrowth(line string) bool {
	if m := cppStringConcatPattern.FindStringSubmatch(line); m != nil {
		state, ok := s.stringVar[m[1]]
		return ok && !state.reserved
	}
	if m := cppStringAppendPattern.FindStringSubmatch(line); m != nil {
		state, ok := s.stringVar[m[1]]
		return ok && !state.reserved
	}
	return false
}

func (s *cppPerformanceScan) addFinding(ruleID string, lineNo int, message string) {
	s.findings = append(s.findings, warnFinding(s.env, ruleID, s.file, lineNo, 1, message))
}
