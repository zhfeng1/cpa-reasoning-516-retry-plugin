package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func TestInterceptStreamChunkDropsReasoning516CompletedChunk(t *testing.T) {
	clearTestStreamState()
	rawReq, errMarshal := json.Marshal(rpcStreamChunkInterceptRequest{
		StreamChunkInterceptRequest: pluginapi.StreamChunkInterceptRequest{
			Model:      "gpt-5.1",
			ChunkIndex: 42,
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

func TestInterceptStreamChunkBuffersSplitCompletedBeforeReasoning516(t *testing.T) {
	clearTestStreamState()
	baseReq := pluginapi.StreamChunkInterceptRequest{
		SourceFormat:    "openai-response",
		Model:           "gpt-5.5",
		RequestedModel:  "gpt-5.5",
		OriginalRequest: []byte(`{"model":"gpt-5.5","input":"test"}`),
	}
	first := baseReq
	first.ChunkIndex = 172
	first.Body = []byte("event: response.completed")

	firstResp := decodeStreamInterceptResponse(t, first)
	if !firstResp.DropChunk {
		t.Fatal("first split completed chunk DropChunk = false, want true")
	}
	if len(firstResp.Body) != 0 {
		t.Fatalf("first split completed chunk body = %q, want empty", firstResp.Body)
	}

	second := baseReq
	second.ChunkIndex = 173
	second.Body = []byte(`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","model":"gpt-5.5","usage":{"output_tokens_details":{"reasoning_tokens":516}}}}`)

	secondResp := decodeStreamInterceptResponse(t, second)
	if secondResp.DropChunk {
		t.Fatal("second split completed chunk DropChunk = true, want false")
	}
	if bytes.Contains(secondResp.Body, []byte("event: response.completed")) {
		t.Fatalf("response body should not contain completed event: %s", secondResp.Body)
	}
	if !bytes.Contains(secondResp.Body, []byte("event: response.failed")) {
		t.Fatalf("response body missing response.failed event: %s", secondResp.Body)
	}
	if !bytes.Contains(secondResp.Body, []byte(retryRequiredReasoning516Message)) {
		t.Fatalf("response body missing retry message: %s", secondResp.Body)
	}
}

func TestInterceptStreamChunkRestoresSplitCompletedWhenReasoningIsNormal(t *testing.T) {
	clearTestStreamState()
	baseReq := pluginapi.StreamChunkInterceptRequest{
		SourceFormat:    "openai-response",
		Model:           "gpt-5.5",
		RequestedModel:  "gpt-5.5",
		OriginalRequest: []byte(`{"model":"gpt-5.5","input":"normal"}`),
	}
	first := baseReq
	first.ChunkIndex = 5
	first.Body = []byte("event: response.completed")

	firstResp := decodeStreamInterceptResponse(t, first)
	if !firstResp.DropChunk {
		t.Fatal("first split completed chunk DropChunk = false, want true")
	}

	second := baseReq
	second.ChunkIndex = 6
	second.Body = []byte(`data: {"type":"response.completed","response":{"id":"resp_2","usage":{"output_tokens_details":{"reasoning_tokens":42}}}}`)

	secondResp := decodeStreamInterceptResponse(t, second)
	if secondResp.DropChunk {
		t.Fatal("second split completed chunk DropChunk = true, want false")
	}
	if !bytes.Contains(secondResp.Body, []byte("event: response.completed")) {
		t.Fatalf("normal completed body missing restored event: %s", secondResp.Body)
	}
	if bytes.Contains(secondResp.Body, []byte("event: response.failed")) {
		t.Fatalf("normal completed body should not contain failed event: %s", secondResp.Body)
	}
}

func TestInterceptStreamChunkDropsChunksAfterRetryFailure(t *testing.T) {
	clearTestStreamState()
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

func clearTestStreamState() {
	streamState.Lock()
	defer streamState.Unlock()
	streamState.entries = make(map[string]streamStateEntry)
}
