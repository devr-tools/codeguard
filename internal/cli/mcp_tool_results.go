package cli

import "encoding/json"

func toolSuccessResult(payload any) map[string]any {
	text := mustJSON(payload)
	return map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": text,
		}},
		"structuredContent": payload,
		"isError":           false,
	}
}

func toolErrorResult(message string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": message,
		}},
		"isError": true,
	}
}

func toolErrorResultData(message string, data any) map[string]any {
	result := toolErrorResult(message)
	if data != nil {
		result["structuredContent"] = data
	}
	return result
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}
