package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// MCP resources expose codeguard's static, read-only knowledge (the rule
// catalog and the active configuration) so agents can pull context without a
// tool round-trip. They wrap the same SDK accessors used by the tools.

const (
	resourceURIRules      = "codeguard://rules"
	resourceURIConfig     = "codeguard://config"
	resourceRulePrefix    = "codeguard://rules/"
	resourceRuleURITmpl   = "codeguard://rules/{rule_id}"
	resourceMIMEType      = "application/json"
	resourceContentsField = "contents"
)

type mcpResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

type mcpResourceTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

// resourcesList returns the concrete (non-templated) resources.
func (s *mcpToolService) resourcesList() []mcpResource {
	return []mcpResource{
		{
			URI:         resourceURIRules,
			Name:        "rules",
			Title:       "Rule Catalog",
			Description: "The full codeguard rule catalog that applies to the active configuration.",
			MIMEType:    resourceMIMEType,
		},
		{
			URI:         resourceURIConfig,
			Name:        "config",
			Title:       "Active Configuration",
			Description: "The resolved codeguard policy configuration the server was started with.",
			MIMEType:    resourceMIMEType,
		},
	}
}

// resourceTemplates returns the parameterized resources.
func resourceTemplates() []mcpResourceTemplate {
	return []mcpResourceTemplate{
		{
			URITemplate: resourceRuleURITmpl,
			Name:        "rule",
			Title:       "Rule Explanation",
			Description: "Machine-first explanation metadata for a single rule, addressed by rule id.",
			MIMEType:    resourceMIMEType,
		},
	}
}

// readResource resolves a resources/read request. The second return value is a
// non-empty error message when the resource could not be served.
func (s *mcpToolService) readResource(params json.RawMessage) (map[string]any, string) {
	var args struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, "invalid resources/read params"
	}
	uri := strings.TrimSpace(args.URI)
	switch {
	case uri == resourceURIRules:
		return resourceResult(uri, map[string]any{"rules": s.rulesForDefaults()}), ""
	case uri == resourceURIConfig:
		cfg, err := s.loadConfig("", "")
		if err != nil {
			return nil, fmt.Sprintf("load config: %v", err)
		}
		return resourceResult(uri, cfg), ""
	case strings.HasPrefix(uri, resourceRulePrefix):
		ruleID := strings.TrimPrefix(uri, resourceRulePrefix)
		if strings.TrimSpace(ruleID) == "" {
			return nil, "resource uri missing rule id"
		}
		rule, ok, err := s.resolveExplainRule(s.defaultConfigPath, s.defaultProfile, ruleID)
		if err != nil {
			return nil, fmt.Sprintf("load config: %v", err)
		}
		if !ok {
			return nil, fmt.Sprintf("unknown rule %q", ruleID)
		}
		return resourceResult(uri, buildExplainAgentOutput(rule)), ""
	default:
		return nil, fmt.Sprintf("unknown resource %q", uri)
	}
}

// rulesForDefaults returns the rule catalog scoped to the server's default
// config when it loads, falling back to the built-in catalog otherwise.
func (s *mcpToolService) rulesForDefaults() []service.RuleMetadata {
	if strings.TrimSpace(s.defaultConfigPath) == "" {
		return service.Rules()
	}
	cfg, err := s.loadConfig("", "")
	if err != nil {
		return service.Rules()
	}
	return service.RulesForConfig(cfg)
}

// resourceResult builds an MCP resources/read result with a single JSON text
// content block.
func resourceResult(uri string, payload any) map[string]any {
	return map[string]any{
		resourceContentsField: []map[string]any{{
			"uri":      uri,
			"mimeType": resourceMIMEType,
			"text":     mustJSON(payload),
		}},
	}
}
