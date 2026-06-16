package cli

import (
	"context"
	"encoding/json"
	"io"
	"strings"
)

func (s *mcpServer) handleToolsList(req mcpRequest, stdout io.Writer) error {
	if !s.isInitialized() {
		return s.responder.writeError(stdout, req.idPtr(), -32002, "server not initialized")
	}
	return s.responder.writeResult(stdout, req.ID, map[string]any{"tools": mcpTools()})
}

func (s *mcpServer) handleToolsCallRequest(req mcpRequest, stdout io.Writer) error {
	if !s.isInitialized() {
		return s.responder.writeError(stdout, req.idPtr(), -32002, "server not initialized")
	}
	if len(req.ID) == 0 {
		return s.responder.writeError(stdout, nil, -32600, "tools/call requires id")
	}
	return s.handleToolCall(req, stdout)
}

func (s *mcpServer) handleToolCall(req mcpRequest, stdout io.Writer) error {
	key, ok := requestKey(req.ID)
	if !ok {
		return s.responder.writeError(stdout, req.idPtr(), -32600, "invalid request id")
	}
	ctx, cancel := context.WithCancel(context.Background())
	progressToken := progressTokenFromParams(req.Params)

	s.mu.Lock()
	s.active[key] = cancel
	delete(s.cancelled, key)
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.finishRequest(key)
		if progressToken != nil {
			_ = s.responder.writeProgress(stdout, *progressToken, 0, 1, "Started")
		}
		result, err := s.tools.callToolWithContext(ctx, req.Params)
		if progressToken != nil {
			message := "Completed"
			if err != nil || s.isCancelled(key) {
				message = "Stopped"
			}
			_ = s.responder.writeProgress(stdout, *progressToken, 1, 1, message)
		}
		if s.isCancelled(key) {
			return
		}
		if err != nil {
			_ = s.responder.writeError(stdout, req.idPtr(), -32602, err.Error())
			return
		}
		_ = s.responder.writeResult(stdout, req.ID, result)
	}()
	return nil
}

func (s *mcpServer) handleCancelledNotification(raw json.RawMessage) {
	var params struct {
		RequestID json.RawMessage `json:"requestId"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return
	}
	key, ok := requestKey(params.RequestID)
	if !ok {
		return
	}

	s.mu.Lock()
	cancel, exists := s.active[key]
	if exists {
		s.cancelled[key] = true
	}
	s.mu.Unlock()
	if exists {
		cancel()
	}
}

func (s *mcpServer) finishRequest(key string) {
	s.mu.Lock()
	delete(s.active, key)
	delete(s.cancelled, key)
	s.mu.Unlock()
}

func (s *mcpServer) isCancelled(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancelled[key]
}

func (s *mcpServer) isInitialized() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.initializeSeen
}

func requestKey(raw json.RawMessage) (string, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", false
	}
	return trimmed, true
}

func progressTokenFromParams(raw json.RawMessage) *json.RawMessage {
	var params struct {
		Meta struct {
			ProgressToken json.RawMessage `json:"progressToken"`
		} `json:"_meta"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil
	}
	trimmed := strings.TrimSpace(string(params.Meta.ProgressToken))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	token := params.Meta.ProgressToken
	return &token
}
