package cli

import (
	"encoding/json"
	"strings"
)

func negotiateMCPProtocolVersion(raw json.RawMessage) string {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return mcpProtocolVersionCompat
	}
	switch strings.TrimSpace(params.ProtocolVersion) {
	case mcpProtocolVersionCurrent, mcpProtocolVersionCompat:
		return params.ProtocolVersion
	default:
		return mcpProtocolVersionCompat
	}
}

func normalizeMCPArguments(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return json.RawMessage([]byte("{}"))
	}
	return raw
}
