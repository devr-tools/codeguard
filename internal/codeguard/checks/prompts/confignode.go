package prompts

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// configNode is a format-neutral view of a parsed JSON, JSONC, or YAML config
// document. Agent and MCP configs ship in all three formats, and every rule
// needs the line a key sits on, which encoding/json does not expose; parsing
// into this tree once lets each rule report the real line instead of line 1.
type configNode struct {
	kind   configNodeKind
	scalar string
	line   int
	keys   []string
	fields map[string]*configNode
	items  []*configNode
}

type configNodeKind int

const (
	nodeScalar configNodeKind = iota
	nodeString
	nodeObject
	nodeArray
)

// maxConfigDepth bounds recursion so a hostile, deeply nested file cannot
// exhaust the stack of the scanning goroutine.
const maxConfigDepth = 256

func (n *configNode) field(name string) *configNode {
	if n == nil || n.kind != nodeObject {
		return nil
	}
	return n.fields[name]
}

// str returns the node's text when it is a string, so rules can treat a
// missing key and a non-string value identically.
func (n *configNode) str() (string, bool) {
	if n == nil || n.kind != nodeString {
		return "", false
	}
	return n.scalar, true
}

func (n *configNode) isTrue() bool {
	return n != nil && (n.kind == nodeScalar || n.kind == nodeString) && strings.EqualFold(n.scalar, "true")
}

// stringItems returns the string elements of an array node; non-string
// elements are skipped rather than failing the whole array.
func (n *configNode) stringItems() []string {
	if n == nil || n.kind != nodeArray {
		return nil
	}
	out := make([]string, 0, len(n.items))
	for _, item := range n.items {
		if value, ok := item.str(); ok {
			out = append(out, value)
		}
	}
	return out
}

// parseConfigDocument parses data by extension. It returns ok=false for
// formats it does not understand (TOML) and for malformed documents, which
// callers treat as "fall back to line heuristics", never as a section error.
func parseConfigDocument(rel string, data []byte) (*configNode, bool) {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".json", ".jsonc":
		return parseJSONC(data)
	case ".yaml", ".yml":
		return parseYAMLConfig(data)
	default:
		return nil, false
	}
}

func parseYAMLConfig(data []byte) (root *configNode, ok bool) {
	defer func() {
		if recover() != nil {
			root, ok = nil, false
		}
	}()
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, false
	}
	node := fromYAMLNode(&doc, 0)
	return node, node != nil
}

func fromYAMLNode(node *yaml.Node, depth int) *configNode {
	if node == nil || depth > maxConfigDepth {
		return nil
	}
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return nil
		}
		return fromYAMLNode(node.Content[0], depth+1)
	case yaml.AliasNode:
		return fromYAMLNode(node.Alias, depth+1)
	case yaml.MappingNode:
		out := &configNode{kind: nodeObject, line: node.Line, fields: map[string]*configNode{}}
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			child := fromYAMLNode(node.Content[idx+1], depth+1)
			if child == nil {
				continue
			}
			out.setField(node.Content[idx].Value, child)
		}
		return out
	case yaml.SequenceNode:
		out := &configNode{kind: nodeArray, line: node.Line}
		for _, item := range node.Content {
			if child := fromYAMLNode(item, depth+1); child != nil {
				out.items = append(out.items, child)
			}
		}
		return out
	default:
		kind := nodeScalar
		if node.Tag == "!!str" {
			kind = nodeString
		}
		return &configNode{kind: kind, scalar: node.Value, line: node.Line}
	}
}

func (n *configNode) setField(key string, child *configNode) {
	if _, exists := n.fields[key]; !exists {
		n.keys = append(n.keys, key)
	}
	n.fields[key] = child
}

// jsoncParser is a small recursive-descent JSON parser that also accepts the
// JSONC extensions editors and agent CLIs write (// and /* */ comments,
// trailing commas) and records the line of every value.
type jsoncParser struct {
	src  []byte
	pos  int
	line int
}

func parseJSONC(data []byte) (*configNode, bool) {
	p := &jsoncParser{src: data, line: 1}
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		p.pos = 3
	}
	root, ok := p.value(0)
	if !ok {
		return nil, false
	}
	if !p.skipSpace() {
		return nil, false
	}
	return root, p.pos == len(p.src)
}

