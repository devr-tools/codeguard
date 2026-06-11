package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

func (s *mcpResponder) writeResult(stdout io.Writer, id json.RawMessage, result any) error {
	if len(id) == 0 {
		return nil
	}
	return s.writeMessage(stdout, map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"result":  result,
	})
}

func (s *mcpResponder) writeError(stdout io.Writer, id *json.RawMessage, code int, message string) error {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"error":   mcpError{Code: code, Message: message},
	}
	if id != nil {
		payload["id"] = json.RawMessage(*id)
	} else {
		payload["id"] = nil
	}
	return s.writeMessage(stdout, payload)
}

func (s *mcpResponder) writeProgress(stdout io.Writer, token json.RawMessage, progress float64, total float64, message string) error {
	return s.writeMessage(stdout, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/progress",
		"params": map[string]any{
			"progressToken": json.RawMessage(token),
			"progress":      progress,
			"total":         total,
			"message":       message,
		},
	})
}

func (s *mcpResponder) writeMessage(stdout io.Writer, payload any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, string(data))
	return err
}

func (req mcpRequest) idPtr() *json.RawMessage {
	if len(req.ID) == 0 {
		return nil
	}
	id := req.ID
	return &id
}
