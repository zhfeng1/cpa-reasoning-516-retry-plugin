package main

import (
	"encoding/json"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func interceptResponse(raw []byte) ([]byte, error) {
	var req rpcResponseInterceptRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	if retryRequiredForReasoningTokens(req.Body) {
		hostLog(req.HostCallbackID, "warn", "reasoning-516-retry response interceptor detected reasoning_tokens=516", map[string]any{
			"model":           req.Model,
			"requested_model": req.RequestedModel,
			"source_format":   req.SourceFormat,
			"stream":          false,
		})
		return okEnvelope(pluginapi.ResponseInterceptResponse{Body: retryResponseBody()})
	}
	return okEnvelope(pluginapi.ResponseInterceptResponse{Body: req.Body})
}

func interceptStreamChunk(raw []byte) ([]byte, error) {
	var req rpcStreamChunkInterceptRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	if retryRequiredForReasoningTokens(req.Body) {
		hostLog(req.HostCallbackID, "warn", "reasoning-516-retry stream interceptor detected reasoning_tokens=516", map[string]any{
			"model":           req.Model,
			"requested_model": req.RequestedModel,
			"source_format":   req.SourceFormat,
			"chunk_index":     req.ChunkIndex,
			"history_chunks":  len(req.HistoryChunks),
			"stream":          true,
		})
		return okEnvelope(pluginapi.StreamChunkInterceptResponse{DropChunk: true})
	}
	return okEnvelope(pluginapi.StreamChunkInterceptResponse{Body: req.Body})
}

func retryResponseBody() []byte {
	return []byte(`{"error":{"message":"` + retryRequiredReasoning516Message + `","type":"retry_required_reasoning_516","code":"RETRY_REQUIRED_REASONING_516"}}`)
}
