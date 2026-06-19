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
	return s.writeMessage(stdout, buildResultMessage(id, result))
}

func (s *mcpResponder) writeError(stdout io.Writer, id *json.RawMessage, code int, message string) error {
	return s.writeMessage(stdout, buildErrorMessage(id, code, message))
}

func (s *mcpResponder) writeProgress(stdout io.Writer, token json.RawMessage, progress float64, total float64, message string) error {
	return s.writeMessage(stdout, buildProgressMessage(token, progress, total, message))
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
