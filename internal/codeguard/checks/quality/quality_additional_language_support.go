package quality

import "strings"

func braceLanguageFunctions(source string, pattern matcherWithSubmatch, countParams func(string) int, complexityFn func(string) int, excludedNames map[string]struct{}) []functionMetrics {
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	functions := make([]functionMetrics, 0)
	for idx, line := range lines {
		match := pattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		name := match[1]
		if _, skip := excludedNames[strings.ToLower(name)]; skip {
			continue
		}

		endIdx := braceDelimitedEnd(lines, idx)
		bodyStart := min(idx+1, len(lines))
		bodyEnd := min(endIdx+1, len(lines))
		body := strings.Join(lines[bodyStart:bodyEnd], "\n")
		functions = append(functions, functionMetrics{
			Name:       name,
			StartLine:  idx + 1,
			Length:     max(1, endIdx-idx+1),
			Params:     countParams(match[2]),
			Complexity: complexityFn(body),
		})
	}
	return functions
}

type matcherWithSubmatch interface {
	FindStringSubmatch(string) []string
}

func rubyFunctions(source string) []functionMetrics {
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	functions := make([]functionMetrics, 0)
	for idx, line := range lines {
		match := rubyFunctionPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		signature := match[2]
		if signature == "" {
			signature = match[3]
		}
		endIdx := rubyFunctionEnd(lines, idx)
		bodyStart := min(idx+1, len(lines))
		bodyEnd := min(endIdx+1, len(lines))
		body := strings.Join(lines[bodyStart:bodyEnd], "\n")
		functions = append(functions, functionMetrics{
			Name:       match[1],
			StartLine:  idx + 1,
			Length:     max(1, endIdx-idx+1),
			Params:     rubyParameterCount(signature),
			Complexity: rubyComplexity(body),
		})
	}
	return functions
}

func braceDelimitedEnd(lines []string, start int) int {
	depth := strings.Count(lines[start], "{") - strings.Count(lines[start], "}")
	if depth == 0 && strings.Contains(lines[start], "{") && strings.Contains(lines[start], "}") {
		return start
	}
	if depth <= 0 {
		depth = 1
	}
	for idx := start + 1; idx < len(lines); idx++ {
		depth += strings.Count(lines[idx], "{")
		depth -= strings.Count(lines[idx], "}")
		if depth <= 0 {
			return idx
		}
	}
	return len(lines) - 1
}

func rubyFunctionEnd(lines []string, start int) int {
	depth := 1
	for idx := start + 1; idx < len(lines); idx++ {
		trimmed := strings.TrimSpace(lines[idx])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if rubyBlockStart(trimmed) {
			depth++
		}
		if trimmed == "end" {
			depth--
			if depth == 0 {
				return idx
			}
		}
	}
	return len(lines) - 1
}

func rubyBlockStart(line string) bool {
	switch {
	case strings.HasPrefix(line, "def "),
		strings.HasPrefix(line, "class "),
		strings.HasPrefix(line, "module "),
		strings.HasPrefix(line, "if "),
		strings.HasPrefix(line, "unless "),
		strings.HasPrefix(line, "case "),
		strings.HasPrefix(line, "begin"),
		strings.HasPrefix(line, "for "),
		strings.HasPrefix(line, "while "),
		strings.HasPrefix(line, "until "):
		return true
	case strings.Contains(line, " do"):
		return true
	default:
		return false
	}
}

func rustParameterCount(signature string) int {
	if strings.TrimSpace(signature) == "" {
		return 0
	}
	count := 0
	for _, part := range strings.Split(signature, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "self" || part == "&self" || part == "&mut self" || strings.HasSuffix(part, " self") {
			continue
		}
		count++
	}
	return count
}

func typedParameterCount(signature string) int {
	if strings.TrimSpace(signature) == "" {
		return 0
	}
	count := 0
	for _, part := range strings.Split(signature, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		count++
	}
	return count
}

func rubyParameterCount(signature string) int {
	signature = strings.TrimSpace(signature)
	signature = strings.TrimPrefix(signature, "|")
	signature = strings.TrimSuffix(signature, "|")
	if signature == "" {
		return 0
	}
	count := 0
	for _, part := range strings.Split(signature, ",") {
		part = strings.TrimSpace(part)
		if part == "" || part == "&block" {
			continue
		}
		count++
	}
	return count
}
