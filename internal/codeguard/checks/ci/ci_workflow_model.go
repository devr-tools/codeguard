package ci

import (
	"bytes"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// workflowDoc is a parsed GitHub Actions workflow or composite action. The
// yaml.v3 node tree is kept (rather than decoding into structs) because every
// finding must point at the real source line, and the raw lines are kept so a
// match inside a multi-line `run: |` block can be pinned to its own line.
type workflowDoc struct {
	rel       string
	lines     []string
	composite bool
	root      *yaml.Node
	triggers  map[string]bool
	jobs      []workflowJob
}

type workflowJob struct {
	id          string
	key         *yaml.Node
	node        *yaml.Node
	permissions *yaml.Node
	steps       []workflowStep
}

type workflowStep struct {
	node    *yaml.Node
	uses    string
	runKey  *yaml.Node
	run     *yaml.Node
	withKey *yaml.Node
	with    *yaml.Node
}

func isWorkflowSecurityFile(rel string) bool {
	normalized := strings.ToLower(path.Clean(strings.ReplaceAll(rel, "\\", "/")))
	if isWorkflowPath(normalized) {
		return true
	}
	base := path.Base(normalized)
	return base == "action.yml" || base == "action.yaml"
}

// isWorkflowPath matches only direct children of .github/workflows: GitHub
// ignores workflow files in subdirectories, so those are not live workflows.
func isWorkflowPath(normalized string) bool {
	rest, ok := strings.CutPrefix(normalized, ".github/workflows/")
	if !ok || strings.Contains(rest, "/") {
		return false
	}
	return strings.HasSuffix(rest, ".yml") || strings.HasSuffix(rest, ".yaml")
}

// parseWorkflowDoc returns nil for files that are not workflows or composite
// actions, or that do not parse: a malformed workflow fails in GitHub itself,
// and guessing at its structure would only produce misplaced findings.
func parseWorkflowDoc(rel string, data []byte) *workflowDoc {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil || len(document.Content) == 0 {
		return nil
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	doc := &workflowDoc{
		rel:   rel,
		lines: strings.Split(string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))), "\n"),
		root:  root,
	}
	normalized := strings.ToLower(path.Clean(strings.ReplaceAll(rel, "\\", "/")))
	switch {
	case isWorkflowPath(normalized):
		doc.triggers = workflowTriggers(root)
		doc.jobs = workflowJobs(root)
	case isCompositeAction(root):
		doc.composite = true
		_, runs := mappingEntry(root, "runs")
		_, steps := mappingEntry(runs, "steps")
		doc.jobs = []workflowJob{{id: "composite", node: runs, steps: workflowSteps(steps)}}
	default:
		return nil
	}
	return doc
}

func isCompositeAction(root *yaml.Node) bool {
	_, runs := mappingEntry(root, "runs")
	_, using := mappingEntry(runs, "using")
	return using != nil && strings.EqualFold(strings.TrimSpace(using.Value), "composite")
}

func workflowTriggers(root *yaml.Node) map[string]bool {
	triggers := map[string]bool{}
	_, on := mappingEntry(root, "on")
	if on == nil {
		return triggers
	}
	switch on.Kind {
	case yaml.ScalarNode:
		triggers[strings.ToLower(on.Value)] = true
	case yaml.SequenceNode:
		for _, item := range on.Content {
			triggers[strings.ToLower(item.Value)] = true
		}
	case yaml.MappingNode:
		for idx := 0; idx+1 < len(on.Content); idx += 2 {
			triggers[strings.ToLower(on.Content[idx].Value)] = true
		}
	}
	return triggers
}

func workflowJobs(root *yaml.Node) []workflowJob {
	_, jobs := mappingEntry(root, "jobs")
	if jobs == nil || jobs.Kind != yaml.MappingNode {
		return nil
	}
	out := make([]workflowJob, 0, len(jobs.Content)/2)
	for idx := 0; idx+1 < len(jobs.Content); idx += 2 {
		key, node := jobs.Content[idx], jobs.Content[idx+1]
		if node.Kind != yaml.MappingNode {
			continue
		}
		_, permissions := mappingEntry(node, "permissions")
		_, steps := mappingEntry(node, "steps")
		out = append(out, workflowJob{id: key.Value, key: key, node: node, permissions: permissions, steps: workflowSteps(steps)})
	}
	return out
}

func workflowSteps(steps *yaml.Node) []workflowStep {
	if steps == nil || steps.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]workflowStep, 0, len(steps.Content))
	for _, node := range steps.Content {
		if node.Kind != yaml.MappingNode {
			continue
		}
		step := workflowStep{node: node}
		if _, uses := mappingEntry(node, "uses"); uses != nil {
			step.uses = strings.TrimSpace(uses.Value)
		}
		step.runKey, step.run = mappingEntry(node, "run")
		step.withKey, step.with = mappingEntry(node, "with")
		out = append(out, step)
	}
	return out
}

// mappingEntry returns the key and value nodes for key in a mapping node, or
// nils when node is not a mapping or lacks the key.
func mappingEntry(node *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, nil
	}
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		if node.Content[idx].Value == key {
			return node.Content[idx], node.Content[idx+1]
		}
	}
	return nil, nil
}

func (step workflowStep) usesAction(name string) bool {
	action, _, _ := strings.Cut(strings.ToLower(step.uses), "@")
	return action == name
}

// scalarSpan is a scalar value plus the source lines it occupies, so text
// matched inside the value can be located on its real line and column.
type scalarSpan struct {
	value string
	first int // 1-based line of the key (or the scalar itself)
	last  int // 1-based last line the scalar can occupy
}

// spanFor bounds a scalar by its key's indentation: the value ends at the
// first following non-blank line indented no deeper than the key, which holds
// for plain, quoted, literal (|) and folded (>) scalars alike.
func (doc *workflowDoc) spanFor(key *yaml.Node, value *yaml.Node) scalarSpan {
	anchor := value
	if key != nil {
		anchor = key
	}
	span := scalarSpan{value: value.Value, first: anchor.Line, last: anchor.Line}
	keyIndent := anchor.Column - 1
	for lineNo := anchor.Line + 1; lineNo <= len(doc.lines); lineNo++ {
		line := doc.lines[lineNo-1]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if leadingSpaces(line) <= keyIndent {
			break
		}
		span.last = lineNo
	}
	return span
}

func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

// spanCursor finds successive occurrences of needles within a span, so two
// identical expressions in one script are reported on their own lines.
type spanCursor struct {
	doc  *workflowDoc
	span scalarSpan
	line int
	col  int // 0-based byte offset within line already consumed
}

func (doc *workflowDoc) cursor(span scalarSpan) *spanCursor {
	return &spanCursor{doc: doc, span: span, line: span.first}
}

// locate returns the 1-based line and column of needle's next occurrence, or
// the span's first line when the raw text does not contain it verbatim (for
// example a double-quoted scalar whose escapes differ from the decoded value).
func (cursor *spanCursor) locate(needle string) (int, int) {
	for lineNo := cursor.line; lineNo <= cursor.span.last && lineNo <= len(cursor.doc.lines); lineNo++ {
		line := cursor.doc.lines[lineNo-1]
		start := 0
		if lineNo == cursor.line {
			start = cursor.col
		}
		if start > len(line) {
			continue
		}
		if offset := strings.Index(line[start:], needle); offset >= 0 {
			cursor.line = lineNo
			cursor.col = start + offset + len(needle)
			return lineNo, start + offset + 1
		}
	}
	return cursor.span.first, 1
}
