package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func execute(raw []byte) ([]byte, error) {
	var req rpcExecutorRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	if bypassExecutorRequest(req.ExecutorRequest) {
		return errorEnvelope("executor_bypass", "reasoning-516-retry wrapper bypass"), nil
	}
	hostLog(req.HostCallbackID, "info", "reasoning-516-retry non-stream execute", map[string]any{
		"source_format": req.SourceFormat,
		"format":        req.Format,
		"model":         req.Model,
	})
	resp, errExecute := executeHostModel(req.ExecutorRequest, req.HostCallbackID)
	if errExecute != nil {
		hostLog(req.HostCallbackID, "warn", "reasoning-516-retry host execute failed", map[string]any{
			"model": req.Model,
			"error": errExecute.Error(),
		})
		return errorEnvelope("executor_error", errExecute.Error()), nil
	}
	status := resp.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	if status < 200 || status >= 300 {
		return okEnvelope(pluginapi.ExecutorResponse{Payload: resp.Body, Headers: cloneHeader(resp.Headers)})
	}
	if retryRequiredForReasoningTokens(resp.Body) {
		hostLog(req.HostCallbackID, "warn", "reasoning-516-retry detected reasoning_tokens=516", map[string]any{
			"model":  req.Model,
			"stream": false,
		})
		return errorEnvelope("retry_required_reasoning_516", retryRequiredReasoning516Message), nil
	}
	return okEnvelope(pluginapi.ExecutorResponse{Payload: resp.Body, Headers: cloneHeader(resp.Headers)})
}

func executeHostModel(exec pluginapi.ExecutorRequest, hostCallbackID string) (pluginapi.HostModelExecutionResponse, error) {
	result, errCall := callHost(pluginabi.MethodHostModelExecute, hostModelExecutionRequest{
		HostModelExecutionRequest: pluginapi.HostModelExecutionRequest{
			EntryProtocol: hostProtocol(exec),
			ExitProtocol:  hostProtocol(exec),
			Model:         strings.TrimSpace(exec.Model),
			Stream:        false,
			Body:          requestBody(exec),
			Headers:       hostModelHeaders(exec),
			Query:         exec.Query,
			Alt:           exec.Alt,
		},
		HostCallbackID: hostCallbackID,
	})
	if errCall != nil {
		return pluginapi.HostModelExecutionResponse{}, errCall
	}
	var resp pluginapi.HostModelExecutionResponse
	if errUnmarshal := json.Unmarshal(result, &resp); errUnmarshal != nil {
		return pluginapi.HostModelExecutionResponse{}, fmt.Errorf("decode host.model.execute result: %w", errUnmarshal)
	}
	return resp, nil
}
