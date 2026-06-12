package contracts

import "strings"

// currentProtoMessage joins enclosing message scopes into a qualified name,
// looking through oneof/enum blocks; fields inside services are ignored.
func currentProtoMessage(stack []protoScope) string {
	names := make([]string, 0, len(stack))
	for _, scope := range stack {
		if scope.kind == "service" {
			return ""
		}
		if scope.kind == "message" {
			names = append(names, scope.name)
		}
	}
	return strings.Join(names, ".")
}

func currentProtoService(stack []protoScope) string {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].kind == "service" {
			return stack[i].name
		}
	}
	return ""
}

func protoQualifiedName(stack []protoScope, name string) string {
	if parent := currentProtoMessage(stack); parent != "" {
		return parent + "." + name
	}
	return name
}

func normalizeProtoType(typ string) string {
	return strings.Join(strings.Fields(typ), "")
}
