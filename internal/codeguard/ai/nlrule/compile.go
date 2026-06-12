package nlrule

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Compile(rule core.CustomRuleConfig, path string, data []byte) EvaluationRequest {
	content := string(data)
	truncated := false
	if len(content) > maxSourceBytes {
		content = content[:maxSourceBytes]
		truncated = true
	}
	numbered := lineNumberedContent(content)
	return EvaluationRequest{
		Version: promptVersion,
		Rule: RuleSpec{
			ID:          rule.ID,
			Title:       rule.Title,
			Description: strings.TrimSpace(rule.Description),
			Message:     rule.Message,
			Instruction: strings.TrimSpace(rule.NaturalLanguage),
		},
		File: FileSpec{
			Path:      filepath.ToSlash(path),
			Content:   numbered,
			Truncated: truncated,
		},
		Prompt: buildPrompt(rule, filepath.ToSlash(path), numbered, truncated),
	}
}

func buildPrompt(rule core.CustomRuleConfig, path string, numbered string, truncated bool) string {
	var builder strings.Builder
	builder.WriteString("Evaluate this repository policy against one file.\n")
	builder.WriteString("Return JSON only with the shape {\"matches\":[{\"line\":number,\"column\":number,\"message\":string,\"rationale\":string}]}.\n")
	builder.WriteString("Return an empty matches array when the file does not violate the policy.\n")
	builder.WriteString("Use 1-based line numbers from the numbered source below.\n")
	builder.WriteString("Do not report speculative violations.\n")
	builder.WriteString("Policy: ")
	builder.WriteString(strings.TrimSpace(rule.NaturalLanguage))
	builder.WriteString("\n")
	builder.WriteString("Default finding message: ")
	builder.WriteString(rule.Message)
	builder.WriteString("\n")
	builder.WriteString("File: ")
	builder.WriteString(path)
	builder.WriteString("\n")
	if truncated {
		builder.WriteString("Note: source was truncated to the first 65536 bytes.\n")
	}
	builder.WriteString("Numbered source:\n")
	builder.WriteString(numbered)
	return builder.String()
}

func lineNumberedContent(source string) string {
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	var builder strings.Builder
	for idx, line := range lines {
		builder.WriteString(strconv.Itoa(idx + 1))
		builder.WriteString(": ")
		builder.WriteString(line)
		if idx < len(lines)-1 {
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}
