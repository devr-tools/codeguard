package design

import (
	"path"
	"regexp"
	"strings"
)

func designPathMatches(patterns []string, value string) bool {
	normalized := normalizeDesignPath(value)
	for _, pattern := range patterns {
		if designPatternMatches(pattern, normalized) {
			return true
		}
	}
	return false
}

func designPatternMatches(pattern string, value string) bool {
	pattern = normalizeDesignPath(pattern)
	if pattern == "" {
		return false
	}
	if !strings.ContainsAny(pattern, "*?[") {
		return value == pattern || strings.HasPrefix(value, strings.TrimSuffix(pattern, "/")+"/")
	}
	if !strings.Contains(pattern, "**") {
		matched, err := path.Match(pattern, value)
		return err == nil && matched
	}
	re, err := regexp.Compile(designGlobRegex(pattern))
	return err == nil && re.MatchString(value)
}

func normalizeDesignPath(value string) string {
	trimmed := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if trimmed == "" {
		return ""
	}
	return strings.TrimPrefix(path.Clean(trimmed), "./")
}

func designGlobRegex(pattern string) string {
	var out strings.Builder
	out.WriteString("^")
	for idx := 0; idx < len(pattern); {
		switch pattern[idx] {
		case '*':
			if idx+1 < len(pattern) && pattern[idx+1] == '*' {
				idx += 2
				if idx < len(pattern) && pattern[idx] == '/' {
					out.WriteString("(?:.*/)?")
					idx++
				} else {
					out.WriteString(".*")
				}
				continue
			}
			out.WriteString("[^/]*")
		case '?':
			out.WriteString("[^/]")
		case '[':
			idx = writeDesignGlobClass(&out, pattern, idx)
		default:
			out.WriteString(regexp.QuoteMeta(string(pattern[idx])))
		}
		idx++
	}
	out.WriteString("$")
	return out.String()
}

func writeDesignGlobClass(out *strings.Builder, pattern string, idx int) int {
	end := strings.IndexByte(pattern[idx+1:], ']')
	if end < 0 {
		out.WriteString(`\[`)
		return idx
	}
	end += idx + 1
	class := pattern[idx+1 : end]
	if strings.HasPrefix(class, "!") {
		out.WriteByte('^')
		class = strings.TrimPrefix(class, "!")
	}
	out.WriteByte('[')
	out.WriteString(strings.ReplaceAll(class, `\`, `\\`))
	out.WriteByte(']')
	return end
}
