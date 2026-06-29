package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func TestInterceptStreamChunkDropsReasoning516CompletedChunk(t *testing.T) {
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
