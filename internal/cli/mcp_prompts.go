package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MCP prompts ship reusable workflows that steer an agent toward the codeguard
// tools for common review tasks. Each prompt renders to one or more user
// messages that name the tool to call and the arguments to pass.

type mcpPrompt struct {
	Name        string              `json:"name"`
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Arguments   []mcpPromptArgument `json:"arguments,omitempty"`
}

type mcpPromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

func mcpPrompts() []mcpPrompt {
	return []mcpPrompt{
		{
			Name:        "review-diff",
			Title:       "Review a diff against policy",
			Description: "Validate a proposed unified diff with the validate_patch tool before writing it to disk.",
			Arguments: []mcpPromptArgument{
				{Name: "diff", Description: "The unified diff to validate.", Required: true},
			},
		},
		{
			Name:        "triage-findings",
			Title:       "Triage repository findings",
			Description: "Run a full scan and summarize the failing sections with concrete next steps.",
			Arguments:   []mcpPromptArgument{},
		},
		{
			Name:        "explain-rule",
			Title:       "Explain a rule",
			Description: "Fetch machine-first explanation metadata for a rule and restate how to fix it.",
			Arguments: []mcpPromptArgument{
				{Name: "rule_id", Description: "The rule id to explain (e.g. security.hardcoded-secret).", Required: true},
			},
		},
	}
}

// getPrompt resolves a prompts/get request. The second return value is a
// non-empty error message when the prompt could not be rendered.
func getPrompt(params json.RawMessage) (map[string]any, string) {
	var args struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, "invalid prompts/get params"
	}

	switch strings.TrimSpace(args.Name) {
	case "review-diff":
		diff := strings.TrimSpace(args.Arguments["diff"])
		if diff == "" {
			return nil, "prompt review-diff requires a diff argument"
		}
		return promptResult(
			"Validate a proposed diff against codeguard policy.",
			fmt.Sprintf("Call the validate_patch tool with this unified diff and report any failing findings before applying it. Do not write the diff to disk if validation fails.\n\n```diff\n%s\n```", diff),
		), ""
	case "triage-findings":
		return promptResult(
			"Triage codeguard findings for the repository.",
			"Call the scan tool with mode \"full\". Group the resulting findings by section, list every failing section first, and for each failing finding give the rule id, the file:line, and a one-line fix suggestion drawn from the finding's how_to_fix field.",
		), ""
	case "explain-rule":
		ruleID := strings.TrimSpace(args.Arguments["rule_id"])
		if ruleID == "" {
			return nil, "prompt explain-rule requires a rule_id argument"
		}
		return promptResult(
			fmt.Sprintf("Explain the %s rule.", ruleID),
			fmt.Sprintf("Call the explain tool with rule_id %q (or read the codeguard://rules/%s resource). Summarize what the rule checks, why it matters, and the concrete steps to fix a violation.", ruleID, ruleID),
		), ""
	default:
		return nil, fmt.Sprintf("unknown prompt %q", args.Name)
	}
}

// promptResult builds an MCP prompts/get result with a single user message.
func promptResult(description string, text string) map[string]any {
	return map[string]any{
		"description": description,
		"messages": []map[string]any{{
			"role": "user",
			"content": map[string]any{
				"type": "text",
				"text": text,
			},
		}},
	}
}
