package security

import (
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// taintSummaryPasses is the number of whole-file passes each engine runs:
// the summary-only passes resolve bounded same-file call chains, and the last
// pass reports findings against the settled summaries.
const taintSummaryPasses = 3

// taintRules names the rules one language engine reports: SSRF sinks get
// ssrfRuleID and every other sink gets taintRuleID. The ids are literal
// fields so the catalog emission-coverage test can resolve them statically.
type taintRules struct {
	taintRuleID string
	ssrfRuleID  string
	isSSRFSink  func(sink string) bool
}

func (r taintRules) forSink(sink string) string {
	if r.isSSRFSink(sink) {
		return r.ssrfRuleID
	}
	return r.taintRuleID
}

// taintEngine is the per-file state shared by the Go, Python and C++ taint
// analyzers: function summaries from the previous pass and the deduplicated
// findings emitted so far.
type taintEngine struct {
	env   support.Context
	file  string
	rules taintRules
	// keepReturnModel carries a callee's framework source/sink models onto
	// the value its summarised return produces at a call site.
	keepReturnModel bool
	summaries       map[string]*funcSummary
	findings        []core.Finding
	seen            map[string]struct{}
}

func newTaintEngine(env support.Context, file string, rules taintRules, keepReturnModel bool) taintEngine {
	return taintEngine{
		env:             env,
		file:            file,
		rules:           rules,
		keepReturnModel: keepReturnModel,
		summaries:       map[string]*funcSummary{},
		seen:            map[string]struct{}{},
	}
}

// runSummaryPasses calls analyzeAll once per pass with a fresh summary map
// to fill; only the final pass emits findings.
func (e *taintEngine) runSummaryPasses(analyzeAll func(emit bool, next map[string]*funcSummary)) {
	for pass := 0; pass < taintSummaryPasses; pass++ {
		next := map[string]*funcSummary{}
		analyzeAll(pass == taintSummaryPasses-1, next)
		e.summaries = next
	}
}

// emitFinding records one deduplicated source-to-sink finding.
func (e *taintEngine) emitFinding(taint *taintTrace, sink string, sinkLine int) {
	e.findings = appendTaintFinding(e.env, e.file, e.seen, e.findings, taintSinkInput{
		ruleID:     e.rules.forSink(sink),
		source:     taint.source,
		sourceLine: taint.sourceLine,
		chain:      taint.chain,
		sink:       sink,
		sinkLine:   sinkLine,
		model:      taint.model,
		sinkModel:  taint.sinkModel,
	})
}

// taintScope is the per-function state every engine shares: tracked
// variables and the summary being built for the function.
type taintScope struct {
	engine  *taintEngine
	emit    bool
	vars    map[string]*taintTrace
	summary *funcSummary
}

func newTaintScope(engine *taintEngine, emit bool) taintScope {
	return taintScope{engine: engine, emit: emit, vars: map[string]*taintTrace{}, summary: newFuncSummary()}
}

// reportSink emits a finding for a concrete source and records a summary
// entry for parameter-conditional taint, so callers decide whether the flow
// is dangerous. A nil trace is ignored.
func (s *taintScope) reportSink(taint *taintTrace, sink string, line int) {
	if taint == nil {
		return
	}
	if taint.paramIndex >= 0 {
		s.summary.paramsToSink = append(s.summary.paramsToSink, paramSink{
			paramIndex: taint.paramIndex,
			sink:       sink,
			line:       line,
		})
		return
	}
	if s.emit {
		s.engine.emitFinding(taint, sink, line)
	}
}

// reportFirstTainted reports the first non-nil trace among candidates.
func (s *taintScope) reportFirstTainted(sink string, line int, candidates ...*taintTrace) {
	for _, taint := range candidates {
		if taint != nil {
			s.reportSink(taint, sink, line)
			return
		}
	}
}

// recordReturn folds one returned value into the function summary.
func (s *taintScope) recordReturn(taint *taintTrace) {
	switch {
	case taint == nil:
		// No summary effect for a return value proven untainted.
	case taint.paramIndex >= 0:
		s.summary.paramsToReturn[taint.paramIndex] = true
	case s.summary.returnTaint == nil:
		s.summary.returnTaint = taint
	}
}

// bind assigns a value to name: a tainted value extends its chain with the
// name, and an untainted one clears earlier taint unless allowClear is false
// (an augmented assignment only appends to the existing value).
func (s *taintScope) bind(name string, taint *taintTrace, allowClear bool) {
	if taint != nil {
		s.vars[name] = taint.extended(name)
		return
	}
	if allowClear {
		delete(s.vars, name)
	}
}

// applyParamSinks reports flows where a tainted argument reaches a sink
// inside a same-file callee. argTaint evaluates the argument at an index
// below argCount.
func (s *taintScope) applyParamSinks(callee string, summary *funcSummary, argCount int, argTaint func(int) *taintTrace) {
	for _, sink := range summary.paramsToSink {
		if sink.paramIndex >= argCount {
			continue
		}
		if taint := argTaint(sink.paramIndex); taint != nil {
			s.reportSink(taint.extended(callee+"()"), sink.sink, sink.line)
		}
	}
}

// summaryCallTaint is the taint a call to a summarised same-file function
// produces: its concrete tainted return, or else the first tainted argument
// whose parameter flows to the return.
func (s *taintScope) summaryCallTaint(callee string, summary *funcSummary, argCount int, argTaint func(int) *taintTrace) *taintTrace {
	step := callee + "()"
	if inner := summary.returnTaint; inner != nil {
		lifted := &taintTrace{
			source:     inner.source,
			sourceLine: inner.sourceLine,
			chain:      append(append([]string{}, inner.chain...), step),
			paramIndex: -1,
		}
		if s.engine.keepReturnModel {
			lifted.model = inner.model
			lifted.sinkModel = inner.sinkModel
		}
		return lifted
	}
	for index := 0; index < argCount; index++ {
		if !summary.paramsToReturn[index] {
			continue
		}
		if taint := argTaint(index); taint != nil {
			return taint.extended(step)
		}
	}
	return nil
}
