package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (s *mcpToolService) callToolWithContext(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &call); err != nil {
		return nil, fmt.Errorf("invalid tool call")
	}

	handler, ok := toolHandlers(s)[strings.TrimSpace(call.Name)]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", call.Name)
	}
	return handler(ctx, normalizeMCPArguments(call.Arguments))
}

func toolHandlers(s *mcpToolService) map[string]func(context.Context, json.RawMessage) (map[string]any, error) {
	return map[string]func(context.Context, json.RawMessage) (map[string]any, error){
		"scan":            s.callScan,
		"validate_config": s.callValidateConfig,
		"validate_patch":  s.callValidatePatch,
		"explain":         s.callExplain,
		"list_rules":      s.callListRules,
		"verify_fix":      s.callVerifyFix,
		"propose_fix":     s.callProposeFix,
		"apply_fix":       s.callApplyFix,
	}
}
