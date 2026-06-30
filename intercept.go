package main

import (
	"bytes"
	"encoding/json"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func interceptResponse(raw []byte) ([]byte, error) {
	var req rpcResponseInterceptRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	if !shouldIntercept(req.SourceFormat, req.RequestedModel, req.Model) {
		return okEnvelope(pluginapi.ResponseInterceptResponse{Body: req.Body})
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
	if !shouldInterceptStreamChunk(req.SourceFormat, req.RequestedModel, req.Model) {
		return okEnvelope(pluginapi.StreamChunkInterceptResponse{Body: req.Body})
	}
	if isRetryFollowupError(req.Body) && streamHistoryHasRetryFailure(req.HistoryChunks) {
		return okEnvelope(pluginapi.StreamChunkInterceptResponse{DropChunk: true})
	}
	body := req.Body
	if retryRequiredForReasoningTokens(body) {
		hostLog(req.HostCallbackID, "warn", "reasoning-516-retry stream interceptor detected reasoning_tokens=516", map[string]any{
			"model":           req.Model,
			"requested_model": req.RequestedModel,
			"source_format":   req.SourceFormat,
			"chunk_index":     req.ChunkIndex,
			"history_chunks":  len(req.HistoryChunks),
			"stream":          true,
		})
		return okEnvelope(pluginapi.StreamChunkInterceptResponse{Body: retryResponseFailedSSE(body, firstNonEmpty(req.Model, req.RequestedModel))})
	}
	return okEnvelope(pluginapi.StreamChunkInterceptResponse{Body: body})
}

func retryResponseBody() []byte {
	return []byte(`{"error":{"message":"` + retryRequiredReasoning516Message + `","type":"retry_required_reasoning_516","code":"RETRY_REQUIRED_REASONING_516"}}`)
}

func retryResponseFailedSSE(payload []byte, fallbackModel string) []byte {
	id, object, model := responseIdentity(payload)
	if id == "" {
		id = "resp_retry_required_reasoning_516"
	}
	if object == "" {
		object = "response"
	}
	if model == "" {
		model = fallbackModel
	}
	event := map[string]any{
		"type": "response.failed",
		"response": map[string]any{
			"id":     id,
			"object": object,
			"model":  model,
			"status": "failed",
			"output": []any{},
			"error": map[string]any{
				"code":    "RETRY_REQUIRED_REASONING_516",
				"message": retryRequiredReasoning516Message,
			},
		},
	}
	raw, errMarshal := json.Marshal(event)
	if errMarshal != nil {
		raw = []byte(`{"type":"response.failed","response":{"id":"resp_retry_required_reasoning_516","object":"response","status":"failed","output":[],"error":{"code":"RETRY_REQUIRED_REASONING_516","message":"` + retryRequiredReasoning516Message + `"}}}`)
	}
	return bytes.Join([][]byte{
		[]byte("event: response.failed"),
		append([]byte("data: "), raw...),
		[]byte(""),
		[]byte(""),
	}, []byte("\n"))
}

func responseIdentity(payload []byte) (string, string, string) {
	for _, raw := range responseJSONPayloads(payload) {
		var value any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			continue
		}
		id := stringAtPath(value, []string{"response", "id"})
		object := stringAtPath(value, []string{"response", "object"})
		model := stringAtPath(value, []string{"response", "model"})
		if model == "" {
			model = stringAtPath(value, []string{"model"})
		}
		if id != "" || object != "" || model != "" {
			return id, object, model
		}
	}
	return "", "", ""
}

func stringAtPath(value any, path []string) string {
	current := value
	for _, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = obj[key]
		if !ok {
			return ""
		}
	}
	if text, ok := current.(string); ok {
		return text
	}
	return ""
}

func isRetryFollowupError(payload []byte) bool {
	return bytes.Contains(payload, []byte("response.failed")) ||
		bytes.Contains(payload, []byte("upstream_error")) ||
		bytes.Contains(payload, []byte("Upstream request failed")) ||
		bytes.Contains(payload, []byte(`"type":"error"`))
}

func streamHistoryHasRetryFailure(history [][]byte) bool {
	for i := len(history) - 1; i >= 0; i-- {
		if bytes.Contains(history[i], []byte("RETRY_REQUIRED_REASONING_516")) {
			return true
		}
	}
	return false
}

func shouldIntercept(sourceFormat, requestedModel, model string) bool {
	return shouldHandle(loadedConfig(), sourceFormat, firstNonEmpty(requestedModel, model))
}

func shouldInterceptStreamChunk(sourceFormat, requestedModel, model string) bool {
	cfg := loadedConfig()
	return cfg.StreamInterceptorFallback && shouldHandle(cfg, sourceFormat, firstNonEmpty(requestedModel, model))
}
