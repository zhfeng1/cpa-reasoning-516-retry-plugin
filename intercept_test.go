package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func TestInterceptStreamChunkDropsReasoning516CompletedChunk(t *testing.T) {
	enableStreamInterceptorFallbackForTest(t)
	rawReq, errMarshal := json.Marshal(rpcStreamChunkInterceptRequest{
		StreamChunkInterceptRequest: pluginapi.StreamChunkInterceptRequest{
			SourceFormat: "openai-response",
			Model:        "gpt-5.1",
			ChunkIndex:   42,
			Body: []byte("event: response.completed\n" +
				`data: {"type":"response.completed","response":{"id":"resp_1"},"usage":{"output_tokens_details":{"reasoning_tokens":516}}}` + "\n\n"),
		},
	})
	if errMarshal != nil {
		t.Fatalf("marshal request: %v", errMarshal)
	}

	rawResp, errIntercept := interceptStreamChunk(rawReq)
	if errIntercept != nil {
		t.Fatalf("interceptStreamChunk() error = %v", errIntercept)
	}
	var env envelope
	if errDecode := json.Unmarshal(rawResp, &env); errDecode != nil {
		t.Fatalf("decode envelope: %v", errDecode)
	}
	if !env.OK {
		t.Fatalf("envelope error = %#v", env.Error)
	}
	var resp pluginapi.StreamChunkInterceptResponse
	if errDecode := json.Unmarshal(env.Result, &resp); errDecode != nil {
		t.Fatalf("decode response: %v", errDecode)
	}
	if resp.DropChunk {
		t.Fatal("DropChunk = true, want false")
	}
	if !bytes.Contains(resp.Body, []byte("event: response.failed")) {
		t.Fatalf("response body missing response.failed event: %s", resp.Body)
	}
	if !bytes.Contains(resp.Body, []byte(retryRequiredReasoning516Message)) {
		t.Fatalf("response body missing retry message: %s", resp.Body)
	}
}

func TestInterceptStreamChunkDropsChunksAfterRetryFailure(t *testing.T) {
	enableStreamInterceptorFallbackForTest(t)
	req := pluginapi.StreamChunkInterceptRequest{
		SourceFormat: "openai-response",
		Model:        "gpt-5.5",
		HistoryChunks: [][]byte{
			[]byte("event: response.failed\ndata: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"RETRY_REQUIRED_REASONING_516\"}}}\n\n"),
		},
		Body: []byte("event: response.failed\n" +
			`data: {"type":"response.failed","response":{"error":{"code":"upstream_error","message":"Upstream request failed"}}}` + "\n\n"),
	}

	resp := decodeStreamInterceptResponse(t, req)
	if !resp.DropChunk {
		t.Fatal("DropChunk = false, want true")
	}
	if len(resp.Body) != 0 {
		t.Fatalf("body = %q, want empty", resp.Body)
	}
}

func TestInterceptStreamChunkPassesSplitCompletedEventWithoutBuffering(t *testing.T) {
	enableStreamInterceptorFallbackForTest(t)
	resp := decodeStreamInterceptResponse(t, pluginapi.StreamChunkInterceptRequest{
		SourceFormat: "openai-response",
		Model:        "gpt-5.5",
		Body:         []byte("event: response.completed"),
	})
	if resp.DropChunk {
		t.Fatal("DropChunk = true, want false")
	}
	if string(resp.Body) != "event: response.completed" {
		t.Fatalf("body = %q, want completed event passthrough", resp.Body)
	}
}

func TestInterceptStreamChunkFallbackDisabledByDefault(t *testing.T) {
	currentConfig.Store(defaultPluginConfig())
	resp := decodeStreamInterceptResponse(t, pluginapi.StreamChunkInterceptRequest{
		SourceFormat: "openai-response",
		Model:        "gpt-5.5",
		Body: []byte("event: response.completed\n" +
			`data: {"type":"response.completed","response":{"usage":{"output_tokens_details":{"reasoning_tokens":516}}}}` + "\n\n"),
	})
	if resp.DropChunk {
		t.Fatal("DropChunk = true, want false")
	}
	if bytes.Contains(resp.Body, []byte("RETRY_REQUIRED_REASONING_516")) {
		t.Fatalf("body contains retry marker with fallback disabled: %s", resp.Body)
	}
}

func enableStreamInterceptorFallbackForTest(t *testing.T) {
	t.Helper()
	cfg := defaultPluginConfig()
	cfg.StreamInterceptorFallback = true
	currentConfig.Store(cfg)
	t.Cleanup(func() {
		currentConfig.Store(defaultPluginConfig())
	})
}

func decodeStreamInterceptResponse(t *testing.T, req pluginapi.StreamChunkInterceptRequest) pluginapi.StreamChunkInterceptResponse {
	t.Helper()
	rawReq, errMarshal := json.Marshal(rpcStreamChunkInterceptRequest{StreamChunkInterceptRequest: req})
	if errMarshal != nil {
		t.Fatalf("marshal request: %v", errMarshal)
	}
	rawResp, errIntercept := interceptStreamChunk(rawReq)
	if errIntercept != nil {
		t.Fatalf("interceptStreamChunk() error = %v", errIntercept)
	}
	var env envelope
	if errDecode := json.Unmarshal(rawResp, &env); errDecode != nil {
		t.Fatalf("decode envelope: %v", errDecode)
	}
	if !env.OK {
		t.Fatalf("envelope error = %#v", env.Error)
	}
	var resp pluginapi.StreamChunkInterceptResponse
	if errDecode := json.Unmarshal(env.Result, &resp); errDecode != nil {
		t.Fatalf("decode response: %v", errDecode)
	}
	return resp
}
