package performance

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	cppLoopStartPattern       = regexp.MustCompile(`(?:^|[^\w])(?:for|while)\b`)
	cppRegexDeclPattern       = regexp.MustCompile(`\bstd::(?:(?:w|u8|u16|u32)?regex|basic_regex\s*<[^>]+>)\s+[A-Za-z_]\w*\s*[\({]`)
	cppRegexLiteralCtor       = regexp.MustCompile(`[\({][ \t]*(?:u8|u|U|L)?(?:R"|")`)
	cppStringDeclPattern      = regexp.MustCompile(`^\s*(?:constexpr\s+|static\s+|inline\s+|const\s+|volatile\s+|mutable\s+)*(?:std::)?(?:basic_string\s*<[^>]+>|string|wstring|u8string|u16string|u32string)\s+([A-Za-z_]\w*)\b`)
	cppStringReservePattern   = regexp.MustCompile(`\b([A-Za-z_]\w*)\.reserve\s*\(`)
	cppStringConcatPattern    = regexp.MustCompile(`^\s*([A-Za-z_]\w*)\s*\+=`)
	cppStringAppendPattern    = regexp.MustCompile(`\b([A-Za-z_]\w*)\.(?:append|push_back)\s*\(`)
	cppThreadSleepPattern     = regexp.MustCompile(`\bstd::this_thread::sleep_(?:for|until)\s*\(`)
	cppStreamFlushPattern     = regexp.MustCompile(`<<\s*(?:std::)?(?:endl|flush)\b`)
	cppRangeForStructuredCopy = regexp.MustCompile(`\bfor\s*\(\s*(?:const\s+)?auto\s*\[[^\]]+\]\s*:`)
	cppRangeForValueCopy      = regexp.MustCompile(`\bfor\s*\(\s*(?:const\s+)?(?:std::(?:basic_string\s*<[^>]+>|string|wstring|u8string|u16string|u32string|vector\s*<[^>]+>|array\s*<[^>]+>|map\s*<[^>]+>|unordered_map\s*<[^>]+>|set\s*<[^>]+>|unordered_set\s*<[^>]+>|pair\s*<[^>]+>|tuple\s*<[^>]+>)|[A-Za-z_]\w*::[A-Za-z_]\w*(?:\s*<[^>]+>)?)\s+[A-Za-z_]\w*\s*:`)
	cppUnboundedThreadPattern = regexp.MustCompile(`(?:\b[A-Za-z_]\w*\.(?:emplace_back|push_back)\s*\(\s*(?:std::)?(?:jthread|thread|async)\b|\bstd::(?:jthread|thread)\s*\([^;\n]*\)\s*\.detach\s*\()`)
	cppThreadVectorDecl       = regexp.MustCompile(`\bstd::vector\s*<\s*std::(?:jthread|thread)\s*>\s*([A-Za-z_]\w*)`)
	cppContainerAppend        = regexp.MustCompile(`\b([A-Za-z_]\w*)\.(?:emplace_back|push_back)\s*\(`)
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
	threadVec map[string]bool
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
		threadVec: make(map[string]bool),
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
	if m := cppThreadVectorDecl.FindStringSubmatch(line); m != nil {
		s.threadVec[m[1]] = true
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
	if toggleEnabled(s.rules.DetectHotPathPatterns) && s.isRangeForCopy(rawLine) {
		s.addFinding("performance.cpp.range-for-copy", lineNo,
			"range-for loop copies each element by value; use const auto& or const T& unless a per-iteration copy is intentional")
	}
	if toggleEnabled(s.rules.DetectHotPathPatterns) && cppStreamFlushPattern.MatchString(line) {
		s.addFinding("performance.cpp.flush-in-loop", lineNo,
			"stream flush inside a loop defeats buffering and can dominate hot-path latency; write a newline and flush once after the loop unless immediate delivery is required")
	}
	if toggleEnabled(s.rules.DetectUnboundedConcurrency) && s.isUnboundedThreadLaunch(line) {
		s.addFinding("performance.cpp.unbounded-concurrency", lineNo,
			"C++ thread/task launch accumulated or detached inside a loop has no visible concurrency bound; use a fixed worker pool, semaphore, or bounded executor")
	}
}

func (s *cppPerformanceScan) isUnboundedThreadLaunch(line string) bool {
	if cppUnboundedThreadPattern.MatchString(line) {
		return true
	}
	match := cppContainerAppend.FindStringSubmatch(line)
	return match != nil && s.threadVec[match[1]]
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

func (s *cppPerformanceScan) isRangeForCopy(line string) bool {
	return cppRangeForStructuredCopy.MatchString(line) || cppRangeForValueCopy.MatchString(line)
}

func (s *cppPerformanceScan) addFinding(ruleID string, lineNo int, message string) {
	s.findings = append(s.findings, warnFinding(s.env, ruleID, s.file, lineNo, 1, message))
}
