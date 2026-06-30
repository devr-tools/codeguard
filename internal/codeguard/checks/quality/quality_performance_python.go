package quality

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	pythonLoopStartPattern = regexp.MustCompile(`^\s*(?:for\s+.+\s+in\s+.+:|while\s+.+:)`)
	pythonAsyncDefPattern  = regexp.MustCompile(`^\s*async\s+def\s+`)
	pythonQueryCallPattern = regexp.MustCompile(`\b(?:requests|httpx)\.(?:get|post|put|delete|patch|head)\s*\(|\bcursor\.execute\s*\(|\.execute\s*\(|\bsession\.query\s*\(`)
	pythonSyncInAsyncCall  = regexp.MustCompile(`\brequests\.\w+\s*\(|\burllib\.request\.urlopen\s*\(|\btime\.sleep\s*\(`)
)

func pythonPerformanceFindings(env support.Context, file string, data []byte) []core.Finding {
	scan := &pythonPerformanceScan{env: env, file: file, rules: env.Config.Checks.QualityRules}
	for idx, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		scan.consumeLine(idx+1, line)
	}
	return scan.findings
}

type pythonPerformanceScan struct {
	env       support.Context
	file      string
	rules     core.QualityRulesConfig
	loops     []int
	asyncDefs []int
	findings  []core.Finding
}

func (s *pythonPerformanceScan) consumeLine(lineNo int, line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	indent := indentationWidth(line)
	s.loops = popIndentRegions(s.loops, indent)
	s.asyncDefs = popIndentRegions(s.asyncDefs, indent)
	startsLoop := pythonLoopStartPattern.MatchString(line)
	s.checkLine(lineNo, line, len(s.loops) > 0 || startsLoop, len(s.asyncDefs) > 0)
	if startsLoop {
		s.loops = append(s.loops, indent)
	}
	if pythonAsyncDefPattern.MatchString(line) {
		s.asyncDefs = append(s.asyncDefs, indent)
	}
}

func (s *pythonPerformanceScan) checkLine(lineNo int, line string, inLoop bool, inAsync bool) {
	if inLoop && qualityToggleEnabled(s.rules.DetectNPlusOneQuery) && pythonQueryCallPattern.MatchString(line) {
		s.addFinding("quality.n-plus-one-query", lineNo,
			"query or request call inside a loop suggests an N+1 pattern; batch the work or hoist the call out of the loop")
	}
	if inAsync && qualityToggleEnabled(s.rules.DetectSyncIOInHandlers) && pythonSyncInAsyncCall.MatchString(line) {
		s.addFinding("quality.python.sync-io-in-async", lineNo,
			"blocking call inside an async function stalls the event loop; use an async client or asyncio.sleep")
	}
}

func (s *pythonPerformanceScan) addFinding(ruleID string, lineNo int, message string) {
	s.findings = append(s.findings, warnFinding(s.env, ruleID, s.file, lineNo, 1, message))
}

func popIndentRegions(regions []int, indent int) []int {
	for len(regions) > 0 && indent <= regions[len(regions)-1] {
		regions = regions[:len(regions)-1]
	}
	return regions
}
func indentationWidth(line string) int {
	width := 0
	for _, ch := range line {
		if ch == ' ' {
			width++
			continue
		}
		if ch == '\t' {
			width += 4
			continue
		}
		break
	}
	return width
}
