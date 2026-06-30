package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

type streamForwardState struct {
	pendingCompleted []byte
}

func executeStream(raw []byte) ([]byte, error) {
	var req rpcExecutorRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	streamID := strings.TrimSpace(req.StreamID)
	if streamID == "" {
		return errorEnvelope("executor_error", "stream_id is required for executor.execute_stream"), nil
	}
	hostLog(req.HostCallbackID, "info", "reasoning-516-retry stream execute", map[string]any{
		"source_format": req.SourceFormat,
		"format":        req.Format,
		"model":         req.Model,
		"stream_id":     streamID,
	})
	go func() {
		errRun := forwardHostModelStream(req.ExecutorRequest, req.HostCallbackID, streamID)
		if errRun != nil {
			hostLog(req.HostCallbackID, "warn", "reasoning-516-retry stream failed", map[string]any{
				"model":     req.Model,
				"stream_id": streamID,
				"error":     errRun.Error(),
			})
			closePluginStream(streamID, errRun.Error())
			return
		}
		closePluginStream(streamID, "")
	}()
	return okEnvelope(map[string]any{
		"headers": http.Header{"Content-Type": []string{"text/event-stream"}},
	})
}

func forwardHostModelStream(exec pluginapi.ExecutorRequest, hostCallbackID, pluginStreamID string) error {
	resp, errStart := startHostModelStream(exec, hostCallbackID)
	if errStart != nil {
		return errStart
	}
	if resp.StatusCode >= 400 {
		_ = closeHostModelStream(resp.StreamID)
		return fmt.Errorf("host model stream returned status %d", resp.StatusCode)
	}
	if strings.TrimSpace(resp.StreamID) == "" {
		return fmt.Errorf("host model stream: empty stream_id")
	}
	defer func() { _ = closeHostModelStream(resp.StreamID) }()

	state := streamForwardState{}
	for {
		chunk, errRead := readHostModelStream(resp.StreamID)
		if errRead != nil {
			return errRead
		}
		if chunk.Error != "" {
			return fmt.Errorf("%s", chunk.Error)
		}
		if len(chunk.Payload) > 0 {
			payload, emit, terminal := state.nextPayload(chunk.Payload, exec.Model)
			if emit {
				if errEmit := emitPluginStreamChunk(pluginStreamID, payload); errEmit != nil {
					return errEmit
				}
			}
			if terminal {
				hostLog(hostCallbackID, "warn", "reasoning-516-retry emitted single response.failed for reasoning_tokens=516", map[string]any{
					"model":  exec.Model,
					"stream": true,
				})
				return nil
			}
		}
		if chunk.Done {
			if pending := state.flush(); len(pending) > 0 {
				if errEmit := emitPluginStreamChunk(pluginStreamID, pending); errEmit != nil {
					return errEmit
				}
			}
			return nil
		}
	}
}

func (s *streamForwardState) nextPayload(payload []byte, fallbackModel string) ([]byte, bool, bool) {
	body := append([]byte(nil), payload...)
	if len(s.pendingCompleted) > 0 {
		body = joinStreamChunks(s.pendingCompleted, body)
		s.pendingCompleted = nil
		if !streamFrameReady(body) {
			s.pendingCompleted = body
			return nil, false, false
		}
	}
	if isCompletedEventOnlyChunk(body) {
		s.pendingCompleted = append([]byte(nil), body...)
		return nil, false, false
	}
	if retryRequiredForReasoningTokens(body) {
		return retryResponseFailedSSE(body, fallbackModel), true, true
	}
	return body, true, false
}

func (s *streamForwardState) flush() []byte {
	out := s.pendingCompleted
	s.pendingCompleted = nil
	return out
}

