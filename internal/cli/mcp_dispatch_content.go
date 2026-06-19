package cli

import "encoding/json"

func (s *mcpToolService) dispatchResourceMethod(method string, id json.RawMessage, params json.RawMessage) (map[string]any, bool) {
	switch method {
	case "resources/list":
		return buildResultMessage(id, map[string]any{"resources": s.resourcesList()}), true
	case "resources/templates/list":
		return buildResultMessage(id, map[string]any{"resourceTemplates": resourceTemplates()}), true
	case "resources/read":
		return resourceReadMessage(s, id, params), true
	default:
		return nil, false
	}
}

func dispatchPromptMethod(method string, id json.RawMessage, params json.RawMessage) (map[string]any, bool) {
	switch method {
	case "prompts/list":
		return buildResultMessage(id, map[string]any{"prompts": mcpPrompts()}), true
	case "prompts/get":
		return promptGetMessage(id, params), true
	default:
		return nil, false
	}
}

func resourceReadMessage(s *mcpToolService, id json.RawMessage, params json.RawMessage) map[string]any {
	result, errMsg := s.readResource(params)
	if errMsg != "" {
		return buildErrorMessage(ptrID(id), -32602, errMsg)
	}
	return buildResultMessage(id, result)
}

func promptGetMessage(id json.RawMessage, params json.RawMessage) map[string]any {
	result, errMsg := getPrompt(params)
	if errMsg != "" {
		return buildErrorMessage(ptrID(id), -32602, errMsg)
	}
	return buildResultMessage(id, result)
}
