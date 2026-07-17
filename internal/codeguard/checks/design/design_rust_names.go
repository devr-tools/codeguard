package design

import "strings"

func rustImplTargetName(header string) string {
	header = strings.Join(strings.Fields(strings.Split(header, "{")[0]), " ")
	header = strings.TrimSpace(strings.TrimPrefix(header, "impl"))
	header = strings.TrimSpace(trimRustGenericPrefix(header))
	if header == "" {
		return ""
	}
	if idx := strings.LastIndex(header, " for "); idx >= 0 {
		header = header[idx+5:]
	}
	if idx := strings.Index(header, " where "); idx >= 0 {
		header = header[:idx]
	}
	return rustPrimaryTypeName(strings.TrimSpace(header))
}

func trimRustGenericPrefix(header string) string {
	if !strings.HasPrefix(header, "<") {
		return header
	}
	depth := 0
	for idx, char := range header {
		switch char {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				return strings.TrimSpace(header[idx+1:])
			}
		}
	}
	return header
}

func rustPrimaryTypeName(target string) string {
	target = strings.TrimSpace(target)
	for {
		target = strings.TrimSpace(strings.TrimPrefix(target, "&"))
		target = strings.TrimSpace(strings.TrimPrefix(target, "mut "))
		target = strings.TrimSpace(strings.TrimPrefix(target, "dyn "))
		fields := strings.Fields(target)
		if len(fields) > 0 && strings.HasPrefix(fields[0], "'") {
			target = strings.TrimSpace(strings.Join(fields[1:], " "))
			continue
		}
		break
	}
	for _, sep := range []string{"<", " ", "(", "[", "{"} {
		if idx := strings.Index(target, sep); idx >= 0 {
			target = target[:idx]
		}
	}
	if idx := strings.LastIndex(target, "::"); idx >= 0 {
		target = target[idx+2:]
	}
	return strings.TrimSpace(target)
}