func startHostModelStream(exec pluginapi.ExecutorRequest, hostCallbackID string) (pluginapi.HostModelStreamResponse, error) {
	result, errCall := callHost(pluginabi.MethodHostModelExecuteStream, hostModelExecutionRequest{
		HostModelExecutionRequest: pluginapi.HostModelExecutionRequest{
			EntryProtocol: hostProtocol(exec),
			ExitProtocol:  hostProtocol(exec),
			Model:         strings.TrimSpace(exec.Model),
			Stream:        true,
			Body:          requestBody(exec),
			Headers:       cloneHeader(exec.Headers),
			Query:         exec.Query,
			Alt:           exec.Alt,
		},
		HostCallbackID: hostCallbackID,
	})
	if errCall != nil {
		return pluginapi.HostModelStreamResponse{}, errCall
	}
	var resp pluginapi.HostModelStreamResponse
	if errUnmarshal := json.Unmarshal(result, &resp); errUnmarshal != nil {
		return pluginapi.HostModelStreamResponse{}, fmt.Errorf("decode host.model.execute_stream result: %w", errUnmarshal)
	}
	return resp, nil
}

func readHostModelStream(streamID string) (pluginapi.HostModelStreamReadResponse, error) {
	result, errCall := callHost(pluginabi.MethodHostModelStreamRead, pluginapi.HostModelStreamReadRequest{StreamID: streamID})
	if errCall != nil {
		return pluginapi.HostModelStreamReadResponse{}, errCall
	}
	var chunk pluginapi.HostModelStreamReadResponse
	if errUnmarshal := json.Unmarshal(result, &chunk); errUnmarshal != nil {
		return pluginapi.HostModelStreamReadResponse{}, fmt.Errorf("decode host.model.stream_read result: %w", errUnmarshal)
	}
	return chunk, nil
}

func closeHostModelStream(streamID string) error {
	if strings.TrimSpace(streamID) == "" {
		return nil
	}
	_, errCall := callHost(pluginabi.MethodHostModelStreamClose, pluginapi.HostModelStreamCloseRequest{StreamID: streamID})
	return errCall
}

func emitPluginStreamChunk(streamID string, payload []byte) error {
	if strings.TrimSpace(streamID) == "" {
		return fmt.Errorf("plugin stream id is required")
	}
	_, errCall := callHost(pluginabi.MethodHostStreamEmit, rpcStreamEmitRequest{
		StreamID: streamID,
		Payload:  payload,
	})
	return errCall
}

func closePluginStream(streamID, errMsg string) {
	if strings.TrimSpace(streamID) == "" {
		return
	}
	_, _ = callHost(pluginabi.MethodHostStreamClose, rpcStreamCloseRequest{
		StreamID: streamID,
		Error:    strings.TrimSpace(errMsg),
	})
}

func isCompletedEventOnlyChunk(payload []byte) bool {
	event := ""
	hasData := false
	for _, line := range bytes.Split(payload, []byte{'\n'}) {
		line = bytes.TrimSpace(bytes.TrimRight(line, "\r"))
		if len(line) == 0 {
			continue
		}
		if bytes.HasPrefix(line, []byte("event:")) {
			event = strings.TrimSpace(string(bytes.TrimPrefix(line, []byte("event:"))))
			continue
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			hasData = true
		}
	}
	return event == "response.completed" && !hasData
}

func streamFrameReady(payload []byte) bool {
	if !bytes.Contains(payload, []byte("data:")) {
		return false
	}
	for _, raw := range responseJSONPayloads(payload) {
		if json.Valid(raw) {
			return true
		}
	}
	return false
}

func joinStreamChunks(left, right []byte) []byte {
	if len(left) == 0 {
		return append([]byte(nil), right...)
	}
	if len(right) == 0 {
		return append([]byte(nil), left...)
	}
	out := make([]byte, 0, len(left)+len(right)+1)
	out = append(out, left...)
	if streamChunksNeedLineBreak(out, right) {
		out = append(out, '\n')
	}
	out = append(out, right...)
	return out
}

func streamChunksNeedLineBreak(left, right []byte) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	if bytes.HasSuffix(left, []byte("\n")) || bytes.HasSuffix(left, []byte("\r")) || right[0] == '\n' || right[0] == '\r' {
		return false
	}
	trimmed := bytes.TrimLeft(right, " \t")
	for _, prefix := range [][]byte{[]byte("data:"), []byte("event:"), []byte("id:"), []byte("retry:"), []byte(":")} {
		if bytes.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}
