package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

const streamStateTTL = 10 * time.Minute

type streamStateEntry struct {
	pendingCompletedEvent []byte
	updatedAt             time.Time
}

var streamState = struct {
	sync.Mutex
	entries map[string]streamStateEntry
}{
	entries: make(map[string]streamStateEntry),
}

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
	if streamHistoryHasRetryFailure(req.HistoryChunks) {
		return okEnvelope(pluginapi.StreamChunkInterceptResponse{DropChunk: true})
	}
	streamKey := streamStateKey(req)
	if isCompletedEventOnlyChunk(req.Body) {
		storePendingCompletedEvent(streamKey, req.Body)
		return okEnvelope(pluginapi.StreamChunkInterceptResponse{DropChunk: true})
	}
	body := req.Body
	if pending := takePendingCompletedEvent(streamKey); len(pending) > 0 {
		body = joinStreamChunks(pending, req.Body)
		if !streamFrameReady(body) {
			storePendingCompletedEvent(streamKey, body)
			return okEnvelope(pluginapi.StreamChunkInterceptResponse{DropChunk: true})
		}
	}
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

func streamHistoryHasRetryFailure(history [][]byte) bool {
	for i := len(history) - 1; i >= 0; i-- {
		if bytes.Contains(history[i], []byte("RETRY_REQUIRED_REASONING_516")) {
			return true
		}
	}
	return false
}

func storePendingCompletedEvent(key string, payload []byte) {
	if key == "" {
		return
	}
	now := time.Now()
	streamState.Lock()
	defer streamState.Unlock()
	cleanupStreamStateLocked(now)
	streamState.entries[key] = streamStateEntry{
		pendingCompletedEvent: append([]byte(nil), payload...),
		updatedAt:             now,
	}
}

func takePendingCompletedEvent(key string) []byte {
	if key == "" {
		return nil
	}
	now := time.Now()
	streamState.Lock()
	defer streamState.Unlock()
	cleanupStreamStateLocked(now)
	entry, ok := streamState.entries[key]
	if !ok {
		return nil
	}
	delete(streamState.entries, key)
	return append([]byte(nil), entry.pendingCompletedEvent...)
}

func cleanupStreamStateLocked(now time.Time) {
	for key, entry := range streamState.entries {
		if now.Sub(entry.updatedAt) > streamStateTTL {
			delete(streamState.entries, key)
		}
	}
}

func streamStateKey(req rpcStreamChunkInterceptRequest) string {
	payload := req.OriginalRequest
	if len(payload) == 0 {
		payload = req.RequestBody
	}
	sum := sha256.Sum256(payload)
	return strings.Join([]string{
		req.SourceFormat,
		req.Model,
		req.RequestedModel,
		hex.EncodeToString(sum[:16]),
	}, "|")
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
