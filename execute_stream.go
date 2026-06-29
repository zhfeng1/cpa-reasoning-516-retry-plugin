package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func executeStream(raw []byte) ([]byte, error) {
	var req rpcExecutorRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	streamID := strings.TrimSpace(req.StreamID)
	if streamID == "" {
		return errorEnvelope("executor_error", "stream_id is required for executor.execute_stream"), nil
	}
	go func() {
		errRun := forwardHostModelStream(req.ExecutorRequest, req.HostCallbackID, streamID)
		if errRun != nil {
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

	for {
		chunk, errRead := readHostModelStream(resp.StreamID)
		if errRead != nil {
			return errRead
		}
		if chunk.Error != "" {
			return fmt.Errorf("%s", chunk.Error)
		}
		if len(chunk.Payload) > 0 {
			if retryRequiredForReasoningTokens(chunk.Payload) {
				return fmt.Errorf("%s", retryRequiredReasoning516Message)
			}
			if errEmit := emitPluginStreamChunk(pluginStreamID, chunk.Payload); errEmit != nil {
				return errEmit
			}
		}
		if chunk.Done {
			return nil
		}
	}
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
