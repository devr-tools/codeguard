package report

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// The taint engines encode the source-to-sink chain in the finding message.
// codeFlows are rebuilt from that text rather than from new finding fields so
// cached and baseline-replayed findings render traces too.
var (
	// Go, Python, and C++ engines (checks/security taintChainMessage):
	// "tainted data from <source> (line N) reaches <sink> (line M) via a -> b -> <sink>".
	nativeTaintMessage = regexp.MustCompile(`^tainted data from (.+?) \(line (\d+)\) reaches (.+?) \(line (\d+)\) via (.+)$`)
	// TypeScript/JavaScript engine: "tainted data flow: <label> (path:line) → ...".
	scriptTaintMessage = regexp.MustCompile(`^tainted data flow: (.+)$`)
	scriptTaintStep    = regexp.MustCompile(`^(.*\S) \(([^()]+):(\d+)\)$`)
)

type taintStep struct {
	label string
	path  string
	line  int
	kinds []string
}

// sarifCodeFlows returns a single-thread code flow for a taint finding whose
// message carries a recognizable chain, or nil.
func sarifCodeFlows(finding core.Finding) []sarifCodeFlow {
	steps := taintSteps(finding)
	if len(steps) < 2 {
		return nil
	}
	locations := make([]sarifThreadFlowLocation, 0, len(steps))
	for _, step := range steps {
		location := sarifLocation{Message: &sarifMessage{Text: step.label}}
		if step.path != "" {
			location.PhysicalLocation = &sarifPhysicalLocation{ArtifactLocation: sarifArtifactLocation{URI: step.path}}
			if step.line > 0 {
				location.PhysicalLocation.Region = &sarifRegion{StartLine: step.line}
			}
		}
		locations = append(locations, sarifThreadFlowLocation{Location: location, Kinds: step.kinds})
	}
	return []sarifCodeFlow{{
		Message:     &sarifMessage{Text: firstNonEmpty(finding.Message, finding.Why)},
		ThreadFlows: []sarifThreadFlow{{Locations: locations}},
	}}
}

func taintSteps(finding core.Finding) []taintStep {
	message := strings.TrimSpace(finding.Message)
	if match := nativeTaintMessage.FindStringSubmatch(message); match != nil {
		return nativeTaintSteps(finding.Path, match)
	}
	if match := scriptTaintMessage.FindStringSubmatch(message); match != nil {
		return scriptTaintSteps(finding.Path, match[1])
	}
	return nil
}

// nativeTaintSteps expands the Go/Python/C++ chain. Those engines analyse one
// file at a time, so every step is in the finding's file; only the source and
// sink carry line numbers, so intermediate steps get the file without a
// region and the step name as their message.
func nativeTaintSteps(path string, match []string) []taintStep {
	source, sink := match[1], match[3]
	sourceLine, _ := strconv.Atoi(match[2])
	sinkLine, _ := strconv.Atoi(match[4])
	chain := splitNonEmpty(match[5], " -> ")
	// The chain repeats the source and sink at its ends; they already get
	// their own located steps.
	if len(chain) > 0 && chain[0] == source {
		chain = chain[1:]
	}
	if n := len(chain); n > 0 && chain[n-1] == sink {
		chain = chain[:n-1]
	}
	steps := make([]taintStep, 0, len(chain)+2)
	steps = append(steps, taintStep{label: "source: " + source, path: path, line: sourceLine, kinds: []string{"taint", "source"}})
	for _, name := range chain {
		steps = append(steps, taintStep{label: "flows through " + name, path: path, kinds: []string{"taint"}})
	}
	return append(steps, taintStep{label: "sink: " + sink, path: path, line: sinkLine, kinds: []string{"taint", "sink"}})
}

// scriptTaintSteps expands the TypeScript/JavaScript chain, whose steps each
// name their own file and line ("req.query (src/app.ts:12)") because that
// engine follows flows across modules. A step without a location falls back
// to the finding's file.
func scriptTaintSteps(path string, chain string) []taintStep {
	parts := splitNonEmpty(chain, " → ")
	steps := make([]taintStep, 0, len(parts))
	for i, part := range parts {
		step := taintStep{label: part, path: path, kinds: []string{"taint"}}
		if match := scriptTaintStep.FindStringSubmatch(part); match != nil {
			step.label = match[1]
			step.path = match[2]
			step.line, _ = strconv.Atoi(match[3])
		}
		switch i {
		case 0:
			step.kinds = []string{"taint", "source"}
		case len(parts) - 1:
			step.kinds = []string{"taint", "sink"}
		}
		steps = append(steps, step)
	}
	return steps
}

func splitNonEmpty(value string, separator string) []string {
	var parts []string
	for _, part := range strings.Split(value, separator) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}
