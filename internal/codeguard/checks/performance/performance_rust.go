package performance

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	rustLoopStartPattern    = regexp.MustCompile(`(?:^|[^\w])(?:for|while|loop)\b`)
	rustRegexImportPattern  = regexp.MustCompile(`\buse\s+regex::Regex\b|\bregex::Regex::new\s*\(`)
	rustRegexCompilePattern = regexp.MustCompile(`(?:^|[^\w:])(?:regex::)?Regex::new\s*\(\s*(?:r|br)?#*"`)
	rustStringInitPattern   = regexp.MustCompile(`^\s*let\s+mut\s+([A-Za-z_]\w*)\s*(?::\s*String)?\s*=\s*(String::new\s*\(\s*\)|String::with_capacity\s*\(|String::from\s*\(\s*"|String::from\s*\(\s*r#*"|format!\s*\()`)
	rustAugmentedAssign     = regexp.MustCompile(`^\s*([A-Za-z_]\w*)\s*\+=`)
	rustPushStrPattern      = regexp.MustCompile(`\b([A-Za-z_]\w*)\.push_str\s*\(`)
	rustThreadSleepPattern  = regexp.MustCompile(`\b(?:std::thread|thread)::sleep\s*\(`)
	rustFormatMacroPattern  = regexp.MustCompile(`\bformat!\s*\(`)
	rustIdentPattern        = regexp.MustCompile(`^[A-Za-z_]\w*$`)
)

func rustPerformanceFindings(env support.Context, file string, data []byte) []core.Finding {
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	masked := support.MaskCLikeSource(source, support.CLikeRust)
	scan := &rustPerformanceScan{
		env:       env,
		file:      file,
		rules:     env.Config.Checks.PerformanceRules,
		hasRegex:  rustRegexImportPattern.MatchString(source),
		stringVar: make(map[string]rustStringState),
	}
	rawLines := strings.Split(source, "\n")
	for idx, line := range strings.Split(masked, "\n") {
		scan.consumeLine(idx+1, line, rawLines[idx])
	}
	return scan.findings
}

type rustStringState struct {
	preallocated bool
}

type rustPerformanceScan struct {
	env       support.Context
	file      string
	rules     core.PerformanceRulesConfig
	hasRegex  bool
	depth     int
	loops     []int
	stringVar map[string]rustStringState
	findings  []core.Finding
}

func (s *rustPerformanceScan) consumeLine(lineNo int, line string, rawLine string) {
	if m := rustStringInitPattern.FindStringSubmatch(rawLine); m != nil {
		s.stringVar[m[1]] = rustStringState{preallocated: strings.HasPrefix(strings.TrimSpace(m[2]), "String::with_capacity")}
	}
	startsLoop := rustLoopStartPattern.MatchString(line)
	s.checkLine(lineNo, line, rawLine, len(s.loops) > 0 || startsLoop)
	s.depth, s.loops = consumeBraceLoopLine(s.depth, s.loops, line, startsLoop)
}

func (s *rustPerformanceScan) checkLine(lineNo int, line string, rawLine string, inLoop bool) {
	if !inLoop {
		return
	}
	if toggleEnabled(s.rules.DetectRegexCompileInLoop) && s.hasRegex && rustRegexCompilePattern.MatchString(rawLine) {
		s.addFinding("performance.regex-compile-in-loop", lineNo,
			"Regex::new inside a loop recompiles the pattern every iteration; compile it once with OnceLock, lazy_static, or before the loop")
	}
	if toggleEnabled(s.rules.DetectAllocInLoop) && s.isStringGrowth(line, rawLine) {
		s.addFinding("performance.rust.alloc-in-loop", lineNo,
			"String grown inside a loop without visible preallocation; collect parts first or initialize with String::with_capacity before the loop")
	}
	if toggleEnabled(s.rules.DetectSleepInLoop) && rustThreadSleepPattern.MatchString(line) {
		s.addFinding("performance.rust.sleep-in-loop", lineNo,
			"thread::sleep inside a loop usually marks a poll; prefer a channel, Condvar, timer primitive, or bounded backoff helper")
	}
}

func (s *rustPerformanceScan) isStringGrowth(line string, rawLine string) bool {
	if m := rustAugmentedAssign.FindStringSubmatch(line); m != nil {
		state, ok := s.stringVar[m[1]]
		return ok && !state.preallocated
	}
	if name, ok := rustReassignedConcatVar(line); ok {
		state, ok := s.stringVar[name]
		return ok && !state.preallocated
	}
	if m := rustPushStrPattern.FindStringSubmatch(line); m != nil {
		state, ok := s.stringVar[m[1]]
		return ok && !state.preallocated
	}
	return rustFormatMacroPattern.MatchString(rawLine) && strings.Contains(rawLine, "+=")
}

func (s *rustPerformanceScan) addFinding(ruleID string, lineNo int, message string) {
	s.findings = append(s.findings, warnFinding(s.env, ruleID, s.file, lineNo, 1, message))
}

func rustReassignedConcatVar(line string) (string, bool) {
	eq := strings.Index(line, "=")
	if eq <= 0 {
		return "", false
	}
	left := strings.TrimSpace(line[:eq])
	if !rustIdentPattern.MatchString(left) {
		return "", false
	}
	right := strings.TrimSpace(line[eq+1:])
	if !strings.HasPrefix(right, left) {
		return "", false
	}
	right = strings.TrimSpace(strings.TrimPrefix(right, left))
	return left, strings.HasPrefix(right, "+")
}
