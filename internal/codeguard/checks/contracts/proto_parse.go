package contracts

import (
	"regexp"
	"strings"
)

type protoField struct {
	number string
	typ    string
}

// protoDefs is a deliberately lightweight, text-parsed view of a .proto file:
// message fields keyed by qualified message name, and rpc names keyed by
// service name. It assumes conventionally formatted protos (one declaration
// per line, closing braces on their own line).
type protoDefs struct {
	messages map[string]map[string]protoField
	services map[string]map[string]bool
}

type protoScope struct {
	kind string // "message", "service", or "block" (enum/oneof/extend/anonymous)
	name string
}

var (
	protoBlockCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)
	protoMessageRe      = regexp.MustCompile(`^message\s+(\w+)\s*\{`)
	protoServiceRe      = regexp.MustCompile(`^service\s+(\w+)\s*\{`)
	protoBlockRe        = regexp.MustCompile(`^(?:enum|oneof|extend)\b[^{]*\{`)
	protoRPCRe          = regexp.MustCompile(`^rpc\s+(\w+)\s*\(`)
	protoFieldRe        = regexp.MustCompile(`^(?:(?:optional|required|repeated)\s+)?(map\s*<[^>]*>|[\w.]+)\s+(\w+)\s*=\s*(\d+)`)
	protoKeywordRe      = regexp.MustCompile(`^(?:syntax|package|import|option|reserved|extensions)\b`)
)

func parseProto(src []byte) protoDefs {
	defs := protoDefs{
		messages: map[string]map[string]protoField{},
		services: map[string]map[string]bool{},
	}
	text := protoBlockCommentRe.ReplaceAllString(string(src), " ")
	var stack []protoScope
	for _, rawLine := range strings.Split(text, "\n") {
		line := rawLine
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		stack = parseProtoLine(line, stack, &defs)
	}
	return defs
}

func parseProtoLine(line string, stack []protoScope, defs *protoDefs) []protoScope {
	pushed := true
	switch {
	case protoMessageRe.MatchString(line):
		name := protoMessageRe.FindStringSubmatch(line)[1]
		qualified := protoQualifiedName(stack, name)
		stack = append(stack, protoScope{kind: "message", name: name})
		if _, ok := defs.messages[qualified]; !ok {
			defs.messages[qualified] = map[string]protoField{}
		}
	case protoServiceRe.MatchString(line):
		name := protoServiceRe.FindStringSubmatch(line)[1]
		stack = append(stack, protoScope{kind: "service", name: name})
		if _, ok := defs.services[name]; !ok {
			defs.services[name] = map[string]bool{}
		}
	case protoBlockRe.MatchString(line):
		stack = append(stack, protoScope{kind: "block"})
	default:
		pushed = false
		recordProtoStatement(line, stack, defs)
	}
	return adjustProtoStack(line, stack, pushed)
}

func recordProtoStatement(line string, stack []protoScope, defs *protoDefs) {
	if protoKeywordRe.MatchString(line) {
		return
	}
	if m := protoRPCRe.FindStringSubmatch(line); m != nil {
		if service := currentProtoService(stack); service != "" {
			defs.services[service][m[1]] = true
		}
		return
	}
	if m := protoFieldRe.FindStringSubmatch(line); m != nil {
		message := currentProtoMessage(stack)
		if message == "" {
			return
		}
		defs.messages[message][m[2]] = protoField{
			number: m[3],
			typ:    normalizeProtoType(m[1]),
		}
	}
}

func adjustProtoStack(line string, stack []protoScope, pushed bool) []protoScope {
	opens := strings.Count(line, "{")
	if pushed && opens > 0 {
		opens--
	}
	for i := 0; i < opens; i++ {
		stack = append(stack, protoScope{kind: "block"})
	}
	for i := 0; i < strings.Count(line, "}"); i++ {
		if len(stack) > 0 {
			stack = stack[:len(stack)-1]
		}
	}
	return stack
}
