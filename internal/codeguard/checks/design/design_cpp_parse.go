package design

import (
	"path/filepath"
	"strings"
)

func cppMethodKey(line string, typeName string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "using ") ||
		strings.HasPrefix(trimmed, "typedef ") || strings.HasPrefix(trimmed, "friend ") ||
		strings.HasPrefix(trimmed, "static_assert") || strings.HasPrefix(trimmed, "return ") {
		return "", false
	}
	open := strings.Index(trimmed, "(")
	if open < 0 {
		return "", false
	}
	head := strings.TrimSpace(trimmed[:open])
	if head == "" || strings.Contains(head, "=") {
		return "", false
	}
	name := cppTrailingIdentifier(head)
	if name == "" || cppNonMethodName(name) {
		return "", false
	}
	if name != typeName && name != "~"+typeName && strings.Contains(name, "::") {
		name = name[strings.LastIndex(name, "::")+2:]
	}
	closePos := cppMatchingParen(trimmed, open)
	if closePos < 0 {
		return "", false
	}
	trailer := strings.TrimSpace(trimmed[closePos+1:])
	if strings.HasPrefix(trailer, "->") {
		return "", false
	}
	params := cppSquashWhitespace(trimmed[open+1 : closePos])
	return name + "(" + params + ")", true
}

func cppTrailingIdentifier(head string) string {
	head = strings.TrimRight(head, " \t*&")
	if head == "" {
		return ""
	}
	start := len(head)
	for start > 0 {
		ch := head[start-1]
		if ch == '_' || ch == '~' || ch == ':' ||
			(ch >= '0' && ch <= '9') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= 'a' && ch <= 'z') {
			start--
			continue
		}
		break
	}
	return head[start:]
}

func cppNonMethodName(name string) bool {
	switch name {
	case "if", "for", "while", "switch", "catch", "return", "requires":
		return true
	default:
		return name == ""
	}
}

func cppMatchingParen(line string, open int) int {
	depth := 0
	for i := open; i < len(line); i++ {
		switch line[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func openCPPNamespaceBlocks(blocks []*cppNamespaceBlock, depth int, line string) {
	if !strings.Contains(line, "{") {
		return
	}
	for _, block := range blocks {
		if block == nil || !block.waiting {
			continue
		}
		block.waiting = false
		block.bodyDepth = depth
	}
}

func openCPPTypeBlocks(blocks []*cppTypeBlock, depth int, line string) {
	if !strings.Contains(line, "{") {
		return
	}
	for _, block := range blocks {
		if block == nil || !block.waiting {
			continue
		}
		block.waiting = false
		block.bodyDepth = depth
		block.access = block.defaultAccess
	}
}

func pruneCPPNamespaceBlocks(blocks []*cppNamespaceBlock, depth int, line string) []*cppNamespaceBlock {
	kept := blocks[:0]
	for _, block := range blocks {
		if block == nil {
			continue
		}
		if block.waiting && strings.Contains(line, ";") && !strings.Contains(line, "{") {
			continue
		}
		if !block.waiting && depth < block.bodyDepth {
			continue
		}
		kept = append(kept, block)
	}
	return kept
}

func pruneCPPTypeBlocks(blocks []*cppTypeBlock, depth int, line string) []*cppTypeBlock {
	kept := blocks[:0]
	for _, block := range blocks {
		if block == nil {
			continue
		}
		if block.waiting && strings.Contains(line, ";") && !strings.Contains(line, "{") {
			continue
		}
		if !block.waiting && depth < block.bodyDepth {
			continue
		}
		kept = append(kept, block)
	}
	return kept
}

func isCPPContractPath(file string) bool {
	rawExt := filepath.Ext(file)
	if rawExt == ".C" {
		return false
	}
	switch strings.ToLower(rawExt) {
	case ".h", ".hh", ".hpp", ".hxx", ".h++", ".ipp", ".tpp", ".inl", ".txx", ".ixx",
		".cppm", ".cxxm", ".ccm", ".c++m", ".mpp", ".mxx", ".inc":
		return true
	default:
		return false
	}
}

func cppSquashWhitespace(text string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}