func (p *jsoncParser) value(depth int) (*configNode, bool) {
	if depth > maxConfigDepth || !p.skipSpace() || p.pos >= len(p.src) {
		return nil, false
	}
	switch p.src[p.pos] {
	case '{':
		return p.object(depth)
	case '[':
		return p.array(depth)
	case '"':
		line := p.line
		text, ok := p.stringLiteral()
		return &configNode{kind: nodeString, scalar: text, line: line}, ok
	default:
		return p.literal()
	}
}

func (p *jsoncParser) object(depth int) (*configNode, bool) {
	out := &configNode{kind: nodeObject, line: p.line, fields: map[string]*configNode{}}
	p.pos++
	for {
		if !p.skipSpace() || p.pos >= len(p.src) {
			return nil, false
		}
		if p.src[p.pos] == '}' {
			p.pos++
			return out, true
		}
		if p.src[p.pos] != '"' {
			return nil, false
		}
		key, ok := p.stringLiteral()
		if !ok || !p.skipSpace() || p.pos >= len(p.src) || p.src[p.pos] != ':' {
			return nil, false
		}
		p.pos++
		child, ok := p.value(depth + 1)
		if !ok {
			return nil, false
		}
		out.setField(key, child)
		if !p.separator('}') {
			return nil, false
		}
	}
}

func (p *jsoncParser) array(depth int) (*configNode, bool) {
	out := &configNode{kind: nodeArray, line: p.line}
	p.pos++
	for {
		if !p.skipSpace() || p.pos >= len(p.src) {
			return nil, false
		}
		if p.src[p.pos] == ']' {
			p.pos++
			return out, true
		}
		child, ok := p.value(depth + 1)
		if !ok {
			return nil, false
		}
		out.items = append(out.items, child)
		if !p.separator(']') {
			return nil, false
		}
	}
}

// separator consumes the comma between members. A closing bracket right
// after a member is left for the caller; a trailing comma is tolerated
// because the next loop iteration sees the bracket.
func (p *jsoncParser) separator(closer byte) bool {
	if !p.skipSpace() || p.pos >= len(p.src) {
		return false
	}
	switch p.src[p.pos] {
	case ',':
		p.pos++
		return true
	case closer:
		return true
	default:
		return false
	}
}

func (p *jsoncParser) stringLiteral() (string, bool) {
	start := p.pos
	p.pos++
	for p.pos < len(p.src) {
		switch p.src[p.pos] {
		case '\\':
			p.pos += 2
			continue
		case '\n':
			return "", false
		case '"':
			p.pos++
			var text string
			if err := json.Unmarshal(p.src[start:p.pos], &text); err != nil {
				return "", false
			}
			return text, true
		}
		p.pos++
	}
	return "", false
}

func (p *jsoncParser) literal() (*configNode, bool) {
	start := p.pos
	for p.pos < len(p.src) && strings.IndexByte("+-.0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", p.src[p.pos]) >= 0 {
		p.pos++
	}
	text := string(p.src[start:p.pos])
	switch {
	case text == "true" || text == "false" || text == "null":
	case text != "" && json.Valid([]byte(text)):
	default:
		return nil, false
	}
	return &configNode{kind: nodeScalar, scalar: text, line: p.line}, true
}

// skipSpace advances past whitespace and comments, counting newlines. It
// returns false only for an unterminated block comment.
func (p *jsoncParser) skipSpace() bool {
	for p.pos < len(p.src) {
		ch := p.src[p.pos]
		switch {
		case ch == '\n':
			p.line++
			p.pos++
		case ch == ' ' || ch == '\t' || ch == '\r':
			p.pos++
		case ch == '/' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '/':
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
		case ch == '/' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '*':
			if !p.skipBlockComment() {
				return false
			}
		default:
			return true
		}
	}
	return true
}

func (p *jsoncParser) skipBlockComment() bool {
	p.pos += 2
	for p.pos+1 < len(p.src) {
		if p.src[p.pos] == '*' && p.src[p.pos+1] == '/' {
			p.pos += 2
			return true
		}
		if p.src[p.pos] == '\n' {
			p.line++
		}
		p.pos++
	}
	return false
}
